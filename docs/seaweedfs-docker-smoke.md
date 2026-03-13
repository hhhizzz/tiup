# SeaweedFS Docker Smoke Test

This document records the current reproducible Docker-based validation flow for `tiup seaweedfs`.

## Goal

Validate the basic SeaweedFS line against a real SSH-based multi-node environment with:

- `tiup-seaweedfs deploy`
- `tiup-seaweedfs start`
- `tiup-seaweedfs display`
- `tiup-seaweedfs destroy`
- filer metadata backed by TiKV
- `volume.paths[]` directory creation

## Scope

This is a smoke test, not a full CI suite. It intentionally reuses the repository's existing Docker-based `tiup-cluster` integration environment.

## Preconditions

- Host machine has Docker available.
- Host machine can build this repository.
- Host machine can install Python `jinja2`.
- The current implementation of `tiup seaweedfs` still invokes TiUP component download tasks during `deploy`, so a local TiUP mirror is needed even though the actual runtime artifact also comes from `package_path`.

## Environment bootstrapping

On macOS, `docker/up.sh` currently assumes GNU-style `python`, `pip`, and `sed -i`. For local smoke testing, a temporary shim directory can be used instead of changing the repository script:

```bash
mkdir -p /tmp/tiup-fakebin
ln -sf "$(command -v python3)" /tmp/tiup-fakebin/python
ln -sf "$(command -v pip3)" /tmp/tiup-fakebin/pip
cat > /tmp/tiup-fakebin/sed <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == "-i" ]]; then
  shift
  /usr/bin/sed -i '' "$@"
else
  /usr/bin/sed "$@"
fi
EOF
chmod +x /tmp/tiup-fakebin/sed
```

Start the integration containers:

```bash
PATH="/tmp/tiup-fakebin:$PATH" \
PIP_BREAK_SYSTEM_PACKAGES=1 \
PIP_INDEX_URL=https://pypi.org/simple \
TIUP_CLUSTER_ROOT=$(pwd) \
./docker/up.sh --daemon -n 3
```

Expected running containers:

- `tiup-cluster-control`
- `tiup-cluster-n1`
- `tiup-cluster-n2`
- `tiup-cluster-n3`

## Control-node preparation

All commands below run from the host unless explicitly wrapped with `docker exec tiup-cluster-control`.

Confirm SSH from control node to the three test nodes:

```bash
docker exec tiup-cluster-control bash -lc 'ssh -o StrictHostKeyChecking=no root@172.19.0.101 hostname'
docker exec tiup-cluster-control bash -lc 'ssh -o StrictHostKeyChecking=no root@172.19.0.102 hostname'
docker exec tiup-cluster-control bash -lc 'ssh -o StrictHostKeyChecking=no root@172.19.0.103 hostname'
```

Build the TiUP binaries needed for the smoke test:

```bash
docker exec tiup-cluster-control bash -lc 'cd /tiup-cluster && make tiup cluster seaweedfs'
```

## Build a SeaweedFS package with TiKV support

The runtime test needs a `weed` binary built with the `tikv` build tag.

The dynamic binary can fail on the Debian-based node image with missing GLIBC versions. Build a static binary instead:

```bash
docker exec tiup-cluster-control bash -lc '
  cd /tmp &&
  rm -rf seaweedfs-bin-static &&
  mkdir -p seaweedfs-bin-static &&
  CGO_ENABLED=0 GOBIN=/tmp/seaweedfs-bin-static \
    go install -tags=tikv github.com/seaweedfs/seaweedfs/weed@4.05 &&
  tar -czf /tmp/seaweedfs-4.05-linux-arm64-tikv-static.tar.gz \
    -C /tmp/seaweedfs-bin-static weed
'
```

Sanity-check on a node:

```bash
docker exec tiup-cluster-control bash -lc '
  scp -o StrictHostKeyChecking=no /tmp/seaweedfs-bin-static/weed root@172.19.0.101:/tmp/weed-static &&
  ssh root@172.19.0.101 "chmod +x /tmp/weed-static && /tmp/weed-static version"
'
```

## Bring up a real PD/TiKV backend

Because the control node may not be able to reach the public TiUP mirror, use real `pd` and `tikv` containers directly for the filer backend:

```bash
docker run -d --name pd-test --network tiops --ip 172.19.0.110 \
  pingcap/pd:v6.2.0 \
  --name=pd-test \
  --data-dir=/data \
  --client-urls=http://0.0.0.0:2379 \
  --advertise-client-urls=http://172.19.0.110:2379 \
  --peer-urls=http://0.0.0.0:2380 \
  --advertise-peer-urls=http://172.19.0.110:2380 \
  --initial-cluster=pd-test=http://172.19.0.110:2380

docker run -d --name tikv-test --network tiops --ip 172.19.0.111 \
  pingcap/tikv:v6.2.0 \
  --addr=0.0.0.0:20160 \
  --advertise-addr=172.19.0.111:20160 \
  --status-addr=0.0.0.0:20180 \
  --pd=http://172.19.0.110:2379 \
  --data-dir=/data
```

Verify PD and TiKV are ready:

```bash
docker logs --tail 30 pd-test
docker logs --tail 30 tikv-test
```

## Seed TiUP-compatible TiDB metadata

`tiup seaweedfs` resolves `from_tidb_cluster` from TiUP cluster metadata. For smoke testing, write a minimal compatible `meta.yaml` that points at the real PD container:

```bash
docker exec tiup-cluster-control bash -lc '
  mkdir -p /root/.tiup/storage/cluster/clusters/tidbmini &&
  cat > /root/.tiup/storage/cluster/clusters/tidbmini/meta.yaml <<\"EOF\"
user: root
tidb_version: v6.2.0
topology:
  global:
    enable_tls: false
  pd_servers:
    - host: 172.19.0.110
      client_port: 2379
EOF
'
```

## Build a local TiUP mirror for SeaweedFS components

Current `tiup seaweedfs deploy` still runs the generic download stage, so publish the static package into a local mirror for:

- `seaweedfs-master`
- `seaweedfs-volume`
- `seaweedfs-filer`

Initialize the mirror:

```bash
docker exec tiup-cluster-control bash -lc '
  cd /tiup-cluster &&
  rm -rf /tmp/swmirror &&
  mkdir -p /tmp/swmirror &&
  ./bin/tiup mirror init /tmp/swmirror
'
```

Grant a temporary local owner and publish the static package under a SemVer that `tiup-seaweedfs deploy` accepts:

```bash
docker exec tiup-cluster-control bash -lc '
  KEY=$(ls /tmp/swmirror/keys/*-root.json | head -n 1)
  cd /tiup-cluster
  TIUP_MIRRORS=/tmp/swmirror ./bin/tiup mirror grant swtest -n swtest -k "$KEY"
  TIUP_MIRRORS=/tmp/swmirror ./bin/tiup mirror publish seaweedfs-master v4.5.1 /tmp/seaweedfs-4.05-linux-arm64-tikv-static.tar.gz weed --os linux --arch arm64 --desc "SeaweedFS master static" -k "$KEY"
  TIUP_MIRRORS=/tmp/swmirror ./bin/tiup mirror publish seaweedfs-volume v4.5.1 /tmp/seaweedfs-4.05-linux-arm64-tikv-static.tar.gz weed --os linux --arch arm64 --desc "SeaweedFS volume static" -k "$KEY"
  TIUP_MIRRORS=/tmp/swmirror ./bin/tiup mirror publish seaweedfs-filer v4.5.1 /tmp/seaweedfs-4.05-linux-arm64-tikv-static.tar.gz weed --os linux --arch arm64 --desc "SeaweedFS filer static" -k "$KEY"
'
```

## Write the SeaweedFS topology

```bash
docker exec tiup-cluster-control bash -lc '
  cat > /tmp/swfsstatic.yaml <<\"EOF\"
global:
  user: "tidb"
  ssh_port: 22
  deploy_dir: "/swfs-deploy"
  data_dir: "/swfs-data"
  arch: "arm64"
package_path: "/tmp/seaweedfs-4.05-linux-arm64-tikv-static.tar.gz"
filer_store:
  type: tikv
  from_tidb_cluster: "tidbmini"
  key_prefix: "swfsstatic"
master_servers:
  - host: 172.19.0.101
volume_servers:
  - host: 172.19.0.102
    paths:
      - path: "/data1/seaweedfs"
      - path: "/data2/seaweedfs"
filer_servers:
  - host: 172.19.0.103
EOF
'
```

## Deploy and inspect

Deploy:

```bash
docker exec tiup-cluster-control bash -lc '
  cd /tiup-cluster &&
  TIUP_MIRRORS=/tmp/swmirror \
  ./bin/tiup-seaweedfs deploy swfsstatic 4.5.1 /tmp/swfsstatic.yaml -u root -y
'
```

Expected deploy checks:

- deploy succeeds
- `filer.toml` exists on `172.19.0.103`
- `filer.toml` contains:
  - `[tikv]`
  - `enabled = true`
  - `pdaddrs = "172.19.0.110:2379"`
  - `keyPrefix = "swfsstatic"`
- `/data1/seaweedfs` and `/data2/seaweedfs` are created on `172.19.0.102`

Useful validation commands:

```bash
docker exec tiup-cluster-control bash -lc 'ssh root@172.19.0.103 "sudo cat /swfs-deploy/seaweedfs-filer-8888/filer.toml"'
docker exec tiup-cluster-control bash -lc 'ssh root@172.19.0.102 "sudo ls -ld /data1/seaweedfs /data2/seaweedfs"'
docker exec tiup-cluster-control bash -lc 'cd /tiup-cluster && ./bin/tiup-seaweedfs display swfsstatic'
```

## Start

Start:

```bash
docker exec tiup-cluster-control bash -lc '
  cd /tiup-cluster &&
  TIUP_MIRRORS=/tmp/swmirror \
  ./bin/tiup-seaweedfs start swfsstatic
'
```

Expected start checks:

- `start` succeeds for all three roles
- `display` reports all three nodes as `Up`

Observed good-state output:

```text
Cluster type:       seaweedfs
Cluster name:       swfsstatic
Cluster version:    v4.5.1
Deploy user:        tidb
SSH type:           builtin
ID                 Role              Host          Ports  OS/Arch        Status  Data Dir                          Deploy Dir
--                 ----              ----          -----  -------        ------  --------                          ----------
172.19.0.103:8888  seaweedfs-filer   172.19.0.103  8888   linux/aarch64  Up      -                                 /swfs-deploy/seaweedfs-filer-8888
172.19.0.101:9333  seaweedfs-master  172.19.0.101  9333   linux/aarch64  Up      /swfs-data/seaweedfs-master-9333  /swfs-deploy/seaweedfs-master-9333
172.19.0.102:8080  seaweedfs-volume  172.19.0.102  8080   linux/aarch64  Up      /data1/seaweedfs                  /swfs-deploy/seaweedfs-volume-8080
Total nodes: 3
```

## Cleanup

```bash
docker exec tiup-cluster-control bash -lc '
  cd /tiup-cluster &&
  TIUP_MIRRORS=/tmp/swmirror \
  ./bin/tiup-seaweedfs destroy swfsstatic -y --force || true
'

docker rm -f pd-test tikv-test || true
```

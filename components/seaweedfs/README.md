# SeaweedFS Deployment Guide

This component adds a basic `tiup seaweedfs` line for deploying and operating SeaweedFS `master`, `volume`, and `filer`.

Current scope:

- `deploy`
- `start`
- `stop`
- `destroy`
- `display`
- filer metadata backed by TiKV from an existing TiUP-managed TiDB cluster
- `volume.paths[]` with multiple storage paths

## Current deployment model

Two artifact inputs are currently required:

1. `package_path`
   An absolute path on the control machine that points to a local `.tar.gz` containing the `weed` binary.
2. TiUP component source
   The current implementation still goes through the generic TiUP download phase during `deploy`, so the SeaweedFS components must also exist in the configured TiUP mirror.

In practice this means:

- for development or private use, use a local TiUP mirror that publishes:
  - `seaweedfs-master`
  - `seaweedfs-volume`
  - `seaweedfs-filer`
- set `package_path` to the same `weed` tarball used for the runtime install

## Prerequisites

Before deploying SeaweedFS, prepare:

- a working SSH path from the control machine to all target hosts
- a local `weed` tarball
- a TiDB cluster name that resolves through `filer_store.from_tidb_cluster`
- a TiUP mirror that contains SeaweedFS component metadata for the target version

For TiKV-backed filer metadata, the `weed` binary must include TiKV support. The upstream TiKV store implementation lives behind the `tikv` build tag, so the simplest safe packaging method is to build a static binary:

```bash
CGO_ENABLED=0 go install -tags=tikv github.com/seaweedfs/seaweedfs/weed@4.05
tar -czf /tmp/seaweedfs-4.05-linux-amd64-tikv-static.tar.gz -C "$(go env GOPATH)/bin" weed
```

If your target nodes are older Debian-based systems, prefer the static build. A dynamically linked build can fail on startup with missing `GLIBC_*` symbols.

## Direct CLI quick start

If you do not want to use Docker, the normal command-line deployment flow is:

1. Build a `weed` package with TiKV support.
2. Publish that package into a local TiUP mirror as:
   - `seaweedfs-master`
   - `seaweedfs-volume`
   - `seaweedfs-filer`
3. Write a topology that points `package_path` at the same local tarball and `filer_store.from_tidb_cluster` at an existing TiUP-managed TiDB cluster.
4. Run `deploy`, `start`, `display`, `stop`, and `destroy` from the shell.

The commands below assume:

- you are on the control machine
- your target hosts are reachable over SSH
- you already have a TiDB cluster managed by TiUP, for example `tidb-prod`
- you want to deploy SeaweedFS as version `4.5.1` on the TiUP side

### 1. Build the SeaweedFS package

Build a static `weed` binary with the `tikv` build tag and pack it into a tarball:

```bash
CGO_ENABLED=0 \
go install -tags=tikv github.com/seaweedfs/seaweedfs/weed@4.05

tar -czf /tmp/seaweedfs-4.05-linux-amd64-tikv-static.tar.gz \
  -C "$(go env GOPATH)/bin" \
  weed
```

If you are deploying to `arm64` machines, build on an `arm64` control host or cross-compile an `arm64` binary first and then pack it.

### 2. Publish the package into a local TiUP mirror

Current `tiup seaweedfs deploy` still runs the generic TiUP download stage, so the SeaweedFS components must exist in the configured TiUP mirror even though the runtime tarball also comes from `package_path`.

Initialize a local mirror:

```bash
mkdir -p /tmp/swmirror
tiup mirror init /tmp/swmirror
```

Pick one generated root key and grant a temporary owner:

```bash
KEY=$(ls /tmp/swmirror/keys/*-root.json | head -n 1)
TIUP_MIRRORS=/tmp/swmirror tiup mirror grant swtest -n swtest -k "$KEY"
```

Publish the same tarball as the three SeaweedFS components:

```bash
TIUP_MIRRORS=/tmp/swmirror tiup mirror publish \
  seaweedfs-master v4.5.1 /tmp/seaweedfs-4.05-linux-amd64-tikv-static.tar.gz weed \
  --os linux --arch amd64 --desc "SeaweedFS master" -k "$KEY"

TIUP_MIRRORS=/tmp/swmirror tiup mirror publish \
  seaweedfs-volume v4.5.1 /tmp/seaweedfs-4.05-linux-amd64-tikv-static.tar.gz weed \
  --os linux --arch amd64 --desc "SeaweedFS volume" -k "$KEY"

TIUP_MIRRORS=/tmp/swmirror tiup mirror publish \
  seaweedfs-filer v4.5.1 /tmp/seaweedfs-4.05-linux-amd64-tikv-static.tar.gz weed \
  --os linux --arch amd64 --desc "SeaweedFS filer" -k "$KEY"
```

The important detail here is that the TiUP deploy version must be a SemVer-like string such as `4.5.1`, even if the upstream SeaweedFS tag used to build the package is `4.05`.

### 3. Write the topology

Example:

```yaml
global:
  user: "tidb"
  ssh_port: 22
  deploy_dir: "/swfs-deploy"
  data_dir: "/swfs-data"
  arch: "amd64"

package_path: "/tmp/seaweedfs-4.05-linux-amd64-tikv-static.tar.gz"

filer_store:
  type: tikv
  from_tidb_cluster: "tidb-prod"
  key_prefix: "swfs-prod"

master_servers:
  - host: 10.0.1.21

volume_servers:
  - host: 10.0.1.22
    paths:
      - path: "/data1/seaweedfs"
      - path: "/data2/seaweedfs"

filer_servers:
  - host: 10.0.1.23
```

Save it as `swfs-topology.yaml`.

### 4. Deploy and start from the shell

Deploy:

```bash
TIUP_MIRRORS=/tmp/swmirror \
tiup-seaweedfs deploy swfs-prod 4.5.1 ./swfs-topology.yaml -u root -y
```

Start:

```bash
TIUP_MIRRORS=/tmp/swmirror \
tiup-seaweedfs start swfs-prod
```

Inspect:

```bash
TIUP_MIRRORS=/tmp/swmirror \
tiup-seaweedfs display swfs-prod
```

Stop:

```bash
TIUP_MIRRORS=/tmp/swmirror \
tiup-seaweedfs stop swfs-prod
```

Destroy:

```bash
TIUP_MIRRORS=/tmp/swmirror \
tiup-seaweedfs destroy swfs-prod -y --force
```

### 5. Check the generated filer configuration

After `deploy`, check that the filer got a TiKV-backed metadata config:

```bash
ssh root@<filer-host> 'sudo cat /swfs-deploy/seaweedfs-filer-8888/filer.toml'
```

You should see at least:

```toml
[tikv]
enabled = true
pdaddrs = "..."
keyPrefix = "swfs-prod"
```

## Topology example

See:

- [minimal.yaml](/Users/qiwei.huang/Source/tiup/embed/examples/seaweedfs/minimal.yaml)
- [topology.example.yaml](/Users/qiwei.huang/Source/tiup/embed/examples/seaweedfs/topology.example.yaml)

Minimal shape:

```yaml
global:
  user: "tidb"
  ssh_port: 22
  deploy_dir: "/swfs-deploy"
  data_dir: "/swfs-data"
  arch: "amd64"

package_path: "/tmp/seaweedfs-4.05-linux-amd64-tikv-static.tar.gz"

filer_store:
  type: tikv
  from_tidb_cluster: "tidb-prod"
  key_prefix: "swfs-prod"

master_servers:
  - host: 10.0.1.21

volume_servers:
  - host: 10.0.1.22
    paths:
      - path: "/data1/seaweedfs"
      - path: "/data2/seaweedfs"

filer_servers:
  - host: 10.0.1.23
```

Important field semantics:

- `package_path`
  Read locally on the control machine and uploaded to every target host.
- `filer_store.from_tidb_cluster`
  Must point to a TiUP-managed TiDB cluster metadata directory under `storage/cluster/clusters/<name>`.
- `filer_store.key_prefix`
  Required namespace for SeaweedFS keys in TiKV.
- `volume_servers[].paths`
  Real storage paths on the target volume host. These paths are created during deploy and passed to `weed volume` as a comma-separated `-dir`.

## Deploy

Use a SemVer-like version string in the TiUP command, even if the upstream SeaweedFS release tag is not strict SemVer. For example, a package built from upstream `4.05` can be published and deployed as `4.5.1` on the TiUP side.

```bash
TIUP_MIRRORS=/path/to/local-mirror \
tiup-seaweedfs deploy swfs-prod 4.5.1 ./swfs-topology.yaml -u root -y
```

What deploy does:

- detects host OS/arch
- downloads SeaweedFS component manifests from the configured mirror
- uploads the local `package_path` tarball to each node
- generates:
  - systemd units
  - `run_seaweedfs-master.sh`
  - `run_seaweedfs-volume.sh`
  - `run_seaweedfs-filer.sh`
  - `filer.toml`
- copies TLS assets for the TiKV backend when the referenced TiDB cluster enables TLS

Recommended post-deploy checks:

```bash
tiup-seaweedfs display swfs-prod
ssh root@<filer-host> 'sudo cat /swfs-deploy/seaweedfs-filer-8888/filer.toml'
ssh root@<volume-host> 'sudo ls -ld /data1/seaweedfs /data2/seaweedfs'
```

## Start

```bash
TIUP_MIRRORS=/path/to/local-mirror \
tiup-seaweedfs start swfs-prod
```

After `start`, `display` should report all nodes as `Up`:

```bash
tiup-seaweedfs display swfs-prod
```

## Stop and destroy

```bash
tiup-seaweedfs stop swfs-prod
tiup-seaweedfs destroy swfs-prod -y --force
```

## Troubleshooting

- `deploy` fails while downloading `seaweedfs-*`
  Your configured TiUP mirror does not contain the SeaweedFS components. Publish them to a local mirror first.
- filer starts but cannot use TiKV
  Check `filer.toml` for:
  - `[tikv]`
  - `enabled = true`
  - correct `pdaddrs`
  - correct `keyPrefix`
  - TLS paths when the referenced TiDB cluster has TLS enabled
- target node reports `GLIBC_* not found`
  Rebuild the `weed` tarball with `CGO_ENABLED=0`.

## Reproducible smoke test

For the exact Docker-based validation flow used during development, including a local TiUP mirror and a real `pd`/`tikv` backend, see [docs/seaweedfs-docker-smoke.md](/Users/qiwei.huang/Source/tiup/docs/seaweedfs-docker-smoke.md).

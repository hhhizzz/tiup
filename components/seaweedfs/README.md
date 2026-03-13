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

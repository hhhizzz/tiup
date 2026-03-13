# SeaweedFS Mirror-Only Deployment Design

## Summary

SeaweedFS deployment currently accepts two artifact inputs:

- a topology-level `package_path` pointing to a local tarball on the control machine
- a TiUP mirror containing `seaweedfs-master`, `seaweedfs-volume`, and `seaweedfs-filer`

This is redundant. The deployment manager already downloads component artifacts from the configured mirror and copies them to target hosts. SeaweedFS adds a second artifact path by overriding instance deployment and uploading a local tarball directly. The result is duplicated semantics and a mismatch between the configured TiUP version and the runtime package source.

This design removes the local package path entirely and makes the configured TiUP mirror the single source of truth for SeaweedFS deployment artifacts.

## Goals

- Remove `package_path` from SeaweedFS topology.
- Make SeaweedFS deploy artifacts come only from the configured TiUP mirror.
- Reuse the generic cluster manager download and copy flow instead of maintaining a SeaweedFS-specific artifact path.
- Update tests, templates, and user documentation to reflect the new single-source artifact model.

## Non-Goals

- Preserve compatibility for existing topology files that still declare `package_path`.
- Introduce multiple artifact source modes or a new abstraction for package origin.
- Change the startup scripts, filer TiKV configuration, or role ordering behavior.

## Current Behavior

Today, SeaweedFS topology includes `package_path`, validates that it is an absolute path on the control machine, checks that the tarball exists, and verifies that it contains a `weed` binary. Each SeaweedFS instance then overrides the generic deployment flow and uploads that local tarball directly to the target host.

At the same time, the generic deploy manager still schedules component downloads from the configured TiUP mirror for each SeaweedFS component source. This means the mirror is already required even though the runtime package is not actually taken from it.

## Proposed Behavior

After this change:

- `package_path` is removed from `components/seaweedfs/spec.Specification`.
- SeaweedFS topology parsing rejects `package_path` because topology parsing uses strict known-field decoding.
- SeaweedFS instance types stop overriding deployment to install a local tarball.
- The generic manager path downloads `seaweedfs-master`, `seaweedfs-volume`, and `seaweedfs-filer` from the configured mirror and copies those packages to target hosts.
- The user-supplied deploy version becomes the real artifact version resolved from the mirror rather than a metadata-only label.

## Architecture

The implementation intentionally removes the subset helper path instead of keeping two ways to install SeaweedFS:

1. Delete the topology field and validation that make `package_path` mandatory.
2. Delete the local package validation and installation helper.
3. Remove `Deploy` implementations from SeaweedFS instances so the manager falls back to `CopyComponent`.
4. Keep `ComponentSource()` and `CalculateVersion()` unchanged so the existing mirror download flow remains the single deployment path.

This aligns SeaweedFS with the rest of the TiUP cluster manager and makes the mirror the single source of truth for artifact resolution.

## Compatibility and Migration

This is an intentional breaking change for topology files:

- Old topologies that include `package_path` will fail parsing because unknown fields are rejected.
- Users must publish SeaweedFS packages to a TiUP mirror before deploying.
- The deploy command version must match a version available in that mirror.

The migration path is simple:

1. Publish the SeaweedFS tarball to the mirror as `seaweedfs-master`, `seaweedfs-volume`, and `seaweedfs-filer`.
2. Remove `package_path` from the topology file.
3. Deploy using a mirror version that exists for those components.

## Testing Strategy

The behavior change will be covered with tests that prove:

- topology parsing succeeds without `package_path`
- topology parsing rejects legacy `package_path`
- deploy topology validation no longer inspects local tarballs
- template output no longer includes `package_path`
- metadata and topology merge logic no longer depend on `package_path`

The implementation will follow TDD:

1. write failing tests for the new mirror-only behavior
2. run the focused test targets and observe failure
3. implement the minimal code changes
4. rerun focused tests
5. run the broader SeaweedFS verification set and repository lint/build commands as appropriate

## Risks

- Existing local examples or operator habits that depend on `package_path` will break immediately.
- If any SeaweedFS path still implicitly depends on the removed field, deploy or metadata tests may fail in non-obvious places.
- Documentation must be updated together with code or users will continue trying the removed topology field.

## Recommendation

Proceed with a clean mirror-only cut. This removes the redundant artifact path rather than hiding it behind compatibility code, and it makes SeaweedFS deployment semantics consistent with the rest of TiUP.

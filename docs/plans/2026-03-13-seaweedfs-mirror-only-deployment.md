# SeaweedFS Mirror-Only Deployment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove `package_path` from SeaweedFS deployment so the configured TiUP mirror is the only artifact source.

**Architecture:** Delete the SeaweedFS-specific local package path from topology, validation, and deploy helpers, then let the generic cluster manager artifact flow handle SeaweedFS like other TiUP components. Update tests first, then remove the redundant implementation, and finally update templates and documentation to match the new mirror-only semantics.

**Tech Stack:** Go, Cobra, TiUP cluster manager, `testify/require`, YAML examples, Markdown docs.

---

### Task 1: Topology and Metadata Tests

**Files:**
- Modify: `components/seaweedfs/spec/topology_seaweedfs_test.go`
- Modify: `components/seaweedfs/spec/cluster_test.go`

**Step 1: Write the failing tests**

Add tests that:

- parse a valid topology without `package_path` and assert the role defaults still work
- reject a topology that still includes `package_path`
- verify metadata/topology setters and merged topologies no longer carry a package path field

Representative test snippets:

```go
func TestTopologyRejectsLegacyPackagePathField(t *testing.T) {
	topoFile := writeTopologyFile(t, `
package_path: "/tmp/seaweedfs.tar.gz"
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
filer_servers:
  - host: 10.0.1.23
`)
	var topo Specification
	err := cspec.ParseTopologyYaml(topoFile, &topo)
	require.Error(t, err)
	require.Contains(t, err.Error(), "package_path")
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./components/seaweedfs/spec -run 'TestTopologyDefaultsAndValidation|TestTopologyRejectsLegacyPackagePathField|TestMetadataSetTopology|TestNewPartAndMergeTopo' -v`

Expected: FAIL because current code still requires `package_path` and tests still reference the removed field.

**Step 3: Write minimal implementation**

Modify `components/seaweedfs/spec/topology_seaweedfs.go` and `components/seaweedfs/spec/cluster_test.go` expectations so the topology no longer contains `PackagePath`.

**Step 4: Run tests to verify they pass**

Run: `go test ./components/seaweedfs/spec -run 'TestTopologyDefaultsAndValidation|TestTopologyRejectsLegacyPackagePathField|TestMetadataSetTopology|TestNewPartAndMergeTopo' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add components/seaweedfs/spec/topology_seaweedfs.go \
  components/seaweedfs/spec/topology_seaweedfs_test.go \
  components/seaweedfs/spec/cluster_test.go
git commit -m "refactor: remove seaweedfs package_path topology"
```

### Task 2: Deploy Validation and Artifact Flow

**Files:**
- Modify: `components/seaweedfs/command/deploy_test.go`
- Modify: `components/seaweedfs/command/deploy.go`
- Modify: `components/seaweedfs/spec/master.go`
- Modify: `components/seaweedfs/spec/volume.go`
- Modify: `components/seaweedfs/spec/filer.go`
- Delete: `components/seaweedfs/spec/package.go`
- Delete: `components/seaweedfs/spec/package_test.go`

**Step 1: Write the failing tests**

Replace deploy tests with mirror-only expectations:

- `validateDeployTopology` accepts a topology that has no `package_path`
- a topology that still includes `package_path` fails because strict YAML parsing rejects the field

Representative test snippet:

```go
func TestDeployAcceptsMirrorOnlyTopology(t *testing.T) {
	topoFile := writeTopologyFile(t, `
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
filer_servers:
  - host: 10.0.1.23
`)

	require.NoError(t, validateDeployTopology(topoFile))
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./components/seaweedfs/command -run 'TestDeployAcceptsMirrorOnlyTopology|TestDeployRejectsLegacyPackagePathField' -v`

Expected: FAIL because deploy validation still inspects `package_path`.

**Step 3: Write minimal implementation**

- remove `ValidateLocalPackage` usage from `components/seaweedfs/command/deploy.go`
- delete the SeaweedFS local package helper file and tests
- remove the `Deploy` methods from SeaweedFS instance types so the manager uses generic `CopyComponent`

**Step 4: Run tests to verify they pass**

Run: `go test ./components/seaweedfs/command -run 'TestDeployAcceptsMirrorOnlyTopology|TestDeployRejectsLegacyPackagePathField' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add components/seaweedfs/command/deploy.go \
  components/seaweedfs/command/deploy_test.go \
  components/seaweedfs/spec/master.go \
  components/seaweedfs/spec/volume.go \
  components/seaweedfs/spec/filer.go \
  components/seaweedfs/spec/package.go \
  components/seaweedfs/spec/package_test.go
git commit -m "refactor: use mirror-only seaweedfs artifacts"
```

### Task 3: Templates and Documentation

**Files:**
- Modify: `components/seaweedfs/command/template_test.go`
- Modify: `embed/examples/seaweedfs/minimal.yaml`
- Modify: `embed/examples/seaweedfs/topology.example.yaml`
- Modify: `components/seaweedfs/README.md`

**Step 1: Write the failing tests**

Update template tests to assert:

- the generated template still includes `master_servers`, `volume_servers`, and `filer_store`
- the generated template no longer includes `package_path`

Representative test snippet:

```go
require.NotContains(t, out.String(), "package_path:")
```

**Step 2: Run tests to verify they fail**

Run: `go test ./components/seaweedfs/command -run TestTemplateCommandPrintsSeaweedTemplate -v`

Expected: FAIL because the embedded examples still print `package_path`.

**Step 3: Write minimal implementation**

- remove `package_path` comments and examples from the embedded topology templates
- rewrite README sections so they describe mirror-only deployment and version semantics

**Step 4: Run tests to verify they pass**

Run: `go test ./components/seaweedfs/command -run TestTemplateCommandPrintsSeaweedTemplate -v`

Expected: PASS

**Step 5: Commit**

```bash
git add components/seaweedfs/command/template_test.go \
  embed/examples/seaweedfs/minimal.yaml \
  embed/examples/seaweedfs/topology.example.yaml \
  components/seaweedfs/README.md
git commit -m "docs: remove seaweedfs package_path usage"
```

### Task 4: Full Verification

**Files:**
- Verify only

**Step 1: Run focused SeaweedFS tests**

Run: `go test ./components/seaweedfs/... -v`

Expected: PASS

**Step 2: Run repository build and lint checks required by the repo**

Run: `make lint && make check-static`

Expected: PASS

**Step 3: Review diff for unintended drift**

Run: `git diff --stat`

Expected: Only SeaweedFS topology, command, templates, docs, and related tests changed.

**Step 4: Commit**

```bash
git add components/seaweedfs embed/examples/seaweedfs docs/plans
git commit -m "refactor: remove redundant seaweedfs package path"
```

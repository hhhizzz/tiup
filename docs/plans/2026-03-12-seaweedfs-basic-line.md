# SeaweedFS Basic Line Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build `tiup seaweedfs` basic production deployment and operations for SeaweedFS `master`, `volume`, and `filer`, with filer metadata backed by TiKV from an existing TiUP-managed TiDB cluster and with deployment from a user-provided local SeaweedFS tarball.

**Architecture:** Introduce an independent `components/seaweedfs` product line modeled after `components/dm`, with its own topology, metadata, and command entrypoint while reusing `pkg/cluster/manager`, `pkg/cluster/task`, and `pkg/cluster/operation`. Avoid all repository download changes by making SeaweedFS instances deploy from a cluster-level `package_path` and by implementing `manager.DeployerInstance` on each role. Resolve `filer_store.from_tidb_cluster` by reading TiDB cluster metadata from the existing cluster storage and translating it into filer TiKV backend config plus copied TLS assets.

**Tech Stack:** Go, Cobra, TiUP cluster manager/task framework, YAML topology parsing, embedded shell templates, `testify/require`, `go test`, `make lint`, `make check-static`

---

**Scope assumptions**

- Keep the CLI surface to `template`, `deploy`, `start`, `stop`, `destroy`, and `display`.
- Do not add repository download or manifest support.
- Require a cluster-level absolute `package_path` that points to a local `.tar.gz` containing the `weed` binary.
- `package_path` is read on the TiUP control machine and deployed with `task.Builder.InstallPackage`, which SCPs the local tarball to each target host and untars it under `<deploy>/bin`.
- Support only `filer_store.type: tikv` in this phase.
- Support `volume.paths[]` by mapping each server entry to one `weed volume` process that receives comma-separated `-dir` and `-disk` values.
- Keep the `deploy <cluster-name> <version> <topology.yaml>` command shape; `version` is operator-supplied SeaweedFS metadata stored in cluster meta and shown in `display`, not a repository lookup key.
- Resolve `filer_store.from_tidb_cluster` against TiUP's TiDB cluster storage under `storage/cluster/clusters`, not the SeaweedFS component profile directory.

### Task 1: Scaffold the SeaweedFS component and template command

**Files:**
- Create: `components/seaweedfs/main.go`
- Create: `components/seaweedfs/command/root.go`
- Create: `components/seaweedfs/command/template.go`
- Create: `components/seaweedfs/command/template_test.go`
- Create: `components/seaweedfs/spec/cluster.go`
- Create: `components/seaweedfs/spec/topology_seaweedfs.go`
- Create: `embed/examples/seaweedfs/minimal.yaml`
- Create: `embed/examples/seaweedfs/topology.example.yaml`
- Modify: `Makefile`

**Step 1: Write the failing template smoke test**

```go
func TestTemplateCommandPrintsSeaweedTemplate(t *testing.T) {
	var out bytes.Buffer
	cmd := newTemplateCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "package_path:")
	require.Contains(t, out.String(), "master_servers:")
	require.Contains(t, out.String(), "volume_servers:")
	require.Contains(t, out.String(), "filer_store:")
}
```

**Step 2: Run the test to confirm the command does not exist yet**

Run: `go test ./components/seaweedfs/command -run TestTemplateCommandPrintsSeaweedTemplate -v`

Expected: FAIL with undefined symbol errors for `newTemplateCmd` or missing package files.

**Step 3: Write the minimal CLI skeleton**

```go
func main() {
	tui.RegisterArg0("tiup seaweedfs")
	command.Execute()
}
```

```go
rootCmd = &cobra.Command{
	Use:           tui.OsArgs0(),
	Short:         "Deploy a SeaweedFS cluster for production",
	SilenceUsage:  true,
	SilenceErrors: true,
}

rootCmd.AddCommand(newTemplateCmd())
```

```go
func newTemplateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "template",
		Short: "Print topology template",
		RunE: func(cmd *cobra.Command, args []string) error {
			tpl, err := embed.ReadExample(path.Join("examples", "seaweedfs", "minimal.yaml"))
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(tpl))
			return nil
		},
	}
}
```

Add a `bin/tiup-seaweedfs` build target to `Makefile` and include `seaweedfs` in the `components:` target list.

**Step 4: Run the template test again**

Run: `go test ./components/seaweedfs/command -run TestTemplateCommandPrintsSeaweedTemplate -v`

Expected: PASS.

**Step 5: Run the new package tests**

Run: `go test ./components/seaweedfs/... -v`

Expected: PASS with only the template test present.

**Step 6: Commit the scaffold**

```bash
git add components/seaweedfs embed/examples/seaweedfs Makefile
git commit -m "feat: scaffold seaweedfs component"
```

### Task 2: Flesh out topology, metadata, defaults, and validation

**Files:**
- Modify: `components/seaweedfs/spec/cluster.go`
- Create: `components/seaweedfs/spec/cluster_test.go`
- Modify: `components/seaweedfs/spec/topology_seaweedfs.go`
- Create: `components/seaweedfs/spec/topology_seaweedfs_test.go`
- Modify: `embed/examples/seaweedfs/minimal.yaml`
- Modify: `embed/examples/seaweedfs/topology.example.yaml`

**Step 1: Write topology parsing and validation tests**

```go
func TestTopologyDefaultsAndValidation(t *testing.T) {
	topoFile := writeTopologyFile(t, `
global:
  user: "tidb"
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
	require.NoError(t, err)
	require.Equal(t, "/tmp/seaweedfs.tar.gz", topo.PackagePath)
	require.Equal(t, "001", topo.MasterServers[0].DefaultReplication)
	require.Equal(t, "/data1/seaweedfs", topo.VolumeServers[0].Paths[0].Path)
}

func TestTopologyRejectsEmptyVolumePaths(t *testing.T) {
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
    paths: []
filer_servers:
  - host: 10.0.1.23
`)
	var topo Specification
	err := cspec.ParseTopologyYaml(topoFile, &topo)
	require.ErrorContains(t, err, "volume_servers")
}

func TestMetadataImplementsManagerContracts(t *testing.T) {
	meta := &Metadata{Topology: new(Specification)}
	meta.SetUser("tidb")
	meta.SetVersion("3.80")
	require.Equal(t, "tidb", meta.GetBaseMeta().User)
	require.Equal(t, "3.80", meta.GetBaseMeta().Version)
	require.IsType(t, &Specification{}, meta.GetTopology())
}
```

**Step 2: Run the topology tests to confirm the richer model is missing**

Run: `go test ./components/seaweedfs/spec -run 'TestTopologyDefaultsAndValidation|TestTopologyRejectsEmptyVolumePaths' -v`

Expected: FAIL because `Specification` does not expose the new fields or validation.

**Step 3: Implement the topology model**

```go
type GlobalOptions = cspec.GlobalOptions

type FilerStoreSpec struct {
	Type            string `yaml:"type"`
	FromTiDBCluster string `yaml:"from_tidb_cluster"`
	KeyPrefix       string `yaml:"key_prefix"`
}

type VolumePathSpec struct {
	Path     string `yaml:"path"`
	DiskType string `yaml:"disk_type,omitempty"`
}

type MasterSpec struct {
	Host               string `yaml:"host"`
	SSHPort            int    `yaml:"ssh_port,omitempty"`
	Port               int    `yaml:"port,omitempty" default:"9333"`
	DefaultReplication string `yaml:"default_replication,omitempty" default:"001"`
	VolumeSizeLimitMB  int    `yaml:"volume_size_limit_mb,omitempty"`
	DeployDir          string `yaml:"deploy_dir,omitempty"`
	DataDir            string `yaml:"data_dir,omitempty"`
	LogDir             string `yaml:"log_dir,omitempty"`
	Arch               string `yaml:"arch,omitempty"`
	OS                 string `yaml:"os,omitempty"`
}

type VolumeSpec struct {
	Host      string           `yaml:"host"`
	SSHPort   int              `yaml:"ssh_port,omitempty"`
	Port      int              `yaml:"port,omitempty" default:"8080"`
	Paths     []VolumePathSpec `yaml:"paths"`
	DataCenter string          `yaml:"data_center,omitempty"`
	Rack      string           `yaml:"rack,omitempty"`
	DeployDir string           `yaml:"deploy_dir,omitempty"`
	DataDir   string           `yaml:"data_dir,omitempty"`
	LogDir    string           `yaml:"log_dir,omitempty"`
	Arch      string           `yaml:"arch,omitempty"`
	OS        string           `yaml:"os,omitempty"`
}

type FilerSpec struct {
	Host      string `yaml:"host"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	Port      int    `yaml:"port,omitempty" default:"8888"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
	LogDir    string `yaml:"log_dir,omitempty"`
	Arch      string `yaml:"arch,omitempty"`
	OS        string `yaml:"os,omitempty"`
}

type Specification struct {
	GlobalOptions GlobalOptions `yaml:"global,omitempty" validate:"global:editable"`
	PackagePath   string        `yaml:"package_path"`
	FilerStore    FilerStoreSpec `yaml:"filer_store"`
	MasterServers []*MasterSpec `yaml:"master_servers"`
	VolumeServers []*VolumeSpec `yaml:"volume_servers"`
	FilerServers  []*FilerSpec  `yaml:"filer_servers"`
}

func (s *Specification) UnmarshalYAML(unmarshal func(any) error) error {
	type topology Specification
	if err := unmarshal((*topology)(s)); err != nil {
		return err
	}
	if err := defaults.Set(s); err != nil {
		return errors.Trace(err)
	}
	if err := fillSeaweedCustomDefaults(&s.GlobalOptions, s); err != nil {
		return err
	}
	return s.Validate()
}
```

Implement `Type()`, `BaseTopo()`, `GetMonitoredOptions()`, `TLSConfig()`, `NewPart()`, `MergeTopo()`, `Merge()`, `ComponentsByStartOrder()`, `ComponentsByStopOrder()`, `ComponentsByUpdateOrder()`, `IterInstance()`, `CountDir()`, `FillHostArchOrOS()`, `GetGrafanaConfig()`, and `Validate()`.

Validation rules:

- `package_path` must be non-empty and absolute.
- `filer_store.type` must be exactly `tikv`.
- `filer_store.from_tidb_cluster` and `filer_store.key_prefix` must be non-empty.
- Each `volume_servers[*].paths` must contain at least one non-empty path.
- Each `volume_servers[*].paths[*].path` must be unique within the same server entry.
- `BaseTopo()` must always return a non-nil `GlobalOptions` because `manager.Deploy()` calls `spec.ExpandRelativeDir(topo)` and then reads `topo.BaseTopo().GlobalOptions`.
- Reuse the shared host/platform/port/dir conflict helpers where possible.

**Step 4: Run the topology tests again**

Run: `go test ./components/seaweedfs/spec -run 'TestTopologyDefaultsAndValidation|TestTopologyRejectsEmptyVolumePaths' -v`

Expected: PASS.

**Step 5: Add interface and metadata regression tests**

```go
func TestComponentsByStartOrder(t *testing.T) {
	topo := &Specification{}
	names := []string{}
	for _, comp := range topo.ComponentsByStartOrder() {
		names = append(names, comp.Name())
	}
	require.Equal(t, []string{
		ComponentSeaweedMaster,
		ComponentSeaweedVolume,
		ComponentSeaweedFiler,
	}, names)
}
```

Add a second assertion set that exercises `NewPart()`, `MergeTopo()`, and `GetBaseMeta()` so the plan covers the methods that `manager.Deploy()` and `SpecManager.SaveMeta()` call directly.

**Step 6: Run the full spec package**

Run: `go test ./components/seaweedfs/spec -v`

Expected: PASS.

**Step 7: Commit the topology layer**

```bash
git add components/seaweedfs/spec embed/examples/seaweedfs
git commit -m "feat: add seaweedfs topology model"
```

### Task 3: Validate the local package and isolate deployment from repository downloads

**Files:**
- Create: `components/seaweedfs/spec/package.go`
- Create: `components/seaweedfs/spec/package_test.go`
- Modify: `components/seaweedfs/spec/topology_seaweedfs.go`

**Step 1: Write the failing package validation tests**

```go
func TestValidateLocalPackageRequiresWeedEntry(t *testing.T) {
	path := buildTarball(t, map[string]string{
		"README.md": "not a binary",
	})
	err := ValidateLocalPackage(path)
	require.ErrorContains(t, err, "weed")
}

func TestValidateLocalPackageAcceptsWeedBinary(t *testing.T) {
	path := buildTarball(t, map[string]string{
		"weed": "#!/bin/sh\necho ok\n",
	})
	require.NoError(t, ValidateLocalPackage(path))
}

func TestValidateLocalPackageRejectsMissingFile(t *testing.T) {
	err := ValidateLocalPackage("/tmp/does-not-exist-seaweedfs.tar.gz")
	require.Error(t, err)
}
```

**Step 2: Run the package tests to confirm the helper does not exist**

Run: `go test ./components/seaweedfs/spec -run 'TestValidateLocalPackage' -v`

Expected: FAIL with undefined `ValidateLocalPackage`.

**Step 3: Implement package validation and a shared deploy helper**

```go
func ValidateLocalPackage(packagePath string) error {
	if !filepath.IsAbs(packagePath) {
		return fmt.Errorf("package_path must be absolute")
	}
	if _, err := os.Stat(packagePath); err != nil {
		return err
	}
	if filepath.Ext(packagePath) != ".gz" {
		return fmt.Errorf("package_path must point to a .tar.gz file")
	}
	if _, err := findTarEntry(packagePath, "weed"); err != nil {
		return err
	}
	return nil
}

func installLocalPackage(b *task.Builder, packagePath, host, deployDir string) *task.Builder {
	return b.InstallPackage(packagePath, host, deployDir).
		Shell(host, fmt.Sprintf("test -f %[1]s/bin/weed && chmod +x %[1]s/bin/weed", deployDir), "chmod-seaweedfs", false)
}
```

Keep `Validate()` strict about syntax and keep file-content checks in this helper so the helper can be called from `deploy` preflight without polluting YAML parsing. Document explicitly that `package_path` must exist on the control machine running `tiup seaweedfs`, not on remote hosts.

**Step 4: Run the package tests again**

Run: `go test ./components/seaweedfs/spec -run 'TestValidateLocalPackage' -v`

Expected: PASS.

**Step 5: Run the full spec package**

Run: `go test ./components/seaweedfs/spec -v`

Expected: PASS.

**Step 6: Commit the package path helper**

```bash
git add components/seaweedfs/spec
git commit -m "feat: validate local seaweedfs package"
```

### Task 4: Implement the master role and script generation

**Files:**
- Create: `components/seaweedfs/spec/master.go`
- Create: `pkg/cluster/template/scripts/seaweedfs_master.go`
- Create: `pkg/cluster/template/scripts/seaweedfs_master_test.go`
- Create: `embed/templates/scripts/run_seaweedfs_master.sh.tpl`
- Modify: `components/seaweedfs/spec/topology_seaweedfs.go`

**Step 1: Write the failing master script test**

```go
func TestSeaweedMasterScriptIncludesReplicationFlags(t *testing.T) {
	conf, err := os.CreateTemp("", "seaweed-master.*.sh")
	require.NoError(t, err)
	defer os.Remove(conf.Name())

	cfg := &SeaweedMasterScript{
		Port:               9333,
		DefaultReplication: "010",
		VolumeSizeLimitMB:  4096,
	}
	require.NoError(t, cfg.ConfigToFile(conf.Name()))
	content, err := os.ReadFile(conf.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), "-defaultReplication=010")
	require.Contains(t, string(content), "-volumeSizeLimitMB=4096")
}
```

**Step 2: Run the master script test**

Run: `go test ./pkg/cluster/template/scripts -run TestSeaweedMasterScriptIncludesReplicationFlags -v`

Expected: FAIL because the script helper and template do not exist.

**Step 3: Implement the master script and instance**

```go
type SeaweedMasterScript struct {
	Port               int
	DefaultReplication string
	VolumeSizeLimitMB  int
	IPBind             string
	DeployDir          string
	LogDir             string
}
```

```go
type MasterComponent struct{ Topology *Specification }

type MasterInstance struct {
	spec.BaseInstance
	topo *Specification
}

func (i *MasterInstance) Deploy(b *task.Builder, _ string, deployDir string, _ string, _ string, _ string) {
	installLocalPackage(b, i.topo.PackagePath, i.GetManageHost(), deployDir)
}
```

`InitConfig` must create `run_seaweedfs_master.sh`, call `BaseInstance.InitConfig`, and skip repository-backed config checks.

**Step 4: Run the master-specific tests**

Run: `go test ./pkg/cluster/template/scripts ./components/seaweedfs/spec -run 'TestSeaweedMasterScriptIncludesReplicationFlags|TestComponentsByStartOrder' -v`

Expected: PASS.

**Step 5: Run the full SeaweedFS test set**

Run: `go test ./components/seaweedfs/... ./pkg/cluster/template/scripts -v`

Expected: PASS.

**Step 6: Commit the master role**

```bash
git add components/seaweedfs/spec/master.go pkg/cluster/template/scripts/seaweedfs_master.go embed/templates/scripts/run_seaweedfs_master.sh.tpl
git commit -m "feat: add seaweedfs master role"
```

### Task 5: Implement the volume role with multi-path support

**Files:**
- Create: `components/seaweedfs/spec/volume.go`
- Create: `pkg/cluster/template/scripts/seaweedfs_volume.go`
- Create: `pkg/cluster/template/scripts/seaweedfs_volume_test.go`
- Create: `embed/templates/scripts/run_seaweedfs_volume.sh.tpl`
- Modify: `components/seaweedfs/spec/topology_seaweedfs.go`

**Step 1: Write the failing volume script test**

```go
func TestSeaweedVolumeScriptJoinsPathsAndDisks(t *testing.T) {
	conf, err := os.CreateTemp("", "seaweed-volume.*.sh")
	require.NoError(t, err)
	defer os.Remove(conf.Name())

	cfg := &SeaweedVolumeScript{
		Port:      8080,
		Dirs:      []string{"/data1/seaweedfs", "/data2/seaweedfs"},
		DiskTypes: []string{"ssd", "hdd"},
		DataCenter: "dc1",
		Rack:      "rack-a",
	}
	require.NoError(t, cfg.ConfigToFile(conf.Name()))
	content, err := os.ReadFile(conf.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), "-dir=/data1/seaweedfs,/data2/seaweedfs")
	require.Contains(t, string(content), "-disk=ssd,hdd")
	require.Contains(t, string(content), "-dataCenter=dc1")
	require.Contains(t, string(content), "-rack=rack-a")
}
```

**Step 2: Run the volume script test**

Run: `go test ./pkg/cluster/template/scripts -run TestSeaweedVolumeScriptJoinsPathsAndDisks -v`

Expected: FAIL because the helper is missing.

**Step 3: Implement the volume script and instance**

```go
type SeaweedVolumeScript struct {
	Port       int
	Dirs       []string
	DiskTypes  []string
	DataCenter string
	Rack       string
	DeployDir  string
	LogDir     string
	MasterAddr string
}
```

```go
func (s *VolumeSpec) dataDirs() []string {
	dirs := make([]string, 0, len(s.Paths))
	for _, p := range s.Paths {
		dirs = append(dirs, p.Path)
	}
	return dirs
}
```

Set `DataDir` on the spec to the comma-joined directory list and set `BaseInstance.Dirs` to the individual directories so conflict detection and cleanup still see every actual path.

**Step 4: Add a validation regression test**

```go
func TestVolumePathsRejectDuplicatePath(t *testing.T) {
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
      - path: "/data1/seaweedfs"
filer_servers:
  - host: 10.0.1.23
`)
	var topo Specification
	err := cspec.ParseTopologyYaml(topoFile, &topo)
	require.ErrorContains(t, err, "duplicate")
}
```

**Step 5: Run the volume-focused test set**

Run: `go test ./components/seaweedfs/spec ./pkg/cluster/template/scripts -run 'TestSeaweedVolumeScriptJoinsPathsAndDisks|TestVolumePathsRejectDuplicatePath' -v`

Expected: PASS.

**Step 6: Commit the volume role**

```bash
git add components/seaweedfs/spec/volume.go pkg/cluster/template/scripts/seaweedfs_volume.go embed/templates/scripts/run_seaweedfs_volume.sh.tpl
git commit -m "feat: add seaweedfs volume role"
```

### Task 6: Implement the filer role and TiDB cluster reference resolution

**Files:**
- Create: `components/seaweedfs/spec/filer.go`
- Create: `components/seaweedfs/spec/tidb_reference.go`
- Create: `components/seaweedfs/spec/filer_test.go`
- Create: `pkg/cluster/template/scripts/seaweedfs_filer.go`
- Create: `pkg/cluster/template/scripts/seaweedfs_filer_test.go`
- Create: `embed/templates/scripts/run_seaweedfs_filer.sh.tpl`

**Step 1: Write the failing TiDB reference resolution test**

```go
func TestResolveTiDBReferenceBuildsTiKVStoreConfig(t *testing.T) {
	clusterBaseDir := filepath.Join(t.TempDir(), cspec.TiUPClusterDir)
	writeTiDBMeta(t, clusterBaseDir, "tidb-prod", &cspec.ClusterMeta{
		Topology: &cspec.Specification{
			GlobalOptions: cspec.GlobalOptions{TLSEnabled: true},
			PDServers: []*cspec.PDSpec{
				{Host: "10.0.1.11", ClientPort: 2379},
				{Host: "10.0.1.12", ClientPort: 2379},
			},
		},
	})

	cfg, err := ResolveTiKVStore(clusterBaseDir, FilerStoreSpec{
		Type:            "tikv",
		FromTiDBCluster: "tidb-prod",
		KeyPrefix:       "swfs-prod",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.1.11:2379", "10.0.1.12:2379"}, cfg.PDEndpoints)
	require.True(t, cfg.TLSEnabled)
	require.Equal(t, "swfs-prod", cfg.KeyPrefix)
}
```

**Step 2: Run the filer reference test**

Run: `go test ./components/seaweedfs/spec -run TestResolveTiDBReferenceBuildsTiKVStoreConfig -v`

Expected: FAIL because `ResolveTiKVStore` and the TiDB metadata fixture helper do not exist.

**Step 3: Implement TiDB reference resolution**

```go
type ResolvedTiKVStore struct {
	PDEndpoints []string
	KeyPrefix   string
	TLSEnabled  bool
	CACertPath  string
	CertPath    string
	KeyPath     string
}

func ResolveTiKVStore(clusterBaseDir string, store FilerStoreSpec) (*ResolvedTiKVStore, error) {
	tidbSpecManager := cspec.NewSpec(clusterBaseDir, func() cspec.Metadata {
		return &cspec.ClusterMeta{Topology: new(cspec.Specification)}
	})
	meta := &cspec.ClusterMeta{Topology: new(cspec.Specification)}
	if err := tidbSpecManager.Metadata(store.FromTiDBCluster, meta); err != nil {
		return nil, err
	}
	return &ResolvedTiKVStore{
		PDEndpoints: meta.Topology.GetPDListWithManageHost(),
		KeyPrefix:   store.KeyPrefix,
		TLSEnabled:  meta.Topology.GlobalOptions.TLSEnabled,
	}, nil
}
```

Have `writeTiDBMeta` serialize a real `cspec.ClusterMeta` to `<clusterBaseDir>/<name>/meta.yaml` so the test matches `SpecManager.Metadata()`'s actual serialization format instead of fabricating a partial YAML fragment.

**Step 4: Write the failing filer script/config test**

```go
func TestSeaweedFilerScriptIncludesTiKVStoreConfig(t *testing.T) {
	conf, err := os.CreateTemp("", "seaweed-filer.*.sh")
	require.NoError(t, err)
	defer os.Remove(conf.Name())

	cfg := &SeaweedFilerScript{
		Port:        8888,
		PDEndpoints: []string{"10.0.1.11:2379", "10.0.1.12:2379"},
		KeyPrefix:   "swfs-prod",
		TLSEnabled:  true,
	}
	require.NoError(t, cfg.ConfigToFile(conf.Name()))
	content, err := os.ReadFile(conf.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), "10.0.1.11:2379,10.0.1.12:2379")
	require.Contains(t, string(content), "swfs-prod")
}
```

**Step 5: Implement the filer role and TLS copy behavior**

```go
type SeaweedFilerScript struct {
	Port        int
	PDEndpoints []string
	KeyPrefix   string
	TLSEnabled  bool
	CACertPath  string
	CertPath    string
	KeyPath     string
	DeployDir   string
	LogDir      string
}
```

In `FilerInstance.InitConfig`:

- resolve the TiDB reference once from the topology,
- copy referenced TLS files into `paths.Deploy + "/tls"` when TLS is enabled,
- compute the TiDB cluster base directory by targeting TiUP's `storage/cluster/clusters` tree instead of the SeaweedFS component profile,
- verify the pinned SeaweedFS version's TiKV backend field names and PD endpoint list format before hardcoding the filer config template,
- render the filer config or script using the resolved endpoints and `key_prefix`,
- skip repository-backed config checks.

**Step 6: Run the filer-specific tests**

Run: `go test ./components/seaweedfs/spec ./pkg/cluster/template/scripts -run 'TestResolveTiDBReferenceBuildsTiKVStoreConfig|TestSeaweedFilerScriptIncludesTiKVStoreConfig' -v`

Expected: PASS.

**Step 7: Commit the filer role**

```bash
git add components/seaweedfs/spec/filer.go components/seaweedfs/spec/tidb_reference.go pkg/cluster/template/scripts/seaweedfs_filer.go embed/templates/scripts/run_seaweedfs_filer.sh.tpl
git commit -m "feat: add seaweedfs filer role"
```

### Task 7: Wire deploy, start, stop, destroy, and display commands

**Files:**
- Create: `components/seaweedfs/command/deploy.go`
- Create: `components/seaweedfs/command/start.go`
- Create: `components/seaweedfs/command/stop.go`
- Create: `components/seaweedfs/command/destroy.go`
- Create: `components/seaweedfs/command/display.go`
- Create: `components/seaweedfs/command/deploy_test.go`
- Modify: `components/seaweedfs/command/root.go`

**Step 1: Write the failing deploy preflight test**

```go
func TestDeployRejectsPackageWithoutWeedBinary(t *testing.T) {
	packagePath := buildTarball(t, map[string]string{
		"README.md": "not weed",
	})
	topoFile := writeTopologyFile(t, `
package_path: "` + packagePath + `"
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

	err := validateDeployTopology(topoFile)
	require.Error(t, err)
}
```

**Step 2: Run the deploy preflight test**

Run: `go test ./components/seaweedfs/command -run TestDeployRejectsPackageWithoutWeedBinary -v`

Expected: FAIL because `validateDeployTopology` does not exist.

**Step 3: Implement deploy preflight and command wrappers**

```go
func validateDeployTopology(topoFile string) error {
	meta := seaweedfsSpec.GetSpecManager().NewMetadata()
	topo := meta.GetTopology()
	if err := cspec.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}
	swTopo := topo.(*spec.Specification)
	return spec.ValidateLocalPackage(swTopo.PackagePath)
}
```

Copy the `dm` command wrappers for `deploy`, `start`, `stop`, `destroy`, and `display`, but keep the SeaweedFS surface limited to the basic commands only.

`deploy` must call the preflight before `cm.Deploy`.

Keep the `dm`-style deploy signature and parse the user-supplied version with `utils.FmtVer(args[1])`. Pass that version to `cm.Deploy` so metadata, `display`, and future patching logic retain a meaningful version string even though the artifact source is `package_path`.

**Step 4: Register the new commands on the root command**

```go
rootCmd.AddCommand(
	newTemplateCmd(),
	newDeployCmd(),
	newStartCmd(),
	newStopCmd(),
	newDestroyCmd(),
	newDisplayCmd(),
)
```

**Step 5: Run the command package tests**

Run: `go test ./components/seaweedfs/command -v`

Expected: PASS.

**Step 6: Run the full SeaweedFS package tests**

Run: `go test ./components/seaweedfs/... -v`

Expected: PASS.

**Step 7: Commit the command wiring**

```bash
git add components/seaweedfs/command
git commit -m "feat: wire seaweedfs basic commands"
```

### Task 8: Final verification, linting, and operator-facing docs

**Files:**
- Create: `components/seaweedfs/README.md`
- Modify: `embed/examples/seaweedfs/topology.example.yaml`
- Modify: `components/seaweedfs/command/template.go`

**Step 1: Write the failing documentation assertion test**

```go
func TestTemplateIncludesLocalPackageAndTiDBReferenceComments(t *testing.T) {
	var out bytes.Buffer
	cmd := newTemplateCmd()
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "package_path")
	require.Contains(t, out.String(), "from_tidb_cluster")
}
```

**Step 2: Run the documentation-oriented template test**

Run: `go test ./components/seaweedfs/command -run TestTemplateIncludesLocalPackageAndTiDBReferenceComments -v`

Expected: PASS or FAIL only if the template comments are missing. If it already passes, keep the test and move on without changing code.

**Step 3: Write the operator README**

```md
# SeaweedFS Component

- `package_path` must point to a local tarball containing `weed`
- `filer_store.type` is limited to `tikv`
- `filer_store.from_tidb_cluster` must name an existing TiUP-managed TiDB cluster
- `volume.paths[]` are rendered into comma-separated `weed volume` flags
```

**Step 4: Run the targeted Go tests**

Run: `go test ./components/seaweedfs/... ./pkg/cluster/template/scripts -v`

Expected: PASS.

**Step 5: Run repository-wide lint and static checks**

Run: `make lint && make check-static`

Expected: PASS.

**Step 6: Build the new binary**

Run: `make build`

Expected: PASS and emit `bin/tiup-seaweedfs`.

**Step 7: Commit the finish-line changes**

```bash
git add components/seaweedfs embed/examples/seaweedfs pkg/cluster/template/scripts Makefile
git commit -m "chore: finalize seaweedfs basic line"
```

### Execution notes

- Use `@superpowers:test-driven-development` discipline inside each task even when the test is tiny.
- Keep each commit focused to the current task only.
- Do not start `scale-out`, `reload`, `patch`, `upgrade`, `s3`, or monitoring integration in this plan.
- If the SeaweedFS tarball format does not contain a top-level `weed` binary, stop and update Task 3 before touching the command or role tasks.

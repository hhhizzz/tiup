// Copyright 2025 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package spec

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/creasty/defaults"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
)

// Component names for SeaweedFS roles.
const (
	ComponentSeaweedMaster = "seaweedfs-master"
	ComponentSeaweedVolume = "seaweedfs-volume"
	ComponentSeaweedFiler  = "seaweedfs-filer"
)

type (
	// GlobalOptions of spec.
	GlobalOptions = spec.GlobalOptions
	// ResourceControl is the spec of ResourceControl.
	ResourceControl = meta.ResourceControl
)

type (
	// Component represents a component of the cluster.
	Component = spec.Component
	// Instance represents an instance.
	Instance = spec.Instance
	// InstanceSpec represent an instance specification.
	InstanceSpec = spec.InstanceSpec
)

// Compile-time interface satisfaction checks.
var _ spec.Topology = (*Specification)(nil)

// Type names used by reflection-based skip logic.
var (
	globalOptionTypeName   = reflect.TypeFor[GlobalOptions]().Name()
	filerStoreSpecTypeName = reflect.TypeFor[FilerStoreSpec]().Name()
)

// isSkipField returns true for non-slice struct fields that are not server lists.
func isSkipField(field reflect.Value) bool {
	if field.Kind() == reflect.Pointer {
		if field.IsZero() {
			return true
		}
		field = field.Elem()
	}
	tp := field.Type().Name()
	return tp == globalOptionTypeName || tp == filerStoreSpecTypeName
}

// --------------------------------------------------------------------------
// Spec structs
// --------------------------------------------------------------------------

// FilerStoreSpec describes where the filer stores its metadata.
type FilerStoreSpec struct {
	Type           string `yaml:"type"`              // must be "tikv"
	FromTiDBCluster string `yaml:"from_tidb_cluster"` // name of the TiDB cluster whose TiKV is reused
	KeyPrefix      string `yaml:"key_prefix"`        // key prefix inside the TiKV store
}

// VolumePathSpec describes a single storage path for a volume server.
type VolumePathSpec struct {
	Path     string `yaml:"path"`
	DiskType string `yaml:"disk_type,omitempty"` // e.g. "ssd", "hdd"
	MaxSize  int    `yaml:"max_size,omitempty"`  // in MB, 0 means use default
}

// MasterSpec represents the SeaweedFS master topology specification.
type MasterSpec struct {
	Host               string `yaml:"host"`
	ManageHost         string `yaml:"manage_host,omitempty"`
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

// Role returns the component role of the instance.
func (s *MasterSpec) Role() string { return ComponentSeaweedMaster }

// SSH returns the host and SSH port of the instance.
func (s *MasterSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance.
func (s *MasterSpec) GetMainPort() int { return s.Port }

// IgnoreMonitorAgent returns if the node does not have monitor agents available.
func (s *MasterSpec) IgnoreMonitorAgent() bool { return false }

// VolumeSpec represents the SeaweedFS volume topology specification.
type VolumeSpec struct {
	Host       string           `yaml:"host"`
	ManageHost string           `yaml:"manage_host,omitempty"`
	SSHPort    int              `yaml:"ssh_port,omitempty"`
	Port       int              `yaml:"port,omitempty" default:"8080"`
	Paths      []VolumePathSpec `yaml:"paths"`
	DataCenter string           `yaml:"data_center,omitempty"`
	Rack       string           `yaml:"rack,omitempty"`
	DeployDir  string           `yaml:"deploy_dir,omitempty"`
	DataDir    string           `yaml:"data_dir,omitempty"`
	LogDir     string           `yaml:"log_dir,omitempty"`
	Arch       string           `yaml:"arch,omitempty"`
	OS         string           `yaml:"os,omitempty"`
}

// Role returns the component role of the instance.
func (s *VolumeSpec) Role() string { return ComponentSeaweedVolume }

// SSH returns the host and SSH port of the instance.
func (s *VolumeSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance.
func (s *VolumeSpec) GetMainPort() int { return s.Port }

// IgnoreMonitorAgent returns if the node does not have monitor agents available.
func (s *VolumeSpec) IgnoreMonitorAgent() bool { return false }

// FilerSpec represents the SeaweedFS filer topology specification.
type FilerSpec struct {
	Host       string `yaml:"host"`
	ManageHost string `yaml:"manage_host,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty"`
	Port       int    `yaml:"port,omitempty" default:"8888"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	DataDir    string `yaml:"data_dir,omitempty"`
	LogDir     string `yaml:"log_dir,omitempty"`
	Arch       string `yaml:"arch,omitempty"`
	OS         string `yaml:"os,omitempty"`
}

// Role returns the component role of the instance.
func (s *FilerSpec) Role() string { return ComponentSeaweedFiler }

// SSH returns the host and SSH port of the instance.
func (s *FilerSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance.
func (s *FilerSpec) GetMainPort() int { return s.Port }

// IgnoreMonitorAgent returns if the node does not have monitor agents available.
func (s *FilerSpec) IgnoreMonitorAgent() bool { return false }

// --------------------------------------------------------------------------
// Specification – the main topology struct
// --------------------------------------------------------------------------

// Specification represents the specification of a SeaweedFS cluster topology.yaml.
type Specification struct {
	GlobalOptions GlobalOptions  `yaml:"global,omitempty" validate:"global:editable"`
	PackagePath   string         `yaml:"package_path"`
	FilerStore    FilerStoreSpec `yaml:"filer_store"`
	MasterServers []*MasterSpec  `yaml:"master_servers"`
	VolumeServers []*VolumeSpec  `yaml:"volume_servers"`
	FilerServers  []*FilerSpec   `yaml:"filer_servers"`
}

// AllSeaweedComponentNames contains the names of all SeaweedFS components.
func AllSeaweedComponentNames() (roles []string) {
	tp := &Specification{}
	tp.IterComponent(func(c Component) {
		roles = append(roles, c.Name())
	})
	return
}

// --------------------------------------------------------------------------
// UnmarshalYAML
// --------------------------------------------------------------------------

// UnmarshalYAML sets default values when unmarshaling the topology file.
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

// --------------------------------------------------------------------------
// Topology interface: metadata
// --------------------------------------------------------------------------

// Type implements Topology interface.
func (s *Specification) Type() string {
	return "seaweedfs-cluster"
}

// BaseTopo implements Topology interface.
func (s *Specification) BaseTopo() *spec.BaseTopo {
	return &spec.BaseTopo{
		GlobalOptions:    &s.GlobalOptions,
		MonitoredOptions: s.GetMonitoredOptions(),
	}
}

// NewPart implements ScaleOutTopology interface.
func (s *Specification) NewPart() spec.Topology {
	return &Specification{
		GlobalOptions: s.GlobalOptions,
	}
}

// MergeTopo implements ScaleOutTopology interface.
func (s *Specification) MergeTopo(rhs spec.Topology) spec.Topology {
	other, ok := rhs.(*Specification)
	if !ok {
		panic("topo should be SeaweedFS Topology")
	}
	return s.Merge(other)
}

// Merge returns a new Topology which sums the old ones.
func (s *Specification) Merge(that spec.Topology) spec.Topology {
	other := that.(*Specification)
	return &Specification{
		GlobalOptions: s.GlobalOptions,
		PackagePath:   s.PackagePath,
		FilerStore:    s.FilerStore,
		MasterServers: append(s.MasterServers, other.MasterServers...),
		VolumeServers: append(s.VolumeServers, other.VolumeServers...),
		FilerServers:  append(s.FilerServers, other.FilerServers...),
	}
}

// --------------------------------------------------------------------------
// Topology interface: component ordering
// --------------------------------------------------------------------------

// ComponentsByStartOrder returns components in the order they need to start.
func (s *Specification) ComponentsByStartOrder() (comps []Component) {
	comps = append(comps, &SeaweedMasterComponent{Topology: s})
	comps = append(comps, &SeaweedVolumeComponent{Topology: s})
	comps = append(comps, &SeaweedFilerComponent{Topology: s})
	return
}

// ComponentsByStopOrder returns components in the order they need to stop.
func (s *Specification) ComponentsByStopOrder() (comps []Component) {
	comps = s.ComponentsByStartOrder()
	// reverse
	i := 0
	j := len(comps) - 1
	for i < j {
		comps[i], comps[j] = comps[j], comps[i]
		i++
		j--
	}
	return
}

// ComponentsByUpdateOrder returns components in the order they need to be updated.
func (s *Specification) ComponentsByUpdateOrder(curVer string) (comps []Component) {
	return s.ComponentsByStartOrder()
}

// --------------------------------------------------------------------------
// Topology interface: iteration
// --------------------------------------------------------------------------

// IterInstance iterates all instances in component starting order.
func (s *Specification) IterInstance(fn func(instance Instance), concurrency ...int) {
	maxWorkers := 1
	wg := sync.WaitGroup{}
	if len(concurrency) > 0 && concurrency[0] > 1 {
		maxWorkers = concurrency[0]
	}
	workerPool := make(chan struct{}, maxWorkers)

	for _, comp := range s.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			wg.Add(1)
			workerPool <- struct{}{}
			go func(inst Instance) {
				defer func() {
					<-workerPool
					wg.Done()
				}()
				fn(inst)
			}(inst)
		}
	}
	wg.Wait()
}

// IterComponent iterates all components in component starting order.
func (s *Specification) IterComponent(fn func(comp Component)) {
	for _, comp := range s.ComponentsByStartOrder() {
		fn(comp)
	}
}

// --------------------------------------------------------------------------
// Topology interface: monitoring / TLS / grafana
// --------------------------------------------------------------------------

// GetMonitoredOptions returns nil; SeaweedFS does not have monitoring in this phase.
func (s *Specification) GetMonitoredOptions() *spec.MonitoredOptions {
	return nil
}

// TLSConfig returns nil; SeaweedFS does not use TLS in this phase.
func (s *Specification) TLSConfig(dir string) (*tls.Config, error) {
	return nil, nil
}

// GetGrafanaConfig returns nil for SeaweedFS.
func (s *Specification) GetGrafanaConfig() map[string]string {
	return nil
}

// FillHostArchOrOS fills the topology with the given host->arch or host->os mapping.
func (s *Specification) FillHostArchOrOS(hostArch map[string]string, fullType spec.FullHostType) error {
	return spec.FillHostArchOrOS(s, hostArch, fullType)
}

// --------------------------------------------------------------------------
// Topology interface: CountDir
// --------------------------------------------------------------------------

// CountDir counts directory paths used by any instance in the cluster with the same
// prefix, useful to find potential path conflicts.
func (s *Specification) CountDir(targetHost, dirPrefix string) int {
	dirTypes := []string{
		"DataDir",
		"DeployDir",
		"LogDir",
	}

	// host-path -> count
	dirStats := make(map[string]int)
	count := 0
	topoSpec := reflect.ValueOf(s).Elem()
	dirPrefix = spec.Abs(s.GlobalOptions.User, dirPrefix)

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		if compSpecs.Kind() != reflect.Slice {
			continue
		}
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			// Directory conflicts
			for _, dirType := range dirTypes {
				if j, found := findField(compSpec, dirType); found {
					dir := compSpec.Field(j).String()
					host := compSpec.FieldByName("Host").String()

					switch dirType {
					case "DataDir":
						deployDir := compSpec.FieldByName("DeployDir").String()
						if dir != "" && !strings.HasPrefix(dir, "/") {
							dir = filepath.Join(deployDir, dir)
						}
					case "LogDir":
						deployDir := compSpec.FieldByName("DeployDir").String()
						field := compSpec.FieldByName("LogDir")
						if field.IsValid() {
							dir = field.Interface().(string)
						}
						if dir == "" {
							dir = "log"
						}
						if !strings.HasPrefix(dir, "/") {
							dir = filepath.Join(deployDir, dir)
						}
					}
					dir = spec.Abs(s.GlobalOptions.User, dir)
					dirStats[host+dir]++
				}
			}
		}
	}

	for k, v := range dirStats {
		if k == targetHost+dirPrefix || strings.HasPrefix(k, targetHost+dirPrefix+"/") {
			count += v
		}
	}

	return count
}

// --------------------------------------------------------------------------
// Validation
// --------------------------------------------------------------------------

// Validate validates the topology specification and produces an error if the
// specification is invalid (e.g: port conflicts or directory conflicts).
func (s *Specification) Validate() error {
	// package_path must be non-empty and absolute
	if s.PackagePath == "" {
		return errors.New("package_path must be set")
	}
	if !filepath.IsAbs(s.PackagePath) {
		return errors.Errorf("package_path must be an absolute path, got %q", s.PackagePath)
	}

	// filer_store validations
	if s.FilerStore.Type != "tikv" {
		return errors.Errorf("filer_store.type must be \"tikv\", got %q", s.FilerStore.Type)
	}
	if s.FilerStore.FromTiDBCluster == "" {
		return errors.New("filer_store.from_tidb_cluster must be non-empty")
	}
	if s.FilerStore.KeyPrefix == "" {
		return errors.New("filer_store.key_prefix must be non-empty")
	}

	// volume path validations
	for idx, vs := range s.VolumeServers {
		if len(vs.Paths) == 0 {
			return errors.Errorf("volume_servers[%d] (%s) must have at least one path entry", idx, vs.Host)
		}
		seen := set.NewStringSet()
		for _, p := range vs.Paths {
			if p.Path == "" {
				return errors.Errorf("volume_servers[%d] (%s) contains empty volume path", idx, vs.Host)
			}
			if seen.Exist(p.Path) {
				return errors.Errorf("volume_servers[%d] (%s) has duplicate volume path %q", idx, vs.Host, p.Path)
			}
			seen.Insert(p.Path)
		}
	}

	if err := s.platformConflictsDetect(); err != nil {
		return err
	}

	if err := s.portConflictsDetect(); err != nil {
		return err
	}

	return s.dirConflictsDetect()
}

// platformConflictsDetect checks for conflicts in topology for different OS / Arch
// on the same host / IP.
func (s *Specification) platformConflictsDetect() error {
	type conflict struct {
		os   string
		arch string
		cfg  string
	}

	platformStats := map[string]conflict{}
	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeFor[Specification]()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		if compSpecs.Kind() != reflect.Slice {
			continue
		}
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			host := compSpec.FieldByName("Host").String()
			cfg := topoType.Field(i).Tag.Get("yaml")
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}

			stat := conflict{cfg: cfg}
			if j, found := findField(compSpec, "OS"); found {
				stat.os = compSpec.Field(j).String()
			}
			if j, found := findField(compSpec, "Arch"); found {
				stat.arch = compSpec.Field(j).String()
			}

			prev, exist := platformStats[host]
			if exist {
				if prev.os != stat.os || prev.arch != stat.arch {
					return &meta.ValidateErr{
						Type:   meta.TypeMismatch,
						Target: "platform",
						LHS:    fmt.Sprintf("%s:%s/%s", prev.cfg, prev.os, prev.arch),
						RHS:    fmt.Sprintf("%s:%s/%s", stat.cfg, stat.os, stat.arch),
						Value:  host,
					}
				}
			}
			platformStats[host] = stat
		}
	}
	return nil
}

// portConflictsDetect checks for port conflicts across all server specs.
func (s *Specification) portConflictsDetect() error {
	type (
		usedPort struct {
			host string
			port int
		}
		conflict struct {
			tp  string
			cfg string
		}
	)

	portTypes := []string{
		"Port",
	}

	portStats := map[usedPort]conflict{}
	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeFor[Specification]()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		if compSpecs.Kind() != reflect.Slice {
			continue
		}
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			host := compSpec.FieldByName("Host").String()
			cfg := topoType.Field(i).Tag.Get("yaml")
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}

			for _, portType := range portTypes {
				if j, found := findField(compSpec, portType); found {
					item := usedPort{
						host: host,
						port: int(compSpec.Field(j).Int()),
					}
					tp := compSpec.Type().Field(j).Tag.Get("yaml")
					prev, exist := portStats[item]
					if exist {
						return &meta.ValidateErr{
							Type:   meta.TypeConflict,
							Target: "port",
							LHS:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
							RHS:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
							Value:  item.port,
						}
					}
					portStats[item] = conflict{
						tp:  tp,
						cfg: cfg,
					}
				}
			}
		}
	}

	return nil
}

// dirConflictsDetect checks for directory conflicts across all server specs.
func (s *Specification) dirConflictsDetect() error {
	type (
		usedDir struct {
			host string
			dir  string
		}
		conflict struct {
			tp  string
			cfg string
		}
	)

	dirTypes := []string{
		"DataDir",
		"DeployDir",
	}

	dirStats := map[usedDir]conflict{}
	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeFor[Specification]()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		if compSpecs.Kind() != reflect.Slice {
			continue
		}
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			host := compSpec.FieldByName("Host").String()
			cfg := topoType.Field(i).Tag.Get("yaml")
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}

			for _, dirType := range dirTypes {
				if j, found := findField(compSpec, dirType); found {
					item := usedDir{
						host: host,
						dir:  compSpec.Field(j).String(),
					}
					// data_dir is relative to deploy_dir by default, so they can be with
					// same (sub) paths as long as the deploy_dirs are different
					if item.dir != "" && !strings.HasPrefix(item.dir, "/") {
						continue
					}
					tp := strings.Split(compSpec.Type().Field(j).Tag.Get("yaml"), ",")[0]
					prev, exist := dirStats[item]
					if exist {
						return &meta.ValidateErr{
							Type:   meta.TypeConflict,
							Target: "directory",
							LHS:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
							RHS:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
							Value:  item.dir,
						}
					}
					dirStats[item] = conflict{
						tp:  tp,
						cfg: cfg,
					}
				}
			}
		}
	}

	return nil
}

// --------------------------------------------------------------------------
// Custom defaults (modeled after DM's fillDMCustomDefaults)
// --------------------------------------------------------------------------

// fillSeaweedCustomDefaults tries to fill custom fields to their default values.
func fillSeaweedCustomDefaults(globalOptions *GlobalOptions, data any) error {
	v := reflect.ValueOf(data).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		if err := setSeaweedCustomDefaults(globalOptions, v.Field(i)); err != nil {
			return err
		}
	}

	return nil
}

func setSeaweedCustomDefaults(globalOptions *GlobalOptions, field reflect.Value) error {
	if !field.CanSet() || isSkipField(field) {
		return nil
	}

	switch field.Kind() {
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			if err := setSeaweedCustomDefaults(globalOptions, field.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		ref := reflect.New(field.Type())
		ref.Elem().Set(field)
		if err := fillSeaweedCustomDefaults(globalOptions, ref.Interface()); err != nil {
			return err
		}
		field.Set(ref.Elem())
	case reflect.Pointer:
		if err := setSeaweedCustomDefaults(globalOptions, field.Elem()); err != nil {
			return err
		}
	}

	if field.Kind() != reflect.Struct {
		return nil
	}

	for j := 0; j < field.NumField(); j++ {
		switch field.Type().Field(j).Name {
		case "SSHPort":
			if field.Field(j).Int() != 0 {
				continue
			}
			field.Field(j).Set(reflect.ValueOf(globalOptions.SSHPort))
		case "DataDir":
			dataDir := field.Field(j).String()
			if dataDir != "" {
				continue
			}
			if strings.HasPrefix(globalOptions.DataDir, "/") {
				field.Field(j).Set(reflect.ValueOf(filepath.Join(
					globalOptions.DataDir,
					fmt.Sprintf("%s-%s", field.Addr().Interface().(InstanceSpec).Role(), getPort(field)),
				)))
				continue
			}
			if globalOptions.DataDir == "" {
				field.Field(j).Set(reflect.ValueOf("data"))
			} else {
				field.Field(j).Set(reflect.ValueOf(globalOptions.DataDir))
			}
		case "DeployDir":
			setDefaultDir(globalOptions.DeployDir, field.Addr().Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
		case "LogDir":
			if field.Field(j).String() == "" && defaults.CanUpdate(field.Field(j).Interface()) {
				field.Field(j).Set(reflect.ValueOf(globalOptions.LogDir))
			}
		case "Arch":
			switch strings.ToLower(field.Field(j).String()) {
			case "x86_64":
				field.Field(j).Set(reflect.ValueOf("amd64"))
			case "aarch64":
				field.Field(j).Set(reflect.ValueOf("arm64"))
			}
			if field.Field(j).String() != "" {
				field.Field(j).Set(reflect.ValueOf(strings.ToLower(field.Field(j).String())))
			}
		case "OS":
			if field.Field(j).String() != "" {
				field.Field(j).Set(reflect.ValueOf(strings.ToLower(field.Field(j).String())))
			}
		}
	}

	return nil
}

// --------------------------------------------------------------------------
// Reflection helpers
// --------------------------------------------------------------------------

func setDefaultDir(parent, role, port string, field reflect.Value) {
	if field.String() != "" {
		return
	}
	if defaults.CanUpdate(field.Interface()) {
		dir := fmt.Sprintf("%s-%s", role, port)
		field.Set(reflect.ValueOf(filepath.Join(parent, dir)))
	}
}

func findField(v reflect.Value, fieldName string) (int, bool) {
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == fieldName {
			return i, true
		}
	}
	return -1, false
}

func getPort(v reflect.Value) string {
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == "Port" {
			return fmt.Sprintf("%d", v.Field(i).Int())
		}
	}
	return ""
}

// --------------------------------------------------------------------------
// Minimal component structs (full implementations will come in later tasks)
// --------------------------------------------------------------------------

// SeaweedMasterComponent represents the SeaweedFS master component.
type SeaweedMasterComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *SeaweedMasterComponent) Name() string { return ComponentSeaweedMaster }

// Role implements Component interface.
func (c *SeaweedMasterComponent) Role() string { return ComponentSeaweedMaster }

// Source implements Component interface.
func (c *SeaweedMasterComponent) Source() string { return ComponentSeaweedMaster }

// CalculateVersion implements Component interface.
func (c *SeaweedMasterComponent) CalculateVersion(clusterVersion string) string {
	return clusterVersion
}

// SetVersion implements Component interface.
func (c *SeaweedMasterComponent) SetVersion(version string) {}

// Instances implements Component interface.
func (c *SeaweedMasterComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.MasterServers))
	for _, s := range c.Topology.MasterServers {
		ins = append(ins, &MasterInstance{
			BaseInstance: spec.BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				ManageHost:   s.ManageHost,
				ListenHost:   c.Topology.BaseTopo().GlobalOptions.ListenHost,
				Port:         s.Port,
				SSHP:         s.SSHPort,
				Source:       ComponentSeaweedMaster,

				Ports: []int{
					s.Port,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: func(_ context.Context, _ time.Duration, _ *tls.Config, _ ...string) string {
					return "-"
				},
				UptimeFn: func(_ context.Context, _ time.Duration, _ *tls.Config) time.Duration {
					return 0
				},
				Component: c,
			},
			topo: c.Topology,
		})
	}
	return ins
}

// SeaweedVolumeComponent represents the SeaweedFS volume component.
type SeaweedVolumeComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *SeaweedVolumeComponent) Name() string { return ComponentSeaweedVolume }

// Role implements Component interface.
func (c *SeaweedVolumeComponent) Role() string { return ComponentSeaweedVolume }

// Source implements Component interface.
func (c *SeaweedVolumeComponent) Source() string { return ComponentSeaweedVolume }

// CalculateVersion implements Component interface.
func (c *SeaweedVolumeComponent) CalculateVersion(clusterVersion string) string {
	return clusterVersion
}

// SetVersion implements Component interface.
func (c *SeaweedVolumeComponent) SetVersion(version string) {}

// Instances implements Component interface.
func (c *SeaweedVolumeComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.VolumeServers))
	for _, s := range c.Topology.VolumeServers {
		dirs := append([]string{s.DeployDir}, s.dataDirs()...)
		ins = append(ins, &VolumeInstance{
			BaseInstance: spec.BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				ManageHost:   s.ManageHost,
				ListenHost:   c.Topology.BaseTopo().GlobalOptions.ListenHost,
				Port:         s.Port,
				SSHP:         s.SSHPort,
				Source:       ComponentSeaweedVolume,

				Ports: []int{
					s.Port,
				},
				Dirs: dirs,
				StatusFn: func(_ context.Context, _ time.Duration, _ *tls.Config, _ ...string) string {
					return "-"
				},
				UptimeFn: func(_ context.Context, _ time.Duration, _ *tls.Config) time.Duration {
					return 0
				},
				Component: c,
			},
			topo: c.Topology,
		})
	}
	return ins
}

// SeaweedFilerComponent represents the SeaweedFS filer component.
type SeaweedFilerComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *SeaweedFilerComponent) Name() string { return ComponentSeaweedFiler }

// Role implements Component interface.
func (c *SeaweedFilerComponent) Role() string { return ComponentSeaweedFiler }

// Source implements Component interface.
func (c *SeaweedFilerComponent) Source() string { return ComponentSeaweedFiler }

// CalculateVersion implements Component interface.
func (c *SeaweedFilerComponent) CalculateVersion(clusterVersion string) string {
	return clusterVersion
}

// SetVersion implements Component interface.
func (c *SeaweedFilerComponent) SetVersion(version string) {}

// Instances implements Component interface.
func (c *SeaweedFilerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.FilerServers))
	for _, s := range c.Topology.FilerServers {
		ins = append(ins, &FilerInstance{
			BaseInstance: spec.BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				ManageHost:   s.ManageHost,
				ListenHost:   c.Topology.BaseTopo().GlobalOptions.ListenHost,
				Port:         s.Port,
				SSHP:         s.SSHPort,
				Source:       ComponentSeaweedFiler,

				Ports: []int{
					s.Port,
				},
				Dirs: []string{
					s.DeployDir,
				},
				StatusFn: func(_ context.Context, _ time.Duration, _ *tls.Config, _ ...string) string {
					return "-"
				},
				UptimeFn: func(_ context.Context, _ time.Duration, _ *tls.Config) time.Duration {
					return 0
				},
				Component: c,
			},
			topo: c.Topology,
		})
	}
	return ins
}

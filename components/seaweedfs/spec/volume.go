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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// dataDirs returns the list of storage directory paths from the volume spec.
func (s *VolumeSpec) dataDirs() []string {
	dirs := make([]string, 0, len(s.Paths))
	for _, p := range s.Paths {
		dirs = append(dirs, p.Path)
	}
	return dirs
}

// diskTypes returns the list of disk types from the volume spec.
func (s *VolumeSpec) diskTypes() []string {
	types := make([]string, 0, len(s.Paths))
	for _, p := range s.Paths {
		if p.DiskType != "" {
			types = append(types, p.DiskType)
		}
	}
	return types
}

// VolumeInstance represents a SeaweedFS volume instance.
type VolumeInstance struct {
	spec.BaseInstance
	topo *Specification
}

// DataDir returns the configured volume paths as a comma-separated list so the
// manager creates and tracks the real storage directories instead of the
// default fallback data dir.
func (i *VolumeInstance) DataDir() string {
	return strings.Join(i.InstanceSpec.(*VolumeSpec).dataDirs(), ",")
}

// Deploy implements manager.DeployerInstance.
func (i *VolumeInstance) Deploy(b *task.Builder, _ string, deployDir string, _ string, _ string, _ string) {
	installLocalPackage(b, i.topo.PackagePath, i.GetManageHost(), deployDir)
}

// InitConfig implements Instance interface.
func (i *VolumeInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	if err := i.BaseInstance.InitConfig(ctx, e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	s := i.InstanceSpec.(*VolumeSpec)

	// Build master address list
	masters := make([]string, 0, len(i.topo.MasterServers))
	for _, ms := range i.topo.MasterServers {
		masters = append(masters, utils.JoinHostPort(ms.Host, ms.Port))
	}

	cfg := &scripts.SeaweedVolumeScript{
		Port:       s.Port,
		RawDirs:    s.dataDirs(),
		RawDisks:   s.diskTypes(),
		DataCenter: s.DataCenter,
		Rack:       s.Rack,
		IPBind:     i.GetListenHost(),
		MasterAddr: strings.Join(masters, ","),
		DeployDir:  paths.Deploy,
		LogDir:     paths.Log,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_seaweedfs_volume_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_seaweedfs-volume.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	return err
}

// ScaleConfig implements Instance interface.
func (i *VolumeInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo spec.Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo.(*Specification)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

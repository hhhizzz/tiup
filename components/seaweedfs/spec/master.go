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

// MasterInstance represents a SeaweedFS master instance.
type MasterInstance struct {
	spec.BaseInstance
	topo *Specification
}

// Deploy implements manager.DeployerInstance.
func (i *MasterInstance) Deploy(b *task.Builder, _ string, deployDir string, _ string, _ string, _ string) {
	installLocalPackage(b, i.topo.PackagePath, i.GetManageHost(), deployDir)
}

// InitConfig implements Instance interface.
func (i *MasterInstance) InitConfig(
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

	s := i.InstanceSpec.(*MasterSpec)

	// Build peers list: all master host:port pairs
	peers := "none"
	if len(i.topo.MasterServers) > 1 {
		peerList := make([]string, 0, len(i.topo.MasterServers))
		for _, ms := range i.topo.MasterServers {
			peerList = append(peerList, utils.JoinHostPort(ms.Host, ms.Port))
		}
		peers = strings.Join(peerList, ",")
	}

	cfg := &scripts.SeaweedMasterScript{
		Port:               s.Port,
		DefaultReplication: s.DefaultReplication,
		IP:                 s.Host,
		VolumeSizeLimitMB:  s.VolumeSizeLimitMB,
		IPBind:             i.GetListenHost(),
		Peers:              peers,
		DeployDir:          paths.Deploy,
		DataDir:            paths.Data[0],
		LogDir:             paths.Log,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_seaweedfs_master_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_seaweedfs-master.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	return err
}

// ScaleConfig implements Instance interface.
func (i *MasterInstance) ScaleConfig(
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

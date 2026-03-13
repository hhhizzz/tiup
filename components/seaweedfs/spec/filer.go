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
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// FilerInstance represents a SeaweedFS filer instance.
type FilerInstance struct {
	spec.BaseInstance
	topo *Specification
}

func buildTiKVFilerToml(store *ResolvedTiKVStore) string {
	var b strings.Builder

	b.WriteString("[tikv]\n")
	b.WriteString("enabled = true\n")
	b.WriteString(fmt.Sprintf("pdaddrs = %q\n", strings.Join(store.PDEndpoints, ",")))
	b.WriteString(fmt.Sprintf("keyPrefix = %q\n", store.KeyPrefix))

	if store.TLSEnabled {
		b.WriteString(fmt.Sprintf("ca_path = %q\n", store.CACertPath))
		b.WriteString(fmt.Sprintf("cert_path = %q\n", store.CertPath))
		b.WriteString(fmt.Sprintf("key_path = %q\n", store.KeyPath))
	}

	return b.String()
}

// Deploy implements manager.DeployerInstance.
func (i *FilerInstance) Deploy(b *task.Builder, _ string, deployDir string, _ string, _ string, _ string) {
	installLocalPackage(b, i.topo.PackagePath, i.GetManageHost(), deployDir)
}

// InitConfig implements Instance interface.
func (i *FilerInstance) InitConfig(
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

	clusterBaseDir, err := TiDBClusterBaseDir()
	if err != nil {
		return err
	}
	storeCfg, err := ResolveTiKVStore(clusterBaseDir, i.topo.FilerStore)
	if err != nil {
		return err
	}

	if storeCfg.TLSEnabled {
		tlsDir := filepath.Join(paths.Deploy, "tls")
		if _, _, err := e.Execute(ctx, fmt.Sprintf("mkdir -p %q", tlsDir), false); err != nil {
			return err
		}

		remoteCACert := filepath.Join(tlsDir, spec.TLSCACert)
		remoteCert := filepath.Join(tlsDir, spec.TLSClientCert)
		remoteKey := filepath.Join(tlsDir, spec.TLSClientKey)
		for _, pair := range []struct {
			src string
			dst string
		}{
			{src: storeCfg.CACertPath, dst: remoteCACert},
			{src: storeCfg.CertPath, dst: remoteCert},
			{src: storeCfg.KeyPath, dst: remoteKey},
		} {
			if err := e.Transfer(ctx, pair.src, pair.dst, false, 0, false); err != nil {
				return err
			}
		}
		storeCfg.CACertPath = remoteCACert
		storeCfg.CertPath = remoteCert
		storeCfg.KeyPath = remoteKey
	}

	filerTomlPath := filepath.Join(paths.Cache, fmt.Sprintf("seaweedfs_filer_%s_%d.toml", i.GetHost(), i.GetPort()))
	if err := os.WriteFile(filerTomlPath, []byte(buildTiKVFilerToml(storeCfg)), 0644); err != nil {
		return err
	}
	if err := e.Transfer(ctx, filerTomlPath, filepath.Join(paths.Deploy, "filer.toml"), false, 0, false); err != nil {
		return err
	}

	// Build master address list
	masters := make([]string, 0, len(i.topo.MasterServers))
	for _, ms := range i.topo.MasterServers {
		masters = append(masters, utils.JoinHostPort(ms.Host, ms.Port))
	}

	cfg := &scripts.SeaweedFilerScript{
		Port:       i.InstanceSpec.(*FilerSpec).Port,
		IPBind:     i.GetListenHost(),
		MasterAddr: strings.Join(masters, ","),
		DeployDir:  paths.Deploy,
		LogDir:     paths.Log,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_seaweedfs_filer_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_seaweedfs-filer.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err = e.Execute(ctx, "chmod +x "+dst, false)
	return err
}

// ScaleConfig implements Instance interface.
func (i *FilerInstance) ScaleConfig(
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

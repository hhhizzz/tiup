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
	"os"
	"path/filepath"
	"testing"
	"time"

	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type filerMockExecutor struct{}

func (e *filerMockExecutor) Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	return nil, nil, nil
}

func (e *filerMockExecutor) Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) error {
	if download {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, content, 0644)
}

// writeTiDBMeta serializes a ClusterMeta to <baseDir>/<name>/meta.yaml so
// that SpecManager.Metadata() can read it.
func writeTiDBMeta(t *testing.T, baseDir, name string, meta *cspec.ClusterMeta) {
	t.Helper()
	dir := filepath.Join(baseDir, name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	data, err := yaml.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.yaml"), data, 0644))
}

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

func TestResolveTiDBReferenceRejectsUnknownCluster(t *testing.T) {
	clusterBaseDir := filepath.Join(t.TempDir(), cspec.TiUPClusterDir)
	_, err := ResolveTiKVStore(clusterBaseDir, FilerStoreSpec{
		Type:            "tikv",
		FromTiDBCluster: "no-such-cluster",
		KeyPrefix:       "pfx",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-such-cluster")
}

func TestResolveTiDBReferenceRejectsNoPD(t *testing.T) {
	clusterBaseDir := filepath.Join(t.TempDir(), cspec.TiUPClusterDir)
	writeTiDBMeta(t, clusterBaseDir, "tidb-empty", &cspec.ClusterMeta{
		Topology: &cspec.Specification{},
	})

	_, err := ResolveTiKVStore(clusterBaseDir, FilerStoreSpec{
		Type:            "tikv",
		FromTiDBCluster: "tidb-empty",
		KeyPrefix:       "pfx",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no PD servers")
}

func TestFilerInitConfigWritesTiKVStoreConfig(t *testing.T) {
	tiupHome := t.TempDir()
	t.Setenv(localdata.EnvNameHome, tiupHome)

	clusterBaseDir := filepath.Join(tiupHome, localdata.StorageParentDir, "cluster", cspec.TiUPClusterDir)
	writeTiDBMeta(t, clusterBaseDir, "tidb-prod", &cspec.ClusterMeta{
		Topology: &cspec.Specification{
			GlobalOptions: cspec.GlobalOptions{TLSEnabled: true},
			PDServers: []*cspec.PDSpec{
				{Host: "10.0.1.11", ClientPort: 2379},
				{Host: "10.0.1.12", ClientPort: 2379},
			},
		},
	})

	tlsDir := filepath.Join(clusterBaseDir, "tidb-prod", cspec.TLSCertKeyDir)
	require.NoError(t, os.MkdirAll(tlsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tlsDir, cspec.TLSCACert), []byte("ca"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tlsDir, cspec.TLSClientCert), []byte("cert"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tlsDir, cspec.TLSClientKey), []byte("key"), 0644))

	deployDir := t.TempDir()
	cacheDir := t.TempDir()
	logDir := filepath.Join(deployDir, "log")
	require.NoError(t, os.MkdirAll(logDir, 0755))

	topo := &Specification{
		GlobalOptions: GlobalOptions{
			User:        "tidb",
			SystemdMode: cspec.UserMode,
		},
		FilerStore: FilerStoreSpec{
			Type:            "tikv",
			FromTiDBCluster: "tidb-prod",
			KeyPrefix:       "swfs-prod",
		},
		MasterServers: []*MasterSpec{
			{Host: "10.0.1.21", Port: 9333},
		},
		FilerServers: []*FilerSpec{
			{Host: "127.0.0.1", Port: 8888, DeployDir: deployDir, LogDir: logDir},
		},
	}

	instance := (&SeaweedFilerComponent{Topology: topo}).Instances()[0].(*FilerInstance)
	paths := meta.DirPaths{
		Deploy: deployDir,
		Cache:  cacheDir,
		Log:    logDir,
	}

	require.NoError(t, instance.InitConfig(context.Background(), &filerMockExecutor{}, "swfs-test", "v1.0.0", "tidb", paths))

	content, err := os.ReadFile(filepath.Join(deployDir, "filer.toml"))
	require.NoError(t, err)
	require.Contains(t, string(content), "[tikv]")
	require.Contains(t, string(content), "enabled = true")
	require.Contains(t, string(content), "pdaddrs = \"10.0.1.11:2379,10.0.1.12:2379\"")
	require.Contains(t, string(content), "keyPrefix = \"swfs-prod\"")
	require.Contains(t, string(content), filepath.Join(deployDir, "tls", cspec.TLSCACert))
	require.Contains(t, string(content), filepath.Join(deployDir, "tls", cspec.TLSClientCert))
	require.Contains(t, string(content), filepath.Join(deployDir, "tls", cspec.TLSClientKey))

	_, err = os.Stat(filepath.Join(deployDir, "tls", cspec.TLSCACert))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(deployDir, "tls", cspec.TLSClientCert))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(deployDir, "tls", cspec.TLSClientKey))
	require.NoError(t, err)
}

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
	"os"
	"path/filepath"

	"github.com/pingcap/errors"
	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/localdata"
)

// ResolvedTiKVStore holds the resolved TiKV backend configuration for the filer.
type ResolvedTiKVStore struct {
	PDEndpoints []string
	KeyPrefix   string
	TLSEnabled  bool
	CACertPath  string
	CertPath    string
	KeyPath     string
}

// ResolveTiKVStore reads a TiDB cluster's metadata from the given cluster base
// directory and extracts PD endpoints and TLS information for filer metadata storage.
func ResolveTiKVStore(clusterBaseDir string, store FilerStoreSpec) (*ResolvedTiKVStore, error) {
	tidbSpecManager := cspec.NewSpec(clusterBaseDir, func() cspec.Metadata {
		return &cspec.ClusterMeta{Topology: new(cspec.Specification)}
	})
	meta := &cspec.ClusterMeta{Topology: new(cspec.Specification)}
	if err := tidbSpecManager.Metadata(store.FromTiDBCluster, meta); err != nil {
		return nil, errors.Annotatef(err, "failed to read TiDB cluster %q metadata", store.FromTiDBCluster)
	}

	pdList := meta.Topology.GetPDListWithManageHost()
	if len(pdList) == 0 {
		return nil, errors.Errorf("TiDB cluster %q has no PD servers", store.FromTiDBCluster)
	}

	return &ResolvedTiKVStore{
		PDEndpoints: pdList,
		KeyPrefix:   store.KeyPrefix,
		TLSEnabled:  meta.Topology.GlobalOptions.TLSEnabled,
		CACertPath:  filepath.Join(clusterBaseDir, store.FromTiDBCluster, cspec.TLSCertKeyDir, cspec.TLSCACert),
		CertPath:    filepath.Join(clusterBaseDir, store.FromTiDBCluster, cspec.TLSCertKeyDir, cspec.TLSClientCert),
		KeyPath:     filepath.Join(clusterBaseDir, store.FromTiDBCluster, cspec.TLSCertKeyDir, cspec.TLSClientKey),
	}, nil
}

// TiDBClusterBaseDir computes the storage base directory for TiUP-managed TiDB
// clusters. This is independent of the SeaweedFS component's own profile directory.
func TiDBClusterBaseDir() (string, error) {
	var tiupRoot string
	switch {
	case os.Getenv(localdata.EnvNameHome) != "":
		tiupRoot = os.Getenv(localdata.EnvNameHome)
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.Trace(err)
		}
		tiupRoot = filepath.Join(home, ".tiup")
	}
	return filepath.Join(tiupRoot, localdata.StorageParentDir, "cluster", cspec.TiUPClusterDir), nil
}

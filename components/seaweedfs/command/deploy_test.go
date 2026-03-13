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

package command

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func buildTarball(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.tar.gz")
	f, err := os.Create(p)
	require.NoError(t, err)
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	return p
}

func writeTopologyFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "topology.yaml")
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))
	return f
}

func TestDeployRejectsPackageWithoutWeedBinary(t *testing.T) {
	packagePath := buildTarball(t, map[string]string{
		"README.md": "not weed",
	})
	topoFile := writeTopologyFile(t, `
package_path: "`+packagePath+`"
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

func TestDeployAcceptsValidPackage(t *testing.T) {
	packagePath := buildTarball(t, map[string]string{
		"weed": "#!/bin/sh\necho seaweedfs",
	})
	topoFile := writeTopologyFile(t, `
package_path: "`+packagePath+`"
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
	require.NoError(t, err)
}

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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTopologyFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "topology.yaml")
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))
	return f
}

func TestDeployRejectsLegacyPackagePathField(t *testing.T) {
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

	err := validateDeployTopology(topoFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "package_path")
}

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

	err := validateDeployTopology(topoFile)
	require.NoError(t, err)
}

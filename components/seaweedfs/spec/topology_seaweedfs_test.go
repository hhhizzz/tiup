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
	"testing"

	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/stretchr/testify/require"
)

func writeTopologyFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "topology.yaml")
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))
	return f
}

func TestTopologyDefaultsAndValidation(t *testing.T) {
	topoFile := writeTopologyFile(t, `
global:
  user: "tidb"
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
	require.Equal(t, "001", topo.MasterServers[0].DefaultReplication)
	require.Equal(t, 9333, topo.MasterServers[0].Port)
	require.Equal(t, 8080, topo.VolumeServers[0].Port)
	require.Equal(t, 8888, topo.FilerServers[0].Port)
	require.Equal(t, "/data1/seaweedfs", topo.VolumeServers[0].Paths[0].Path)
}

func TestTopologyRejectsEmptyVolumePaths(t *testing.T) {
	topoFile := writeTopologyFile(t, `
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "volume_servers")
}

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

func TestTopologyRejectsDuplicateVolumePaths(t *testing.T) {
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
      - path: "/data1/seaweedfs"
filer_servers:
  - host: 10.0.1.23
`)
	var topo Specification
	err := cspec.ParseTopologyYaml(topoFile, &topo)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestInstancesCreation(t *testing.T) {
	topoFile := writeTopologyFile(t, `
global:
  user: "tidb"
filer_store:
  type: tikv
  from_tidb_cluster: "tidb-prod"
  key_prefix: "swfs-prod"
master_servers:
  - host: 10.0.1.21
  - host: 10.0.1.22
volume_servers:
  - host: 10.0.1.31
    paths:
      - path: "/data1/seaweedfs"
filer_servers:
  - host: 10.0.1.41
`)
	var topo Specification
	require.NoError(t, cspec.ParseTopologyYaml(topoFile, &topo))

	// Master instances
	masterComp := &SeaweedMasterComponent{Topology: &topo}
	masters := masterComp.Instances()
	require.Len(t, masters, 2)
	require.Equal(t, "10.0.1.21", masters[0].GetHost())
	require.Equal(t, 9333, masters[0].GetPort())
	require.Equal(t, "10.0.1.22", masters[1].GetHost())

	// Volume instances
	volumeComp := &SeaweedVolumeComponent{Topology: &topo}
	volumes := volumeComp.Instances()
	require.Len(t, volumes, 1)
	require.Equal(t, "10.0.1.31", volumes[0].GetHost())
	require.Equal(t, 8080, volumes[0].GetPort())
	require.Equal(t, "/data1/seaweedfs", volumes[0].DataDir())
	require.Contains(t, volumes[0].UsedDirs(), "/data1/seaweedfs")

	// Filer instances
	filerComp := &SeaweedFilerComponent{Topology: &topo}
	filers := filerComp.Instances()
	require.Len(t, filers, 1)
	require.Equal(t, "10.0.1.41", filers[0].GetHost())
	require.Equal(t, 8888, filers[0].GetPort())

	// IterInstance should visit all 4 instances
	var visited int
	topo.IterInstance(func(_ cspec.Instance) { visited++ })
	require.Equal(t, 4, visited)
}

func TestVolumeInstanceExposesConfiguredPathsAsDataDirs(t *testing.T) {
	topoFile := writeTopologyFile(t, `
global:
  user: "tidb"
filer_store:
  type: tikv
  from_tidb_cluster: "tidb-prod"
  key_prefix: "swfs-prod"
master_servers:
  - host: 10.0.1.21
volume_servers:
  - host: 10.0.1.31
    paths:
      - path: "/data1/seaweedfs"
      - path: "/data2/seaweedfs"
filer_servers:
  - host: 10.0.1.41
`)
	var topo Specification
	require.NoError(t, cspec.ParseTopologyYaml(topoFile, &topo))

	volume := (&SeaweedVolumeComponent{Topology: &topo}).Instances()[0]
	require.Equal(t, "/data1/seaweedfs,/data2/seaweedfs", volume.DataDir())
	require.Contains(t, volume.UsedDirs(), "/data1/seaweedfs")
	require.Contains(t, volume.UsedDirs(), "/data2/seaweedfs")
}

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

func TestComponentsByStopOrder(t *testing.T) {
	topo := &Specification{}
	names := []string{}
	for _, comp := range topo.ComponentsByStopOrder() {
		names = append(names, comp.Name())
	}
	require.Equal(t, []string{
		ComponentSeaweedFiler,
		ComponentSeaweedVolume,
		ComponentSeaweedMaster,
	}, names)
}

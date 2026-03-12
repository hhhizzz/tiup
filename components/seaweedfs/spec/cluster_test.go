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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadataImplementsManagerContracts(t *testing.T) {
	meta := &Metadata{Topology: new(Specification)}
	meta.SetUser("tidb")
	meta.SetVersion("3.80")
	require.Equal(t, "tidb", meta.GetBaseMeta().User)
	require.Equal(t, "3.80", meta.GetBaseMeta().Version)
	require.IsType(t, &Specification{}, meta.GetTopology())
}

func TestMetadataSetTopology(t *testing.T) {
	meta := &Metadata{Topology: new(Specification)}
	newTopo := &Specification{PackagePath: "/tmp/test.tar.gz"}
	meta.SetTopology(newTopo)
	require.Equal(t, "/tmp/test.tar.gz", meta.Topology.PackagePath)
}

func TestNewPartAndMergeTopo(t *testing.T) {
	topo := &Specification{
		GlobalOptions: GlobalOptions{User: "tidb"},
		PackagePath:   "/tmp/seaweedfs.tar.gz",
		FilerStore: FilerStoreSpec{
			Type:            "tikv",
			FromTiDBCluster: "tidb-prod",
			KeyPrefix:       "swfs-prod",
		},
		MasterServers: []*MasterSpec{{Host: "10.0.1.21", Port: 9333}},
		VolumeServers: []*VolumeSpec{{Host: "10.0.1.22", Port: 8080, Paths: []VolumePathSpec{{Path: "/data1"}}}},
		FilerServers:  []*FilerSpec{{Host: "10.0.1.23", Port: 8888}},
	}

	part := topo.NewPart()
	require.IsType(t, &Specification{}, part)
	partSpec := part.(*Specification)
	require.Equal(t, "tidb", partSpec.GlobalOptions.User)
	// NewPart should not carry server lists
	require.Empty(t, partSpec.MasterServers)

	// MergeTopo
	other := &Specification{
		MasterServers: []*MasterSpec{{Host: "10.0.1.24", Port: 9333}},
	}
	merged := topo.MergeTopo(other)
	mergedSpec := merged.(*Specification)
	require.Len(t, mergedSpec.MasterServers, 2)
	require.Equal(t, "10.0.1.24", mergedSpec.MasterServers[1].Host)
}

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

package scripts

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeaweedMasterScriptIncludesReplicationFlags(t *testing.T) {
	conf, err := os.CreateTemp("", "seaweed-master.*.sh")
	require.NoError(t, err)
	defer os.Remove(conf.Name())

	cfg := &SeaweedMasterScript{
		Port:               9333,
		DefaultReplication: "010",
		VolumeSizeLimitMB:  4096,
		IP:                 "172.19.0.101",
		IPBind:             "0.0.0.0",
		Peers:              "172.19.0.101:9333,172.19.0.102:9333,172.19.0.103:9333",
		DeployDir:          "/deploy",
		DataDir:            "/data",
		LogDir:             "/log",
	}
	require.NoError(t, cfg.ConfigToFile(conf.Name()))
	content, err := os.ReadFile(conf.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), "-defaultReplication=\"010\"")
	require.Contains(t, string(content), "-volumeSizeLimitMB=4096")
	require.Contains(t, string(content), "-port=9333")
	require.Contains(t, string(content), "-ip=\"172.19.0.101\"")
	require.Contains(t, string(content), "-ip.bind=\"0.0.0.0\"")
	require.Contains(t, string(content), "-peers=\"172.19.0.101:9333,172.19.0.102:9333,172.19.0.103:9333\"")
	require.Contains(t, string(content), "bin/weed master")
}

func TestSeaweedMasterScriptUsesSingleMasterMode(t *testing.T) {
	conf, err := os.CreateTemp("", "seaweed-master-single.*.sh")
	require.NoError(t, err)
	defer os.Remove(conf.Name())

	cfg := &SeaweedMasterScript{
		Port:               9333,
		DefaultReplication: "001",
		IP:                 "172.19.0.101",
		IPBind:             "0.0.0.0",
		Peers:              "none",
		DeployDir:          "/deploy",
		DataDir:            "/data",
		LogDir:             "/log",
	}
	require.NoError(t, cfg.ConfigToFile(conf.Name()))
	content, err := os.ReadFile(conf.Name())
	require.NoError(t, err)

	require.Contains(t, string(content), "-ip=\"172.19.0.101\"")
	require.Contains(t, string(content), "-ip.bind=\"0.0.0.0\"")
	require.Contains(t, string(content), "-peers=\"none\"")
	require.NotContains(t, string(content), "-ip=\"0.0.0.0\"")
}

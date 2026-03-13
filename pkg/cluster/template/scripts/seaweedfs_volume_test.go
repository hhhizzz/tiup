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

func TestSeaweedVolumeScriptJoinsPathsAndDisks(t *testing.T) {
	conf, err := os.CreateTemp("", "seaweed-volume.*.sh")
	require.NoError(t, err)
	defer os.Remove(conf.Name())

	cfg := &SeaweedVolumeScript{
		Port:       8080,
		RawDirs:    []string{"/data1/seaweedfs", "/data2/seaweedfs"},
		RawDisks:   []string{"ssd", "hdd"},
		DataCenter: "dc1",
		Rack:       "rack-a",
		DeployDir:  "/deploy",
		LogDir:     "/log",
		MasterAddr: "10.0.1.21:9333",
	}
	require.NoError(t, cfg.ConfigToFile(conf.Name()))
	content, err := os.ReadFile(conf.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), "-dir=\"/data1/seaweedfs,/data2/seaweedfs\"")
	require.Contains(t, string(content), "-disk=\"ssd,hdd\"")
	require.Contains(t, string(content), "-dataCenter=\"dc1\"")
	require.Contains(t, string(content), "-rack=\"rack-a\"")
	require.Contains(t, string(content), "bin/weed volume")
}

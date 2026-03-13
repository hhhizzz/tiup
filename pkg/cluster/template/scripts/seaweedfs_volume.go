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
	"bytes"
	"path"
	"strings"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// SeaweedVolumeScript represents the data to generate a SeaweedFS volume startup script.
type SeaweedVolumeScript struct {
	Port       int
	RawDirs    []string // individual directory paths
	RawDisks   []string // individual disk types
	DataCenter string
	Rack       string
	IPBind     string
	MasterAddr string
	DeployDir  string
	LogDir     string
	NumaNode   string
}

// Dirs returns the comma-joined directory list for the template.
func (c *SeaweedVolumeScript) Dirs() string {
	return strings.Join(c.RawDirs, ",")
}

// DiskTypes returns the comma-joined disk type list for the template.
func (c *SeaweedVolumeScript) DiskTypes() string {
	if len(c.RawDisks) == 0 {
		return ""
	}
	return strings.Join(c.RawDisks, ",")
}

// ConfigToFile writes the rendered startup script to the given path.
func (c *SeaweedVolumeScript) ConfigToFile(file string) error {
	fp := path.Join("templates", "scripts", "run_seaweedfs_volume.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}
	tmpl, err := template.New("seaweedfs-volume").Parse(string(tpl))
	if err != nil {
		return err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return err
	}

	return utils.WriteFile(file, content.Bytes(), 0755)
}

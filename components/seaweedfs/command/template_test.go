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
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateCommandPrintsSeaweedTemplate(t *testing.T) {
	var out bytes.Buffer
	cmd := newTemplateCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())
	require.NotContains(t, out.String(), "package_path:")
	require.Contains(t, out.String(), "master_servers:")
	require.Contains(t, out.String(), "volume_servers:")
	require.Contains(t, out.String(), "filer_store:")
}

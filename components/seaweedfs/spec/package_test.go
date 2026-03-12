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
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildTarball creates a .tar.gz in a temp directory with the given filename->content map.
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
			Mode: 0755,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err = tw.Write([]byte(content))
		require.NoError(t, err)
	}

	return p
}

func TestValidateLocalPackageRequiresWeedEntry(t *testing.T) {
	path := buildTarball(t, map[string]string{
		"README.md": "not a binary",
	})
	err := ValidateLocalPackage(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "weed")
}

func TestValidateLocalPackageAcceptsWeedBinary(t *testing.T) {
	path := buildTarball(t, map[string]string{
		"weed": "#!/bin/sh\necho ok\n",
	})
	require.NoError(t, ValidateLocalPackage(path))
}

func TestValidateLocalPackageRejectsMissingFile(t *testing.T) {
	err := ValidateLocalPackage("/tmp/does-not-exist-seaweedfs.tar.gz")
	require.Error(t, err)
}

func TestValidateLocalPackageRejectsRelativePath(t *testing.T) {
	err := ValidateLocalPackage("relative/path.tar.gz")
	require.Error(t, err)
	require.Contains(t, err.Error(), "absolute")
}

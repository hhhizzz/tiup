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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/task"
)

// ValidateLocalPackage checks that packagePath points to an existing .tar.gz
// that contains a "weed" binary entry. packagePath must be absolute and exist
// on the control machine.
func ValidateLocalPackage(packagePath string) error {
	if !filepath.IsAbs(packagePath) {
		return fmt.Errorf("package_path must be absolute, got %q", packagePath)
	}
	if _, err := os.Stat(packagePath); err != nil {
		return err
	}
	if !strings.HasSuffix(packagePath, ".tar.gz") && !strings.HasSuffix(packagePath, ".tgz") {
		return fmt.Errorf("package_path must point to a .tar.gz file, got %q", packagePath)
	}
	found, err := findTarEntry(packagePath, "weed")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("package %q does not contain a \"weed\" binary entry", packagePath)
	}
	return nil
}

// findTarEntry scans a .tar.gz for an entry whose base name matches target.
func findTarEntry(packagePath, target string) (bool, error) {
	f, err := os.Open(packagePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return false, fmt.Errorf("failed to open gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("failed to read tar: %w", err)
		}
		base := filepath.Base(hdr.Name)
		if base == target {
			return true, nil
		}
	}
	return false, nil
}

// installLocalPackage appends tasks to upload and untar the local package
// on the target host, then chmod +x the weed binary.
func installLocalPackage(b *task.Builder, packagePath, host, deployDir string) *task.Builder {
	return b.InstallPackage(packagePath, host, deployDir).
		Shell(host, fmt.Sprintf("test -f %[1]s/bin/weed && chmod +x %[1]s/bin/weed", deployDir), "chmod-seaweedfs", false)
}

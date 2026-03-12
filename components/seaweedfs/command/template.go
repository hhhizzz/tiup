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
	"fmt"
	"path"

	"github.com/pingcap/tiup/embed"
	"github.com/spf13/cobra"
)

func newTemplateCmd() *cobra.Command {
	var full bool

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Print topology template",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "minimal.yaml"
			if full {
				name = "topology.example.yaml"
			}

			fp := path.Join("examples", "seaweedfs", name)
			tpl, err := embed.ReadExample(fp)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(tpl))
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Print the full topology template for SeaweedFS cluster.")

	return cmd
}

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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap/tiup/pkg/cluster/manager"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	errNS       = errorx.NewNamespace("cmd")
	rootCmd     *cobra.Command
	gOpt        operator.Options
	skipConfirm bool
	log         = logprinter.NewLogger("")
)

var swspec *cspec.SpecManager
var cm *manager.Manager

func init() {
	logger.InitGlobalLogger()

	tui.AddColorFunctionsForCobra()

	cobra.EnableCommandSorting = false

	rootCmd = &cobra.Command{
		Use:           tui.OsArgs0(),
		Short:         "Deploy a SeaweedFS cluster for production",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiUPVersion().String(),
	}

	tui.BeautifyCobraUsageAndHelp(rootCmd)

	rootCmd.AddCommand(
		newTemplateCmd(),
	)
}

func printErrorMessageForNormalError(err error) {
	_, _ = tui.ColorErrorMsg.Fprintf(os.Stderr, "\nError: %s\n", err.Error())
}

func printErrorMessageForErrorX(err *errorx.Error) {
	var msg strings.Builder
	ident := 0
	causeErrX := err
	for causeErrX != nil {
		if ident > 0 {
			msg.WriteString(strings.Repeat("  ", ident) + "caused by: ")
		}
		currentErrMsg := causeErrX.Message()
		if len(currentErrMsg) > 0 {
			if ident == 0 {
				msg.WriteString(fmt.Sprintf("%s (%s)\n", currentErrMsg, causeErrX.Type().FullName()))
			} else {
				msg.WriteString(fmt.Sprintf("%s\n", currentErrMsg))
			}
			ident++
		}
		cause := causeErrX.Cause()
		if c := errorx.Cast(cause); c != nil {
			causeErrX = c
		} else {
			if cause != nil {
				if ident > 0 {
					msg.WriteString(strings.Repeat("  ", ident) + "caused by: ")
				}
				msg.WriteString(fmt.Sprintf("%s\n", cause.Error()))
			}
			break
		}
	}
	_, _ = tui.ColorErrorMsg.Fprintf(os.Stderr, "\nError: %s", msg.String())
}

// Execute executes the root command
func Execute() {
	zap.L().Info("Execute command", zap.String("command", tui.OsArgs()))

	code := 0
	err := rootCmd.Execute()
	if err != nil {
		code = 1
	}

	zap.L().Info("Execute command finished", zap.Int("code", code), zap.Error(err))

	if err != nil {
		switch strings.ToLower(gOpt.DisplayMode) {
		case "json":
			obj := struct {
				Err string `json:"error"`
			}{
				Err: err.Error(),
			}
			data, err := json.Marshal(obj)
			if err != nil {
				fmt.Printf("{\"error\": \"%s\"}", err)
				break
			}
			fmt.Fprintln(os.Stderr, string(data))
		default:
			if errx := errorx.Cast(err); errx != nil {
				printErrorMessageForErrorX(errx)
			} else {
				printErrorMessageForNormalError(err)
			}

			if !errorx.HasTrait(err, utils.ErrTraitPreCheck) {
				logger.OutputDebugLog("tiup-seaweedfs")
			}
		}
	}

	err = logger.OutputAuditLogIfEnabled()
	if err != nil {
		zap.L().Warn("Write audit log file failed", zap.Error(err))
		code = 1
	}

	color.Unset()

	if code != 0 {
		os.Exit(code)
	}
}

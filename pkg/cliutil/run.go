/*
Copyright 2023 The maco Authors

This program is offered under a commercial and under the AGPL license.
For AGPL licensing, see below.

AGPL licensing:
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package cliutil

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	cliflag "github.com/vine-io/maco/pkg/cliutil/flags"
)

// Run provides the common boilerplate code around executing a cobra command.
// For example, it ensures that logging is set up properly. Logging
// flags get added to the command line if not added already. Flags get normalized
// so that help texts show them with hyphens. Underscores are accepted
// as alternative for the command parameters.
//
// Run tries to be smart about how to print errors that are returned by the
// command: before logging is known to be set up, it prints them as plain text
// to stderr. This covers command line flag parse errors and unknown commands.
// Afterwards it logs them. This covers runtime errors.
func Run(cmd *cobra.Command) int {
	if logsInitialized, err := run(cmd); err != nil {
		var errText string
		if v, ok := errors.Unwrap(err).(interface {
			GetDetail() string
		}); ok {
			errText = v.GetDetail()
		} else {
			errText = err.Error()
		}

		if !logsInitialized {
			fmt.Fprintf(os.Stderr, "Error: %v\n", errText)
		} else {
			zap.S().Errorf("command failed: %v", errText)
		}
		return 1
	}
	return 0
}

// RunNoErrOutput is a version of Run which returns the cobra command error
// instead of printing it.
func RunNoErrOutput(cmd *cobra.Command) error {
	_, err := run(cmd)
	return err
}

func run(cmd *cobra.Command) (logsInitialized bool, err error) {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	cmd.SetGlobalNormalizationFunc(cliflag.WordSepNormalizeFunc)

	// When error printing is enabled for the Cobra command, a flag parse
	// error gets printed first, then optionally the often long usage
	// text. This is very unreadable in a console because the last few
	// lines that will be visible on screen don't include the error.
	//
	// The recommendation from #sig-cli was to print the usage text, then
	// the error. We implement this consistently for all commands here.
	// However, we don't want to print the usage text when command
	// execution fails for other reasons than parsing. We detect this via
	// the FlagParseError callback.
	//
	// Some commands, like kubectl, already deal with this themselves.
	// We don't change the behavior for those.
	if !cmd.SilenceUsage {
		cmd.SilenceUsage = true
		cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
			// Re-enable usage printing.
			c.SilenceUsage = false
			return err
		})
	}

	// In all cases error printing is done below.
	cmd.SilenceErrors = true

	// This is idempotent.
	//logs.AddFlags(cmd.PersistentFlags())

	// Inject logs.InitLogs after command line parsing into one of the
	// PersistentPre* functions.
	switch {
	case cmd.PersistentPreRun != nil:
		pre := cmd.PersistentPreRun
		cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			logsInitialized = true
			pre(cmd, args)
		}
	case cmd.PersistentPreRunE != nil:
		pre := cmd.PersistentPreRunE
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			logsInitialized = true
			return pre(cmd, args)
		}
	case cmd.PreRun != nil:
		pre := cmd.PreRun
		cmd.PreRun = func(cmd *cobra.Command, args []string) {
			logsInitialized = true
			pre(cmd, args)
		}
	case cmd.PreRunE != nil:
		pre := cmd.PreRunE
		cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
			logsInitialized = true
			return pre(cmd, args)
		}
	default:
	}

	err = cmd.Execute()
	return
}

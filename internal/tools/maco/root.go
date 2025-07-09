/*
Copyright 2025 The maco Authors

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

package maco

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/client"
	"github.com/vine-io/maco/pkg/logutil"
	version "github.com/vine-io/maco/pkg/version"
)

var defaultUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}} '<target>' <function> [arguments]{{end}} {{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func NewMacoCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "maco",
		Short:   "the client of maco system",
		Version: version.ReleaseVersion(),
		RunE:    runMacoCmd,
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)
	app.SetVersionTemplate(version.GetVersionTemplate())
	app.SetUsageTemplate(defaultUsageTemplate)

	app.ResetFlags()
	//flags := root.PersistentFlags()

	return app
}

func runMacoCmd(cmd *cobra.Command, args []string) error {
	if len(args) <= 1 {
		return cmd.Usage()
	}

	logCfg := logutil.NewLogConfig()
	_ = logCfg.SetupLogging()
	logCfg.SetupGlobalLoggers()

	targets := ""
	if len(args) > 0 {
		targets = strings.Trim(args[0], `'`)
		targets = strings.Trim(targets, `"`)
	}
	function := ""
	if len(args) > 1 {
		function = args[1]
	}
	var argments []string
	if len(args) > 2 {
		argments = args[2:]
	}

	target := "127.0.0.1:4550"

	lg, _ := zap.NewProduction()
	cfg := client.NewConfig(target)
	mc, err := client.NewClient(cfg)
	if err != nil {
		return err
	}

	options, err := types.NewSelectionOptions(types.WithHosts(targets))
	if err != nil {
		fmt.Printf("target failed: %s\n", err.Error())
	}

	ctx := context.Background()
	in := &types.CallRequest{}
	in.Options = options
	in.Function = function
	in.Args = argments

	out, err := mc.Call(ctx, in)
	if err != nil {
		lg.Fatal("call error", zap.Error(err))
	}

	for _, item := range out.Items {
		fmt.Printf("%s:\n", item.Minion)
		if item.Result {
			fmt.Printf("    %s\n", string(item.Data))
		} else {
			fmt.Printf("    Error: %s\n", string(item.Error))
		}
	}

	return nil
}

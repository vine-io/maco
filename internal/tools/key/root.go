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

package key

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	version "github.com/vine-io/maco/pkg/version"
)

var defaultUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}%s{{end}} {{if .HasAvailableSubCommands}}
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

func NewKeyCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "maco-key",
		Short:   "maco-key is used to manage Maco authentication keys",
		Version: version.ReleaseVersion(),
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)
	app.SetVersionTemplate(version.GetVersionTemplate())

	app.SetUsageTemplate(fmt.Sprintf(defaultUsageTemplate, " [global flags] "))

	app.AddCommand(newAcceptKeysCmd(stdin, stdout, stderr))
	app.AddCommand(newDeleteKeysCmd(stdin, stdout, stderr))
	app.AddCommand(newRejectKeysCmd(stdin, stdout, stderr))

	app.AddCommand(newListKeysCmd(stdin, stdout, stderr))
	app.AddCommand(newPrintKeysCmd(stdin, stdout, stderr))

	app.ResetFlags()

	var configPath string
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		configPath = filepath.Join(homeDir, ".maco", "maco.toml")
	}

	globalSet := app.PersistentFlags()
	globalSet.StringP("config", "C", configPath, "Set path to the configuration file.")
	globalSet.StringP("format", "F", "", "Set the format of output, etc text, json, yaml.")
	globalSet.StringP("output", "O", "", "Write the output to the specified file.")
	globalSet.BoolP("output-append", "", false, "Append the output to the specified file.")
	globalSet.BoolP("no-color", "", false, "Disable all colored output.")

	return app
}

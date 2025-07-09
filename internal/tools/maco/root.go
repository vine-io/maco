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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/internal/tools/utils"
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

	var configPath string
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		configPath = filepath.Join(homeDir, ".maco", "maco.toml")
	}

	globalSet := app.PersistentFlags()
	globalSet.StringP("config", "", configPath, "Set path to the configuration file.")

	// Output Flags
	globalSet.StringP("format", "F", "", "Set the format of output, etc text, json, yaml.")
	globalSet.StringP("output", "O", "", "Write the output to the specified file.")
	globalSet.BoolP("output-append", "", false, "Append the output to the specified file.")
	globalSet.BoolP("no-color", "", false, "Disable all colored output.")

	flagSet := app.Flags()

	flagSet.Int64P("timeout", "t", 10, "Change the timeout, if applicable, for the running \ncommand (in seconds).")

	// Target Selection Flags
	flagSet.StringP("hosts", "H", "", "List all known hosts to currently visible or other \nspecified rosters")
	flagSet.StringP("pcre", "E", "", "Instead of using shell globs to evaluate the target \nservers, use pcre regular expressions.")
	flagSet.StringP("list", "L", "", "Instead of using shell globs to evaluate the target \nservers, take a comma or whitespace delimited list of \nservers.")
	flagSet.StringP("grain", "G", "", "Instead of using shell globs to evaluate the target \nuse a grain value to identify targets, the syntax for \nthe target is the grain key followed by a \nglobexpression: \"os:Arch*\".")
	flagSet.StringP("grain-pcre", "P", "", "Instead of using shell globs to evaluate the target \nuse a grain value to identify targets, the syntax for \nthe target is the grain key followed by a pcre regular \nexpression: \"os:Arch.*\".")
	flagSet.StringP("nodegroup", "N", "", "Instead of using shell globs to evaluate the target \nuse one of the predefined nodegroups to identify a \nlist of targets.")
	flagSet.StringP("range", "R", "", "Instead of using shell globs to evaluate the target \nuse a range expression to identify targets. Range \nexpressions look like %cluster.")
	flagSet.StringP("compound", "C", "", "The compound target option allows for multiple target \ntypes to be evaluated, allowing for greater \ngranularity in target matching. The compound target is \nspace delimited, targets other than globs are preceded \nwith an identifier matching the specific targets \nargument type: salt 'G@os:RedHat and webser* or \nE@database.*'.")
	flagSet.StringP("pillar", "I", "", "Instead of using shell globs to evaluate the target \nuse a pillar value to identify targets, the syntax for \nthe target is the pillar key followed by a glob \nexpression: \"role:production*\".")
	flagSet.StringP("pillar-pcre", "J", "", "Instead of using shell globs to evaluate the target \nuse a pillar value to identify targets, the syntax for \nthe target is the pillar key followed by a pcre \nregular expression: \"role:prod.*\".")
	flagSet.StringP("ipcidr", "S", "", "Match based on Subnet (CIDR notation) or IP address.")

	return app
}

func runMacoCmd(cmd *cobra.Command, args []string) error {
	flagSet := cmd.Flags()
	globalSet := cmd.PersistentFlags()

	noColor, _ := globalSet.GetBool("no-color")
	format, _ := globalSet.GetString("format")
	outputFile, _ := globalSet.GetString("output")
	outputAppend, _ := globalSet.GetBool("output-append")

	timeout, _ := globalSet.GetInt64("timeout")

	hosts, _ := flagSet.GetString("hosts")
	pcre, _ := flagSet.GetString("pcre")
	list, _ := flagSet.GetString("list")
	grain, _ := flagSet.GetString("grain")
	grainPcre, _ := flagSet.GetString("grain-pcre")
	nodegroup, _ := flagSet.GetString("nodegroup")
	idRange, _ := flagSet.GetString("range")
	compound, _ := flagSet.GetString("compound")
	pillar, _ := flagSet.GetString("pillar")
	pillarPcre, _ := flagSet.GetString("pillar-pcre")
	ipcidr, _ := flagSet.GetString("ipcidr")

	targetFlaged := false
	var selectionOptions *types.SelectionOptions
	var err error
	if compound != "" {
		selectionOptions, err = types.ParseSelection(compound)
		if err != nil {
			return fmt.Errorf("parse compound target selection options: %v", err)
		}
	} else {
		options := []types.SelectionOption{}
		if hosts != "" {
			options = append(options, types.WithHosts(hosts))
		}
		if pcre != "" {
			options = append(options, types.WithHostRegex(pcre))
		}
		if list != "" {
			options = append(options, types.WithList(strings.Split(list, ",")))
		}
		if grain != "" {
			key, value, ok := strings.Cut(grain, ":")
			if ok {
				options = append(options, types.WithGrains(key, value))
			} else {
				return fmt.Errorf("invalid grain option: %s", grain)
			}
		}
		if grainPcre != "" {
			key, value, ok := strings.Cut(grainPcre, ":")
			if ok {
				options = append(options, types.WithGrains(key, value))
			} else {
				return fmt.Errorf("invalid grain regexp option: %s", grain)
			}
		}
		if nodegroup != "" {
			options = append(options, types.WithHostGroup(strings.Split(nodegroup, ",")))
		}
		if idRange != "" {
			options = append(options, types.WithRange(idRange))
		}
		if pillar != "" {
			key, value, ok := strings.Cut(pillar, ":")
			if ok {
				options = append(options, types.WithPillar(key, value))
			} else {
				return fmt.Errorf("invalid pillar option: %s", grain)
			}
		}
		if pillarPcre != "" {
			key, value, ok := strings.Cut(pillar, ":")
			if ok {
				options = append(options, types.WithPillarRegex(key, value))
			} else {
				return fmt.Errorf("invalid pillar regexp option: %s", grain)
			}
		}
		if ipcidr != "" {
			options = append(options, types.WithIPCidr(ipcidr))
		}
		if len(options) != 0 {
			targetFlaged = true

			selectionOptions, err = types.NewSelectionOptions(options...)
			if err != nil {
				return fmt.Errorf("parse selection options: %v", err)
			}
		}
	}

	targets := ""
	function := ""
	arguments := []string{}
	if targetFlaged {
		if len(args) == 0 {
			return fmt.Errorf("no function specified")
		}
		function = args[0]
		if len(args) > 1 {
			arguments = args[1:]
		}
	} else {
		if len(args) < 1 {
			return fmt.Errorf("no target specified")
		}
		if len(args) < 2 {
			return fmt.Errorf("no function specified")
		}
		targets = strings.Trim(args[0], `'`)
		targets = strings.Trim(targets, `"`)

		selectionOptions, err = types.NewSelectionOptions(types.WithHosts(targets))
		if err != nil {
			return fmt.Errorf("parse target selection options: %v", err)
		}

		function = args[1]
		if len(args) > 2 {
			arguments = args[2:]
		}
	}

	if selectionOptions == nil {
		return fmt.Errorf("no selection options")
	}

	output := cmd.OutOrStdout()
	if len(outputFile) != 0 {
		mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if outputAppend {
			mode = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		}
		fd, fdErr := os.OpenFile(outputFile, mode, 0755)
		if fdErr != nil {
			return fmt.Errorf("open output file: %w", fdErr)
		}
		defer fd.Close()
		output = fd
	}

	mc, err := utils.ClientFromFlags(globalSet)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	in := &types.CallRequest{
		Options:  selectionOptions,
		Function: function,
		Args:     arguments,
		Timeout:  timeout,
	}

	out, err := mc.Call(ctx, in)
	if err != nil {
		return fmt.Errorf("call: %w", err)
	}

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(out, " ", "   ")
		if err != nil {
			return fmt.Errorf("json marshal: %w", err)
		}
		data = append(data, '\n')
	case "yaml":
		data, err = yaml.Marshal(out)
		if err != nil {
			return fmt.Errorf("yaml marshal: %w", err)
		}
	default:
		if noColor || !utils.AllowColor() {
			color.NoColor = true
		}

		buf := bytes.NewBufferString("")
		for _, item := range out.Items {
			color.New(color.FgBlue).Fprintf(buf, "%s:\n", item.Minion)
			if item.Result {
				color.New(color.FgGreen).Fprintf(buf, "    %s\n", string(item.Data))
			} else {
				color.New(color.FgRed).Fprintf(buf, "    Error: %s\n", item.Error)
			}
		}
		data = buf.Bytes()
	}

	fmt.Fprintf(output, "%s", string(data))
	return nil
}

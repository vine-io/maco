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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	apiErr "github.com/vine-io/maco/api/errors"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/internal/tools/utils"
)

type minionMapping struct {
	Accepted   []string `json:"accepted" yaml:"accepted"`
	AutoSigned []string `json:"auto_signed" yaml:"auto_signed"`
	Denied     []string `json:"denied" yaml:"denied,omitempty"`
	Unaccepted []string `json:"unaccepted" yaml:"unaccepted"`
	Rejected   []string `json:"rejected" yaml:"rejected"`
}

type minionKeyMapping struct {
	Accepted   []*types.MinionKey `json:"accepted" yaml:"accepted"`
	AutoSigned []*types.MinionKey `json:"auto_signed" yaml:"auto_signed"`
	Denied     []*types.MinionKey `json:"denied" yaml:"denied,omitempty"`
	Unaccepted []*types.MinionKey `json:"unaccepted" yaml:"unaccepted"`
	Rejected   []*types.MinionKey `json:"rejected" yaml:"rejected"`
}

func newListKeysCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "list",
		Aliases: []string{"L", "ls"},
		Short:   "list minions key",
		RunE:    runListKeysCmd,
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)

	app.ResetFlags()

	return app
}

func runListKeysCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	globalSet := cmd.Parent().PersistentFlags()
	noColor, _ := globalSet.GetBool("no-color")
	format, _ := globalSet.GetString("format")
	outputFile, _ := globalSet.GetString("output")
	outputAppend, _ := globalSet.GetBool("output-append")

	mc, err := utils.ClientFromFlags(globalSet)
	if err != nil {
		return err
	}

	states := []types.MinionState{
		types.Unaccepted,
		types.Accepted,
		types.AutoSign,
		types.Denied,
		types.Rejected,
	}
	out, err := mc.ListMinions(ctx, states...)
	if err != nil {
		return fmt.Errorf("%v", apiErr.Parse(err).Detail)
	}

	mapping := minionMapping{
		Accepted:   out[types.Accepted],
		AutoSigned: out[types.AutoSign],
		Denied:     out[types.Denied],
		Unaccepted: out[types.Unaccepted],
		Rejected:   out[types.Rejected],
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

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(mapping, " ", "   ")
		if err != nil {
			return fmt.Errorf("json marshal: %w", err)
		}
		data = append(data, '\n')
	case "yaml":
		data, err = yaml.Marshal(mapping)
		if err != nil {
			return fmt.Errorf("yaml marshal: %w", err)
		}
	default:
		if noColor || !utils.AllowColor() {
			color.NoColor = true
		}

		buf := bytes.NewBufferString("")
		color.New(color.FgGreen).Fprintf(buf, "Accepted Keys:\n")
		for _, minion := range mapping.Accepted {
			color.New(color.FgGreen).Fprintf(buf, "%s\n", minion)
		}
		for _, minion := range mapping.AutoSigned {
			color.New(color.FgGreen).Fprintf(buf, "%s\n", minion)
		}
		color.New(color.FgMagenta).Fprintf(buf, "Denied Keys:\n")
		for _, minion := range mapping.Denied {
			color.New(color.FgMagenta).Fprintf(buf, "%s\n", minion)
		}
		color.New(color.FgYellow).Fprintf(buf, "Unaccepted Keys:\n")
		for _, minion := range mapping.Unaccepted {
			color.New(color.FgYellow).Fprintf(buf, "%s\n", minion)
		}
		color.New(color.FgRed).Fprintf(buf, "Rejected Keys:\n")
		for _, minion := range mapping.Rejected {
			color.New(color.FgRed).Fprintf(buf, "%s\n", minion)
		}

		data = buf.Bytes()
	}

	fmt.Fprintf(output, "%s", string(data))
	return nil
}

func newPrintKeysCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "print",
		Aliases: []string{"P"},
		Short:   "list minions key",
		RunE:    runPrintKeysCmd,
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)

	app.SetUsageTemplate(fmt.Sprintf(defaultUsageTemplate, " [minions] "))

	app.ResetFlags()
	flagSet := app.Flags()
	flagSet.BoolP("all", "", false, "print all keys")

	return app
}

func runPrintKeysCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	flagSet := cmd.Flags()
	globalSet := cmd.Parent().PersistentFlags()
	noColor, _ := globalSet.GetBool("no-color")
	format, _ := globalSet.GetString("format")
	outputFile, _ := globalSet.GetString("output")
	outputAppend, _ := globalSet.GetBool("output-append")

	all, _ := flagSet.GetBool("all")

	mc, err := utils.ClientFromFlags(globalSet)
	if err != nil {
		return err
	}

	minions := []string{}
	if len(args) > 0 {
		minions = strings.Split(args[0], ",")
	}
	if (len(minions) == 0 || minions[0] == "") && !all {
		return errors.New("no minions specified")
	}

	out, err := mc.PrintMinion(ctx, minions, all)
	if err != nil {
		return fmt.Errorf("%v", apiErr.Parse(err).Detail)
	}

	mapping := &minionKeyMapping{
		Accepted:   []*types.MinionKey{},
		AutoSigned: []*types.MinionKey{},
		Denied:     []*types.MinionKey{},
		Unaccepted: []*types.MinionKey{},
		Rejected:   []*types.MinionKey{},
	}

	for _, key := range out {
		switch types.MinionState(key.State) {
		case types.Accepted:
			mapping.Accepted = append(mapping.Accepted, key)
		case types.AutoSign:
			mapping.AutoSigned = append(mapping.AutoSigned, key)
		case types.Denied:
			mapping.Denied = append(mapping.Denied, key)
		case types.Unaccepted:
			mapping.Unaccepted = append(mapping.Unaccepted, key)
		case types.Rejected:
			mapping.Rejected = append(mapping.Rejected, key)
		}
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

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(mapping, " ", "   ")
		if err != nil {
			return fmt.Errorf("json marshal: %w", err)
		}
		data = append(data, '\n')
	case "yaml":
		data, err = yaml.Marshal(mapping)
		if err != nil {
			return fmt.Errorf("yaml marshal: %w", err)
		}
	default:
		if noColor || !utils.AllowColor() {
			color.NoColor = true
		}

		buf := bytes.NewBufferString("")
		color.New(color.FgGreen).Fprintf(buf, "Accepted Keys:\n")
		for _, minion := range mapping.Accepted {
			color.New(color.FgGreen).Fprintf(buf, "  %s: %s", minion.Minion.Name, minion.PubKey)
		}
		for _, minion := range mapping.AutoSigned {
			color.New(color.FgGreen).Fprintf(buf, "  %s: %s", minion.Minion.Name, minion.PubKey)
		}
		color.New(color.FgMagenta).Fprintf(buf, "Denied Keys:\n")
		for _, minion := range mapping.Denied {
			color.New(color.FgMagenta).Fprintf(buf, "  %s: %s", minion.Minion.Name, minion.PubKey)
		}
		color.New(color.FgYellow).Fprintf(buf, "Unaccepted Keys:\n")
		for _, minion := range mapping.Unaccepted {
			color.New(color.FgYellow).Fprintf(buf, "  %s: %s", minion.Minion.Name, minion.PubKey)
		}
		color.New(color.FgRed).Fprintf(buf, "Rejected Keys:\n")
		for _, minion := range mapping.Rejected {
			color.New(color.FgRed).Fprintf(buf, "  %s: %s", minion.Minion.Name, minion.PubKey)
		}

		data = buf.Bytes()
	}

	fmt.Fprintf(output, "%s", string(data))
	return nil
}

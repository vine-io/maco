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
	"gopkg.in/yaml.v3"

	apiErr "github.com/vine-io/maco/api/errors"
	"github.com/vine-io/maco/internal/tools/utils"
)

func newAcceptKeysCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "accept",
		Aliases: []string{"A", "acc"},
		Short:   "accept specified minions key",
		RunE:    runAcceptKeysCmd,
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)

	app.SetUsageTemplate(fmt.Sprintf(defaultUsageTemplate, " [minions] "))
	app.UsageFunc()

	app.ResetFlags()

	flagSet := app.Flags()
	flagSet.BoolP("all", "", false, "accept all keys")
	flagSet.BoolP("include-denied", "", false, "accept denied keys")
	flagSet.BoolP("include-rejected", "", false, "accept rejected keys")

	return app
}

func runAcceptKeysCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	flagSet := cmd.Flags()
	globalSet := cmd.Parent().PersistentFlags()

	noColor, _ := globalSet.GetBool("no-color")
	format, _ := globalSet.GetString("format")
	outputFile, _ := globalSet.GetString("output")
	outputAppend, _ := globalSet.GetBool("output-append")

	all, _ := flagSet.GetBool("all")
	includeDenied, _ := flagSet.GetBool("include-denied")
	includeRejected, _ := flagSet.GetBool("include-rejected")

	mc, err := utils.ClientFromFlags(globalSet)
	if err != nil {
		return err
	}

	minions := []string{}
	if len(args) > 0 {
		minions = strings.Split(args[0], ",")
	}
	if (len(minions) == 0 || minions[0] == "") && all {
		return errors.New("no minions specified")
	}

	out, err := mc.AcceptMinion(ctx, minions, all, includeRejected, includeDenied)
	if err != nil {
		return fmt.Errorf("%v", apiErr.Parse(err).Detail)
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

	mapping := map[string][]string{
		"accepted": out,
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
		for _, minion := range out {
			color.New(color.FgGreen).Fprintf(buf, "%s\n", minion)
		}

		data = buf.Bytes()
	}

	fmt.Fprintf(output, "%s", string(data))
	return nil
}

func newRejectKeysCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "reject",
		Aliases: []string{"R", "rej"},
		Short:   "reject specified minions key",
		RunE:    runRejectKeysCmd,
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)

	app.SetUsageTemplate(fmt.Sprintf(defaultUsageTemplate, " [minions] "))
	app.UsageFunc()

	app.ResetFlags()

	flagSet := app.Flags()
	flagSet.BoolP("all", "", false, "accept all keys")
	flagSet.BoolP("include-accepted", "", false, "reject accepted keys")
	flagSet.BoolP("include-denied", "", false, "accept denied keys")

	return app
}

func runRejectKeysCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	flagSet := cmd.Flags()
	globalSet := cmd.Parent().PersistentFlags()

	noColor, _ := globalSet.GetBool("no-color")
	format, _ := globalSet.GetString("format")
	outputFile, _ := globalSet.GetString("output")
	outputAppend, _ := globalSet.GetBool("output-append")

	all, _ := flagSet.GetBool("all")
	includeAccepted, _ := flagSet.GetBool("include-accepted")
	includeDenied, _ := flagSet.GetBool("include-denied")

	mc, err := utils.ClientFromFlags(globalSet)
	if err != nil {
		return err
	}

	minions := []string{}
	if len(args) > 0 {
		minions = strings.Split(args[0], ",")
	}
	if (len(minions) == 0 || minions[0] == "") && all {
		return errors.New("no minions specified")
	}

	out, err := mc.RejectMinion(ctx, minions, all, includeAccepted, includeDenied)
	if err != nil {
		return fmt.Errorf("%v", apiErr.Parse(err).Detail)
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

	mapping := map[string][]string{
		"rejected": out,
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
		color.New(color.FgRed).Fprintf(buf, "Rejected Keys:\n")
		for _, minion := range out {
			color.New(color.FgRed).Fprintf(buf, "%s\n", minion)
		}

		data = buf.Bytes()
	}

	fmt.Fprintf(output, "%s", string(data))
	return nil
}

func newDeleteKeysCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"D", "del"},
		Short:   "delete specified minions key",
		RunE:    runDeleteKeysCmd,
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)

	app.SetUsageTemplate(fmt.Sprintf(defaultUsageTemplate, " [minions] "))
	app.UsageFunc()

	app.ResetFlags()

	flagSet := app.Flags()
	flagSet.BoolP("all", "", false, "delete all keys")

	return app
}

func runDeleteKeysCmd(cmd *cobra.Command, args []string) error {
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
	if (len(minions) == 0 || minions[0] == "") && all {
		return errors.New("no minions specified")
	}

	out, err := mc.DeleteMinion(ctx, minions, all)
	if err != nil {
		return fmt.Errorf("%v", apiErr.Parse(err).Detail)
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

	mapping := map[string][]string{
		"deleted": out,
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
		color.New(color.FgRed).Fprintf(buf, "Deleted Keys:\n")
		for _, minion := range out {
			color.New(color.FgRed).Fprintf(buf, "%s\n", minion)
		}

		data = buf.Bytes()
	}

	fmt.Fprintf(output, "%s", string(data))
	return nil
}

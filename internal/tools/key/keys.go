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
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	apiErr "github.com/vine-io/maco/api/errors"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/internal/tools/utils"
)

func newListKeysCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "list",
		Aliases: []string{"L", "ls"},
		Short:   "list minions key",
		RunE:    runCmd,
	}

	app.SetIn(stdin)
	app.SetOut(stdout)
	app.SetErr(stderr)

	app.ResetFlags()

	return app
}

type MinionKey struct {
	Accepted   []string `json:"accepted" yaml:"accepted"`
	AutoSigned []string `json:"auto_signed" yaml:"auto_signed"`
	Denied     []string `json:"denied" yaml:"denied,omitempty"`
	Unaccepted []string `json:"unaccepted" yaml:"unaccepted"`
	Rejected   []string `json:"rejected" yaml:"rejected"`
}

func runCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	globalSet := cmd.Parent().PersistentFlags()
	noColor, _ := globalSet.GetBool("no-color")
	format, _ := globalSet.GetString("format")
	outputFile, _ := globalSet.GetString("output")

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

	mk := MinionKey{
		Accepted:   out[types.Accepted],
		AutoSigned: out[types.AutoSign],
		Denied:     out[types.Denied],
		Unaccepted: out[types.Unaccepted],
		Rejected:   out[types.Rejected],
	}

	output := cmd.OutOrStdout()
	if len(outputFile) != 0 {
		fd, fdErr := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if fdErr != nil {
			return fmt.Errorf("open output file: %w", fdErr)
		}
		defer fd.Close()
		output = fd
	}

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(mk, " ", "   ")
		if err != nil {
			return fmt.Errorf("json marshal: %w", err)
		}
		data = append(data, '\n')
	case "yaml":
		data, err = yaml.Marshal(mk)
		if err != nil {
			return fmt.Errorf("yaml marshal: %w", err)
		}
	default:
		if noColor {
			color.NoColor = true
		}

		buf := bytes.NewBufferString("")
		color.New(color.FgGreen).Fprintf(buf, "Accepted Keys:\n")
		for _, minion := range mk.Accepted {
			color.New(color.FgGreen).Fprintf(buf, "%s\n", minion)
		}
		for _, minion := range mk.Accepted {
			color.New(color.FgGreen).Fprintf(buf, "%s\n", minion)
		}
		color.New(color.FgMagenta).Fprintf(buf, "Denied Keys:\n")
		for _, minion := range mk.Denied {
			color.New(color.FgMagenta).Fprintf(buf, "%s\n", minion)
		}
		color.New(color.FgRed).Fprintf(buf, "Unaccepted Keys:\n")
		for _, minion := range mk.Unaccepted {
			color.New(color.FgRed).Fprintf(buf, "%s\n", minion)
		}
		color.New(color.FgYellow).Fprintf(buf, "Rejected Keys:\n")
		for _, minion := range mk.Rejected {
			color.New(color.FgYellow).Fprintf(buf, "%s\n", minion)
		}

		data = buf.Bytes()
	}

	fmt.Fprintf(output, "%s", string(data))
	return nil
}

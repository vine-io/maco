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
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	version "github.com/vine-io/maco/pkg/version"
)

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

	app.AddCommand(newListKeysCmd(stdin, stdout, stderr))

	app.ResetFlags()

	var configPath string
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		configPath = filepath.Join(homeDir, ".maco", "maco.toml")
	}

	pflags := app.PersistentFlags()
	pflags.StringP("config", "C", configPath, "Set path to the configuration file.")
	pflags.StringP("format", "f", "", "Set the format of output, etc text, json, yaml.")
	pflags.StringP("output", "", "", "Write the output to the specified file.")
	pflags.BoolP("no-color", "", false, "Disable all colored output.")

	return app
}

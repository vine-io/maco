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

package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vine-io/maco/internal/master"
	genericserver "github.com/vine-io/maco/pkg/server"
	version "github.com/vine-io/maco/pkg/version"
)

func NewMasterCommand(stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "maco-master",
		Short:   "the master component of maco system",
		Version: version.ReleaseVersion(),
		PreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, _ := cmd.Flags().GetString("config")
			return runMaster(ctx, cfg)
		},
	}

	app.SetOut(stdout)
	app.SetErr(stderr)
	app.SetVersionTemplate(version.GetVersionTemplate())

	app.ResetFlags()
	flags := app.PersistentFlags()

	var configPath string
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		configPath = filepath.Join(homeDir, ".maco", "master.toml")
	}

	flags.StringP("config", "C", configPath, "path to the configuration file")

	return app
}

func runMaster(ctx context.Context, configPath string) error {
	cfg, err := master.FromPath(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err = cfg.Init(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}

	app, err := master.NewMaster(cfg)
	if err != nil {
		return fmt.Errorf("create maco-master server: %w", err)
	}

	ctx = genericserver.SetupSignalContext(ctx)
	return app.Start(ctx)
}

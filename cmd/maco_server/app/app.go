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
	"runtime"

	"github.com/spf13/cobra"

	"github.com/vine-io/maco/internal/server"
	"github.com/vine-io/maco/internal/server/config"
	genericserver "github.com/vine-io/maco/pkg/server"
	version "github.com/vine-io/maco/pkg/version"
)

func NewServerCommand(stdout, stderr io.Writer) *cobra.Command {
	app := &cobra.Command{
		Use:     "maco-server",
		Short:   "the server component of maco system",
		Version: version.ReleaseVersion(),
		PreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, _ := cmd.Flags().GetString("config")
			return runServer(ctx, cfg)
		},
	}

	app.SetOut(stdout)
	app.SetErr(stderr)
	app.SetVersionTemplate(getVersionTemplate())

	app.ResetFlags()
	flags := app.PersistentFlags()

	var configPath string
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		configPath = filepath.Join(homeDir, ".maco", "config.toml")
	}

	flags.StringP("config", "c", configPath, "path to the configuration file")

	return app
}

func getVersionTemplate() string {
	var tpl string
	tpl += fmt.Sprintf("maco Version: %s\n", version.GitTag)
	tpl += fmt.Sprintf("Git SHA: %s\n", version.GitCommit)
	tpl += fmt.Sprintf("Go Version: %s\n", runtime.Version())
	tpl += fmt.Sprintf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return tpl
}

func runServer(ctx context.Context, configPath string) error {
	cfg, err := config.FromPath(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err = cfg.Init(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}

	maco, err := server.NewMacoServer(cfg)
	if err != nil {
		return fmt.Errorf("create maco server: %w", err)
	}

	ctx = genericserver.SetupSignalContext(ctx)
	return maco.Start(ctx)
}

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

package utils

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/vine-io/maco/client"
)

func ClientFromFlags(flagSet *pflag.FlagSet) (*client.Client, error) {
	cfgPath, err := flagSet.GetString("config")
	if err != nil {
		return nil, fmt.Errorf("read from flags: %w", err)
	}

	cfg, err := client.FromPath(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load local config: %w", err)
	}

	if err = cfg.Init(); err != nil {
		return nil, fmt.Errorf("check config: %w", err)
	}

	masterClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create master client: %w", err)
	}

	return masterClient, nil
}

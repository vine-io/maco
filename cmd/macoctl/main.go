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

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/client"
	"github.com/vine-io/maco/pkg/logutil"
)

func main() {
	cfg := logutil.NewLogConfig()
	_ = cfg.SetupLogging()
	cfg.SetupGlobalLoggers()

	host := ""
	shell := ""
	flag.StringVar(&host, "host", "*", "target minion")
	flag.StringVar(&shell, "shell", "hostname", "linux bash command")
	flag.Parse()

	target := "192.168.141.128:4550"

	lg, _ := zap.NewProduction()
	opts := client.NewOptions(lg, target, "_output")
	mc, err := client.NewClient(opts)
	if err != nil {
		lg.Fatal(err.Error())
		return
	}

	ctx := context.Background()
	in := &types.CallRequest{
		Selector: &types.Selector{},
	}
	in.Selector.Minions = strings.Split(host, ",")
	shellParts := strings.Split(shell, " ")
	in.Function = shellParts[0]
	if len(shellParts) > 1 {
		in.Args = shellParts[1:]
	}

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
}

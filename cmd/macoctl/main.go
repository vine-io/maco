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
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/vine-io/maco/api/rpc"
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

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithIdleTimeout(time.Second * 15),
	}
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		zap.L().Fatal("grpc client error", zap.Error(err))
	}

	client := pb.NewMacoRPCClient(conn)

	ctx := context.Background()
	in := &pb.CallRequest{}
	in.Hosts = strings.Split(host, ",")
	shellParts := strings.Split(shell, " ")
	in.Function = shellParts[0]
	if len(shellParts) > 1 {
		in.Args = shellParts[1:]
	}

	out, err := client.Call(ctx, in)
	if err != nil {
		zap.L().Fatal("call error", zap.Error(err))
	}

	for _, item := range out.Report.Items {
		fmt.Printf("%s:\n", item.Minion)
		if item.Result {
			fmt.Printf("    %s\n", string(item.Data))
		} else {
			fmt.Printf("    Error: %s\n", string(item.Error))
		}
	}
}

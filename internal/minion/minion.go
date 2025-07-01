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

package minion

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/pemutil"
	genericserver "github.com/vine-io/maco/pkg/server"
	version "github.com/vine-io/maco/pkg/version"
)

type Minion struct {
	genericserver.IEmbedServer

	cfg *Config
}

func NewMinion(cfg *Config) (*Minion, error) {
	lg := cfg.Logger()
	es := genericserver.NewEmbedServer(lg)

	ms := &Minion{
		IEmbedServer: es,

		cfg: cfg,
	}
	return ms, nil
}

func (m *Minion) Start(ctx context.Context) error {

	zap.L().Info("starting maco server")
	if err := m.start(ctx); err != nil {
		return err
	}

	m.Destroy(m.destroy)

	<-ctx.Done()

	return m.stop(ctx)
}

func (m *Minion) start(ctx context.Context) error {
	cfg := m.cfg
	lg := cfg.Logger()

	target := cfg.Master

	kecp := keepalive.ClientParameters{
		Time:                time.Second * 10,
		Timeout:             time.Second * 30,
		PermitWithoutStream: true,
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kecp),
		grpc.WithIdleTimeout(time.Second * 5),
	}
	client, err := grpc.NewClient(target, opts...)
	if err != nil {
		return err
	}
	conn := pb.NewInternalRPCClient(client)

	exists := true
	root := cfg.DataRoot
	pem := filepath.Join(root, "minion.pem")
	pemBytes, err := os.ReadFile(pem)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		exists = false
	}

	pub := filepath.Join(root, "minion.pub")
	pubBytes, err := os.ReadFile(pub)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		exists = false
	}

	pair := &pemutil.RsaPair{
		Private: pemBytes,
		Public:  pubBytes,
	}

	if !exists {
		pair, err = pemutil.GenerateRSA(2048, "MACO")
		if err != nil {
			return err
		}

		lg.Info("generate master pki",
			zap.String("private", pem),
			zap.String("public", pub))

		err = os.WriteFile(pem, pair.Private, 0600)
		if err != nil {
			return fmt.Errorf("save server private key: %v", err)
		}
		err = os.WriteFile(pub, pair.Public, 0600)
		if err != nil {
			return fmt.Errorf("save server public key: %v", err)
		}
	}

	hostname, _ := os.Hostname()
	node := &types.Minion{
		Name:     m.cfg.Name,
		Uid:      "",
		Ip:       "",
		Hostname: hostname,
		Tags:     map[string]string{},
		Os:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  version.GitTag,
	}

	go func(ctx context.Context) {
		callOpts := []grpc.CallOption{}

		internal := time.Second * 5
		timer := time.NewTimer(internal)
		defer timer.Stop()

		for {
			stream, err := conn.Dispatch(ctx, callOpts...)
			if err != nil {
				return
			}

			err = stream.Send(&pb.DispatchRequest{
				Type: types.EventType_EventConnect,
				Connect: &types.ConnectRequest{
					Minion:          node,
					MinionPublicKey: pubBytes,
				},
				Call: nil,
			})
			if err != nil {
				lg.Info("dispatch stream error", zap.Error(err))
			}

			lg.Info("connect to master, waiting for stream receiver")

		LOOP:
			for {
				rsp, err := stream.Recv()
				if err != nil {
					break LOOP
				}

				switch rsp.Type {
				case types.EventType_EventConnect:
				case types.EventType_EventCall:
					in := rsp.Call
					if in == nil {
						continue
					}

					shell := fmt.Sprintf("%s", in.Function)
					for _, arg := range in.Args {
						shell += " " + arg
					}

					buf := bytes.NewBufferString("")
					cmd := exec.CommandContext(ctx, "/bin/bash", "-c", shell)
					cmd.Stdout = buf
					cmd.Stderr = buf
					var e1 error
					if e1 = cmd.Start(); e1 == nil {
						e1 = cmd.Wait()
					}

					callRsp := &types.CallResponse{
						Id:   in.Id,
						Type: types.ResultType_ResultOk,
					}

					callRsp.RetCode = int32(cmd.ProcessState.ExitCode())
					if e1 != nil {
						callRsp.Type = types.ResultType_ResultError
						callRsp.Error = buf.String()
					} else {
						callRsp.Result = bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
					}

					_ = stream.Send(&pb.DispatchRequest{
						Type: types.EventType_EventCall,
						Call: callRsp,
					})
				}
			}

			timer.Reset(internal)

			select {
			case <-ctx.Done():
			case <-timer.C:
			}

			lg.Info("reconnecting to master")
		}
	}(ctx)

	return nil
}

func (m *Minion) destroy() {}

func (m *Minion) stop(ctx context.Context) error {
	return nil
}

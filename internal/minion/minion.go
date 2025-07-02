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

	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/client"
	"github.com/vine-io/maco/pkg/pemutil"
	genericserver "github.com/vine-io/maco/pkg/server"
	version "github.com/vine-io/maco/pkg/version"
)

type Minion struct {
	genericserver.IEmbedServer

	ctx    context.Context
	cancel context.CancelFunc

	cfg *Config

	rsaPair *pemutil.RsaPair

	masterClient *client.Client
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

	m.ctx, m.cancel = context.WithCancel(ctx)
	if err := m.start(ctx); err != nil {
		return err
	}

	m.Destroy(m.destroy)

	<-ctx.Done()

	return m.stop()
}

func (m *Minion) start(ctx context.Context) error {
	cfg := m.cfg
	lg := cfg.Logger()

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
	m.rsaPair = pair

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

	target := cfg.Master

	copts := client.NewOptions(lg, target, m.cfg.DataRoot)
	masterClient, err := client.NewClient(copts)
	if err != nil {
		return fmt.Errorf("connect to maco-master failed: %v", err)
	}

	in := &types.ConnectRequest{
		Minion:          node,
		MinionPublicKey: pair.Public,
	}
	dispatcher, err := masterClient.NewDispatcher(ctx, in)
	if err != nil {
		return fmt.Errorf("connect to dispatcher: %v", err)
	}

	go m.dispatch(dispatcher)

	return nil
}

func (m *Minion) dispatch(dispatcher *client.Dispatcher) {
	for {
		event, err := dispatcher.Recv()
		if err != nil {
			return
		}

		if event.Err != nil {
			continue
		}

		switch event.EventType {
		case types.EventType_EventCall:
			msg := event.Call
			if msg == nil {
				continue
			}
			in := &types.CallRequest{}
			b, e1 := pemutil.DecodeByRSA(msg.Data, m.rsaPair.Private)
			if e1 != nil {
				event.Err = e1
			} else {
				e1 = msgpack.Unmarshal(b, in)
				event.Err = e1
			}

			if e1 != nil {
				reply := &types.CallResponse{
					Id:    msg.Id,
					Type:  types.ResultType_ResultError,
					Error: e1.Error(),
				}
				_ = dispatcher.Call(reply)
				continue
			}

			if in.Timeout == 0 {
				in.Timeout = 10
			}
			rsp, e1 := runCmd(m.ctx, in)
			if e1 != nil {

			}
			_ = dispatcher.Call(rsp)
		}
	}
}

func runCmd(ctx context.Context, in *types.CallRequest) (*types.CallResponse, error) {
	timeout := time.Duration(in.Timeout) * time.Second
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell := fmt.Sprintf("%s", in.Function)
	for _, arg := range in.Args {
		shell += " " + arg
	}

	buf := bytes.NewBufferString("")
	cmd := exec.CommandContext(callCtx, "/bin/bash", "-c", shell)
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

	return callRsp, e1
}

func (m *Minion) destroy() {}

func (m *Minion) stop() error {
	m.cancel()

	if err := m.masterClient.Close(); err != nil {
		return fmt.Errorf("close master client: %v", err)
	}
	return nil
}

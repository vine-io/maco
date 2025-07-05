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
	"os/exec"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/client"
	"github.com/vine-io/maco/pkg/pemutil"
)

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

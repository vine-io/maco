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

package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/pemutil"
)

type Event struct {
	EventType types.EventType
	Call      *pb.DispatchCallMsg
	Err       error
}

type Dispatcher struct {
	ctx context.Context

	lg             *zap.Logger
	internalClient pb.InternalRPCClient

	stream pb.InternalRPC_DispatchClient

	connMsg *types.ConnectRequest

	masterPubKey []byte

	callOptions []grpc.CallOption

	connected *atomic.Bool

	ech chan *Event

	done chan struct{}
}

func (c *Client) NewDispatcher(ctx context.Context, req *types.ConnectRequest) (*Dispatcher, *types.Minion, error) {
	opts := c.buildCallOptions()

	connected := &atomic.Bool{}
	connected.Store(false)
	dispatcher := &Dispatcher{
		ctx:            ctx,
		lg:             c.opt.Logger,
		internalClient: c.internalClient,
		connMsg:        req,
		callOptions:    opts,
		connected:      connected,
		ech:            make(chan *Event, 10),
		done:           c.done,
	}
	rsp, err := dispatcher.connect(ctx)
	if err != nil {
		return nil, nil, err
	}
	pubKey := filepath.Join(c.opt.DataRoot, "master")
	_ = os.WriteFile(pubKey, rsp.MasterPublicKey, 0600)

	go dispatcher.process()
	return dispatcher, rsp.Minion, nil
}

func (d *Dispatcher) Context() context.Context {
	return d.ctx
}

func (d *Dispatcher) connect(ctx context.Context) (*types.ConnectResponse, error) {
	d.lg.Info("connecting to master dispatch")
	stream, err := d.internalClient.Dispatch(ctx, d.callOptions...)
	if err != nil {
		return nil, err
	}
	d.stream = stream

	msg := &pb.DispatchRequest{
		Type:    types.EventType_EventConnect,
		Connect: d.connMsg,
	}
	err = stream.Send(msg)
	if err != nil {
		return nil, parse(err)
	}

	out, err := stream.Recv()
	if err != nil {
		return nil, parse(err)
	}
	rsp := out.Connect
	d.masterPubKey = rsp.MasterPublicKey

	d.lg.Info("connect to master dispatch succeeded")
	d.connected.Store(true)
	return rsp, nil
}

// Call 返回 master-master Call 请求的执行结果
func (d *Dispatcher) Call(in *types.CallResponse) error {

	rsp := &pb.DispatchCallMsg{
		Id: in.Id,
	}

	b, err := msgpack.Marshal(in)
	if err != nil {
		err = fmt.Errorf("msgpack marshal: %w", err)
	} else {
		b, err = pemutil.EncodeByRSA(b, d.masterPubKey)
		if err != nil {
			err = fmt.Errorf("rsa encode: %w", err)
		}
	}

	rsp.Data = b
	if err != nil {
		return err
	}

	msg := &pb.DispatchRequest{
		Type: types.EventType_EventCall,
		Call: rsp,
	}

	err = d.stream.Send(msg)
	return parse(err)
}

func (d *Dispatcher) Recv() (*Event, error) {
	select {
	case <-d.done:
		return nil, io.EOF
	case <-d.ctx.Done():
		return nil, d.ctx.Err()
	case e := <-d.ech:
		return e, nil
	}
}

func (d *Dispatcher) process() {
	ctx := d.Context()
	attempts := 0
	var interval time.Duration

	timer := time.NewTimer(interval)
	defer timer.Stop()

START:
	for {
		interval = retryInterval(attempts)
		timer.Reset(interval)

		attempts += 1

		select {
		case <-d.done:
			d.lg.Info("dispatch disconnected")
			return
		case <-timer.C:
		}

		if !d.connected.Load() {
			d.lg.Info("reconnecting to master dispatch")
			_, err := d.connect(ctx)
			if err != nil {
				if isUnavailable(err) {
					d.lg.Error("master dispatch is unavailable")
				} else {
					d.lg.Error("connects to master dispatch error", zap.Error(err))
					event := &Event{Err: err}
					d.ech <- event
				}
				d.connected.Store(false)
				break START
			} else {
				// 重连成功，重置连接间隔
				attempts = 0
			}
		}

		d.lg.Info("receives dispatch message")
	LOOP:
		for {
			select {
			case <-ctx.Done():
				return
			case <-d.done:
				return
			default:
			}

			rsp, err := d.stream.Recv()
			if err != nil {
				if isUnavailable(err) {
					d.lg.Error("master dispatch is unavailable")
				} else {
					d.lg.Error("connect to master dispatch error", zap.Error(err))
					event := &Event{Err: err}
					d.ech <- event
				}
				d.connected.Store(false)
				break LOOP
			}

			event := &Event{EventType: rsp.Type, Call: rsp.Call}
			d.ech <- event
		}
	}
}

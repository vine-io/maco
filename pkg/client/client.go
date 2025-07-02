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
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	apiErr "github.com/vine-io/maco/api/errors"
	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/pemutil"
)

const (
	DefaultTimeout = time.Second * 5
)

type Options struct {
	Logger *zap.Logger

	Target         string
	DataRoot       string
	DialTimeout    time.Duration
	RequestTimeout time.Duration

	TLS *tls.Config

	masterPubKey []byte
}

func NewOptions(lg *zap.Logger, target, dataRoot string) *Options {
	opts := &Options{
		Logger:         lg,
		Target:         target,
		DataRoot:       dataRoot,
		DialTimeout:    DefaultTimeout,
		RequestTimeout: DefaultTimeout,
	}
	return opts
}

type Client struct {
	opt *Options

	conn *grpc.ClientConn

	macoClient     pb.MacoRPCClient
	internalClient pb.InternalRPCClient

	done chan struct{}
}

func NewClient(opt *Options) (*Client, error) {
	target := opt.Target

	var creds credentials.TransportCredentials
	if opt.TLS != nil {
		creds = credentials.NewTLS(opt.TLS)
	} else {
		creds = insecure.NewCredentials()
	}

	kecp := keepalive.ClientParameters{
		Time:                time.Second * 10,
		Timeout:             time.Second * 30,
		PermitWithoutStream: true,
	}

	DialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(kecp),
		grpc.WithIdleTimeout(DefaultTimeout),
	}

	conn, err := grpc.NewClient(target, DialOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialize grpc client: %w", err)
	}

	macoClient := pb.NewMacoRPCClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), opt.DialTimeout)
	defer cancel()

	callOpts := []grpc.CallOption{}
	_, err = macoClient.Ping(ctx, &pb.PingRequest{}, callOpts...)
	if err != nil {
		return nil, parse(err)
	}

	internalClient := pb.NewInternalRPCClient(conn)

	client := &Client{
		opt:            opt,
		conn:           conn,
		macoClient:     macoClient,
		internalClient: internalClient,
		done:           make(chan struct{}, 1),
	}

	return client, nil
}

func (c *Client) Ping(ctx context.Context) error {
	in := &pb.PingRequest{}
	_, err := c.macoClient.Ping(ctx, in)
	if err != nil {
		return parse(err)
	}
	return nil
}

func (c *Client) Call(ctx context.Context, req *types.CallRequest) (*types.Report, error) {
	in := &pb.CallRequest{
		Request: req,
	}
	rsp, err := c.macoClient.Call(ctx, in)
	if err != nil {
		return nil, parse(err)
	}
	return rsp.Report, nil
}

func (c *Client) NewDispatcher(ctx context.Context, req *types.ConnectRequest) (*Dispatcher, error) {
	opts := c.buildCallOptions()

	connected := &atomic.Bool{}
	connected.Store(false)
	dispatcher := &Dispatcher{
		lg: c.opt.Logger,

		internalClient: c.internalClient,

		connMsg:     req,
		callOptions: opts,
		connected:   connected,
		ech:         make(chan *Event, 10),
		done:        c.done,
	}
	rsp, err := dispatcher.connect(ctx)
	if err != nil {
		return nil, err
	}
	pubKey := filepath.Join(c.opt.DataRoot, "master")
	_ = os.WriteFile(pubKey, rsp.MasterPublicKey, 0600)

	go dispatcher.process()
	return dispatcher, nil
}

func (c *Client) buildCallOptions() []grpc.CallOption {
	opts := []grpc.CallOption{}
	return opts
}

func (c *Client) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	return c.conn.Close()
}

type Event struct {
	EventType types.EventType
	Call      *pb.DispatchCallMsg
	Err       error
}

type Dispatcher struct {
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

func (d *Dispatcher) Context() context.Context {
	return d.stream.Context()
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
	case <-d.stream.Context().Done():
		return nil, d.stream.Context().Err()
	case e := <-d.ech:
		return e, nil
	}
}

func (d *Dispatcher) process() {
	ctx := d.Context()
	attempts := 0
	interval := retryInterval(attempts)

	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		interval = retryInterval(attempts)
		timer.Reset(interval)

		attempts++

		select {
		case <-ctx.Done():
			return
		case <-d.done:
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
					event := &Event{Err: err}
					d.ech <- event
				}
				d.connected.Store(false)
				continue
			} else {
				// 重连成功，重置连接间隔
				attempts = 0
			}
		}

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

func parse(err error) error {
	if v, ok := status.FromError(err); ok {
		return apiErr.FromStatus(v)
	}

	return err
}

func isUnavailable(err error) bool {
	return err == io.EOF ||
		errors.Is(err, context.Canceled) ||
		status.Code(err) == codes.Unavailable
}

func retryInterval(attempts int) time.Duration {
	if attempts <= 0 {
		return time.Second
	}
	if attempts > 5 {
		return time.Minute
	}
	return time.Duration(attempts*10) * time.Second
}

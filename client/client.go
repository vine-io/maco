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
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
)

type Client struct {
	cfg *Config

	conn *grpc.ClientConn

	macoClient     pb.MacoRPCClient
	internalClient pb.InternalRPCClient

	done chan struct{}
}

func NewClient(cfg *Config) (*Client, error) {
	target := cfg.Target

	var tlsConfig *tls.Config
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load certificate pair: %w", err)
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
		}

		if cfg.CaFile != "" {
			caCert, err := os.ReadFile(cfg.CaFile)
			if err != nil {
				return nil, fmt.Errorf("load certificate CA: %w", err)
			}
			caPool := x509.NewCertPool()
			caPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caPool
		}
	}

	var creds credentials.TransportCredentials
	if tlsConfig != nil {
		creds = credentials.NewTLS(tlsConfig)
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	callOpts := []grpc.CallOption{}
	_, err = macoClient.Ping(ctx, &pb.PingRequest{}, callOpts...)
	if err != nil {
		return nil, parse(err)
	}

	internalClient := pb.NewInternalRPCClient(conn)

	client := &Client{
		cfg:            cfg,
		conn:           conn,
		macoClient:     macoClient,
		internalClient: internalClient,
		done:           make(chan struct{}, 1),
	}

	return client, nil
}

func (c *Client) Ping(ctx context.Context) error {
	opts := c.buildCallOptions()

	in := &pb.PingRequest{}
	_, err := c.macoClient.Ping(ctx, in, opts...)
	if err != nil {
		return parse(err)
	}
	return nil
}

func (c *Client) ListMinions(ctx context.Context, stateList ...types.MinionState) (map[types.MinionState][]string, error) {
	opts := c.buildCallOptions()

	in := &pb.ListMinionsRequest{
		StateList: []string{},
	}
	for _, state := range stateList {
		in.StateList = append(in.StateList, string(state))
	}
	rsp, err := c.macoClient.ListMinions(ctx, in, opts...)
	if err != nil {
		return nil, parse(err)
	}
	minions := map[types.MinionState][]string{
		types.Unaccepted: rsp.Unaccepted,
		types.Accepted:   rsp.Accepted,
		types.AutoSign:   rsp.AutoSign,
		types.Denied:     rsp.Denied,
		types.Rejected:   rsp.Rejected,
	}
	return minions, nil
}

func (c *Client) GetMinion(ctx context.Context, name string) (*types.MinionKey, error) {
	opts := c.buildCallOptions()

	in := &pb.GetMinionRequest{
		Name: name,
	}

	rsp, err := c.macoClient.GetMinion(ctx, in, opts...)
	if err != nil {
		return nil, parse(err)
	}
	return rsp.Minion, nil
}

func (c *Client) AcceptMinion(ctx context.Context, minions []string, acceptAll, includeRejected, includeDenied bool) ([]string, error) {
	opts := c.buildCallOptions()

	in := &pb.AcceptMinionRequest{
		Minions:         minions,
		All:             acceptAll,
		IncludeRejected: includeRejected,
		IncludeDenied:   includeDenied,
	}
	rsp, err := c.macoClient.AcceptMinion(ctx, in, opts...)
	if err != nil {
		return nil, parse(err)
	}

	return rsp.Minions, nil
}

func (c *Client) RejectMinion(ctx context.Context, minions []string, rejectAll, includeAccepted, includeDenied bool) ([]string, error) {
	opts := c.buildCallOptions()

	in := &pb.RejectMinionRequest{
		Minions:         minions,
		All:             rejectAll,
		IncludeAccepted: includeAccepted,
		IncludeDenied:   includeDenied,
	}

	rsp, err := c.macoClient.RejectMinion(ctx, in, opts...)
	if err != nil {
		return nil, parse(err)
	}

	return rsp.Minions, nil
}

func (c *Client) PrintMinion(ctx context.Context, minions []string, printAll bool) ([]*types.MinionKey, error) {
	opts := c.buildCallOptions()

	in := &pb.PrintMinionRequest{
		Minions: minions,
		All:     printAll,
	}

	rsp, err := c.macoClient.PrintMinion(ctx, in, opts...)
	if err != nil {
		return nil, parse(err)
	}

	return rsp.Minions, nil
}

func (c *Client) DeleteMinion(ctx context.Context, minions []string, deleteAll bool) ([]string, error) {
	opts := c.buildCallOptions()

	in := &pb.DeleteMinionRequest{
		Minions: minions,
		All:     deleteAll,
	}

	rsp, err := c.macoClient.DeleteMinion(ctx, in, opts...)
	if err != nil {
		return nil, parse(err)
	}

	return rsp.Minions, nil
}

func (c *Client) Call(ctx context.Context, req *types.CallRequest) (*types.Report, error) {
	opts := c.buildCallOptions()

	in := &pb.CallRequest{
		Request: req,
	}
	rsp, err := c.macoClient.Call(ctx, in, opts...)
	if err != nil {
		return nil, parse(err)
	}
	return rsp.Report, nil
}

func (c *Client) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	return c.conn.Close()
}

func (c *Client) buildCallOptions() []grpc.CallOption {
	opts := []grpc.CallOption{}
	return opts
}

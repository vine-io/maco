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

package master

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	gwrt "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/docs"
)

type DispatchStream = grpc.BidiStreamingServer[pb.DispatchRequest, pb.DispatchResponse]

type options struct {
	listener  net.Listener
	cfg       *Config
	storage   *Storage
	scheduler *Scheduler
}

func registerRPCHandler(ctx context.Context, opt *options) (http.Handler, error) {
	cfg := opt.cfg

	macoHl, err := newMacoHandler(ctx, opt.storage, opt.scheduler)
	if err != nil {
		return nil, fmt.Errorf("setup maco handler: %w", err)
	}
	internalHl, err := newInternalHandler(ctx, cfg, opt.scheduler)
	if err != nil {
		return nil, fmt.Errorf("setup internal handler: %w", err)
	}

	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}
	kasp := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Second,
		MaxConnectionAge:      30 * time.Second,
		MaxConnectionAgeGrace: 5 * time.Second,
		Time:                  5 * time.Second,
		Timeout:               3 * time.Second,
	}

	sopts := []grpc.ServerOption{
		//grpc.UnaryInterceptor(interceptor),
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
	}
	gs := grpc.NewServer(sopts...)

	muxOpts := []gwrt.ServeMuxOption{}
	gwmux := gwrt.NewServeMux(muxOpts...)

	pb.RegisterMacoRPCServer(gs, macoHl)
	pb.RegisterInternalRPCServer(gs, internalHl)
	if err = pb.RegisterMacoRPCHandlerServer(ctx, gwmux, macoHl); err != nil {
		return nil, fmt.Errorf("setup maco handler: %w", err)
	}

	serveMux := mux.NewRouter()
	serveMux.Handle("/metrics", promhttp.Handler())

	serveMux.Handle("/v1/",
		wsproxy.WebsocketProxy(
			gwmux,
			wsproxy.WithRequestMutator(
				// Default to the POST method for streams
				func(_ *http.Request, outgoing *http.Request) *http.Request {
					outgoing.Method = "POST"
					return outgoing
				},
			),
			wsproxy.WithMaxRespBodyBufferSize(0x7fffffff),
		),
	)

	serveMux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		openapiYAML, _ := docs.GetOpenYAML()
		w.WriteHeader(http.StatusOK)
		w.Write(openapiYAML)
	})

	if cfg.EnableOpenAPI {
		pattern := "/swagger-ui/"
		zap.L().Info("openapi at " + pattern)
		swaggerFs, err := docs.GetSwagger()
		if err != nil {
			return nil, fmt.Errorf("read swagger file: %w", err)
		}
		serveMux.PathPrefix(pattern).Handler(http.StripPrefix(pattern, http.FileServer(http.FS(swaggerFs))))
		serveMux.PathPrefix("/").Handler(gwmux)
	}

	handler := grpcWithHttp(gs, serveMux)
	return handler, nil
}

type macoHandler struct {
	pb.UnimplementedMacoRPCServer

	ctx context.Context

	storage *Storage
	sch     *Scheduler
}

func newMacoHandler(ctx context.Context, storage *Storage, sch *Scheduler) (pb.MacoRPCServer, error) {
	handler := &macoHandler{ctx: ctx}
	return handler, nil
}

func (h *macoHandler) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Message: "OK"}, nil
}

type internalHandler struct {
	pb.UnimplementedInternalRPCServer

	ctx context.Context
	cfg *Config

	sch *Scheduler
}

func newInternalHandler(ctx context.Context, cfg *Config, sch *Scheduler) (pb.InternalRPCServer, error) {

	handler := &internalHandler{
		ctx: ctx,
		cfg: cfg,
		sch: sch,
	}

	return handler, nil
}

func (h *internalHandler) Dispatch(stream DispatchStream) error {
	rsp, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return status.Errorf(codes.Unknown, "dispatch stream error: %v", err)
	}

	if rsp.Type != types.EventType_EventConnect {
		return status.Errorf(codes.InvalidArgument, "missing connect message")
	}
	connMsg := rsp.Connect
	if connMsg == nil {
		return status.Errorf(codes.InvalidArgument, "missing connect message")
	}

	p, err := h.sch.addStream(connMsg, stream)
	if err != nil {
		return status.Errorf(codes.Internal, "add stream error: %v", err)
	}

	err = p.start()
	if err != nil {
		return status.Errorf(codes.Internal, "start stream error: %v", err)
	}
	return nil
}

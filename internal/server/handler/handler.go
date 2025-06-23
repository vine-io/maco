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

package handler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	gwrt "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/docs"
	"github.com/vine-io/maco/internal/server/config"
)

type Options struct {
	Listener net.Listener
	Cfg      *config.Config
}

func RegisterRPC(ctx context.Context, opt *Options) (http.Handler, error) {
	cfg := opt.Cfg

	macoHl, err := NewMacoHandler(ctx)
	if err != nil {
		return nil, fmt.Errorf("setup maco handler: %w", err)
	}

	sopts := []grpc.ServerOption{
		//grpc.UnaryInterceptor(interceptor),
	}
	gs := grpc.NewServer(sopts...)

	muxOpts := []gwrt.ServeMuxOption{}
	gwmux := gwrt.NewServeMux(muxOpts...)

	pb.RegisterMacoRPCServer(gs, macoHl)
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
}

func NewMacoHandler(ctx context.Context) (pb.MacoRPCServer, error) {
	handler := &macoHandler{ctx: ctx}
	return handler, nil
}

func (h *macoHandler) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Message: "OK"}, nil
}

func (h *macoHandler) Dispatch(stream grpc.BidiStreamingServer[pb.DispatchRequest, pb.DispatchResponse]) error {
	return stream.Send(&pb.DispatchResponse{})
}

func grpcWithHttp(gh *grpc.Server, hh http.Handler) http.Handler {
	h2s := &http2.Server{}
	return h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			gh.ServeHTTP(w, r)
		} else {
			hh.ServeHTTP(w, r)
		}
	}), h2s)
}

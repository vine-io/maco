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
	"strings"
	"time"

	"github.com/gorilla/mux"
	gwrt "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	apiErr "github.com/vine-io/maco/api/errors"
	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/docs"
)

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
	internalHl, err := newInternalHandler(ctx, cfg, opt.storage, opt.scheduler)
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
	handler := &macoHandler{
		ctx:     ctx,
		storage: storage,
		sch:     sch,
	}
	return handler, nil
}

func (h *macoHandler) Ping(ctx context.Context, _ *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{}, nil
}

func (h *macoHandler) ListMinions(ctx context.Context, req *pb.ListMinionsRequest) (*pb.ListMinionsResponse, error) {
	rsp := &pb.ListMinionsResponse{
		Unaccepted: make([]string, 0),
		Accepted:   make([]string, 0),
		AutoSign:   make([]string, 0),
		Denied:     make([]string, 0),
		Rejected:   make([]string, 0),
	}

	for _, value := range req.StateList {
		state := types.MinionState(value)
		minions, err := h.storage.GetMinions(state)
		if err != nil {
			return nil, err
		}

		switch state {
		case types.Unaccepted:
			rsp.Unaccepted = minions
		case types.Accepted:
			rsp.Accepted = minions
		case types.AutoSign:
			rsp.AutoSign = minions
		case types.Denied:
			rsp.Denied = minions
		case types.Rejected:
			rsp.Rejected = minions
		}
	}
	return rsp, nil
}

func (h *macoHandler) GetMinion(ctx context.Context, req *pb.GetMinionRequest) (*pb.GetMinionResponse, error) {
	minion, err := h.storage.GetMinion(req.Name)
	if err != nil {
		return nil, err
	}
	rsp := &pb.GetMinionResponse{
		Minion: minion,
	}
	return rsp, nil
}

func (h *macoHandler) AcceptMinion(ctx context.Context, req *pb.AcceptMinionRequest) (*pb.AcceptMinionResponse, error) {
	targets := make([]string, 0)
	if req.All {
		minions, _ := h.storage.GetMinions(types.Unaccepted)
		if req.IncludeRejected {
			values, _ := h.storage.GetMinions(types.Rejected)
			targets = append(targets, values...)
		}
		if req.IncludeDenied {
			values, _ := h.storage.GetMinions(types.Denied)
			targets = append(targets, values...)
		}

		for _, minion := range minions {
			err := h.storage.AcceptMinion(minion, req.IncludeRejected, req.IncludeDenied)
			if err != nil {
				zap.L().Error("accept minion", zap.String("id", minion), zap.Error(err))
			} else {
				targets = append(targets, minion)
			}
		}
	} else {
		if len(req.Minions) == 0 {
			return nil, apiErr.NewBadRequest("minions is required").ToStatus().Err()
		}
		for _, minion := range req.Minions {
			err := h.storage.AcceptMinion(minion, req.IncludeRejected, req.IncludeDenied)
			if err != nil {
				return nil, apiErr.Parse(err).ToStatus().Err()
			}
			targets = append(targets, minion)
		}
	}

	rsp := &pb.AcceptMinionResponse{
		Minions: targets,
	}
	return rsp, nil
}

func (h *macoHandler) RejectMinion(ctx context.Context, req *pb.RejectMinionRequest) (*pb.RejectMinionResponse, error) {
	targets := make([]string, 0)
	if req.All {
		minions, _ := h.storage.GetMinions(types.Unaccepted)
		if req.IncludeAccepted {
			values, _ := h.storage.GetMinions(types.Accepted)
			targets = append(targets, values...)
		}
		if req.IncludeDenied {
			values, _ := h.storage.GetMinions(types.Denied)
			targets = append(targets, values...)
		}

		for _, minion := range minions {
			err := h.storage.RejectMinion(minion, req.IncludeAccepted, req.IncludeDenied)
			if err != nil {
				zap.L().Error("reject minion", zap.String("id", minion), zap.Error(err))
			} else {
				targets = append(targets, minion)
			}
		}
	} else {
		if len(req.Minions) == 0 {
			return nil, apiErr.NewBadRequest("minions is required").ToStatus().Err()
		}
		for _, minion := range req.Minions {
			err := h.storage.RejectMinion(minion, req.IncludeAccepted, req.IncludeDenied)
			if err != nil {
				return nil, apiErr.Parse(err).ToStatus().Err()
			}
			targets = append(targets, minion)
		}
	}

	rsp := &pb.RejectMinionResponse{
		Minions: targets,
	}
	return rsp, nil
}

func (h *macoHandler) DeleteMinion(ctx context.Context, req *pb.DeleteMinionRequest) (*pb.DeleteMinionResponse, error) {
	targets := make([]string, 0)
	if req.All {
		minions := h.storage.ListMinions()
		for _, minion := range minions {
			err := h.storage.DeleteMinion(minion)
			if err != nil {
				zap.L().Error("delete minion", zap.String("id", minion), zap.Error(err))
			} else {
				targets = append(targets, minion)
			}
		}
	} else {
		if len(req.Minions) == 0 {
			return nil, apiErr.NewBadRequest("minions is required").ToStatus().Err()
		}
		for _, minion := range req.Minions {
			err := h.storage.DeleteMinion(minion)
			if err != nil {
				return nil, apiErr.Parse(err).ToStatus().Err()
			}
			targets = append(targets, minion)
		}
	}

	rsp := &pb.DeleteMinionResponse{
		Minions: targets,
	}
	return rsp, nil
}

func (h *macoHandler) PrintMinion(ctx context.Context, req *pb.PrintMinionRequest) (*pb.PrintMinionResponse, error) {
	targets := make([]*types.MinionKey, 0)
	if req.All {
		minions := h.storage.ListMinions()
		for _, minion := range minions {
			key, _ := h.storage.GetMinion(minion)
			if key != nil {
				targets = append(targets, key)
			}
		}
	} else {
		if len(req.Minions) == 0 {
			return nil, apiErr.NewBadRequest("minions is required").ToStatus().Err()
		}
		for _, minion := range req.Minions {
			key, err := h.storage.GetMinion(minion)
			if err != nil {
				return nil, apiErr.NewNotFound("minion not found").ToStatus().Err()
			}
			targets = append(targets, key)
		}
	}

	rsp := &pb.PrintMinionResponse{
		Minions: targets,
	}
	return rsp, nil
}

func (h *macoHandler) Call(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	in := req.Request
	if opt := in.Options; opt != nil {
		if err := opt.Validate(); err != nil {
			return nil, apiErr.NewBadRequest(err.Error()).ToStatus().Err()
		}
	}
	if in.Timeout <= 0 {
		in.Timeout = 10
	}
	out, err := h.sch.HandleCall(ctx, in)
	if err != nil {
		return nil, apiErr.Parse(err).ToStatus().Err()
	}

	rsp := &pb.CallResponse{
		Report: out.Report.Report,
	}
	return rsp, nil
}

type internalHandler struct {
	pb.UnimplementedInternalRPCServer

	ctx context.Context
	cfg *Config

	storage *Storage
	sch     *Scheduler
}

func newInternalHandler(ctx context.Context, cfg *Config, storage *Storage, sch *Scheduler) (pb.InternalRPCServer, error) {

	handler := &internalHandler{
		ctx:     ctx,
		cfg:     cfg,
		storage: storage,
		sch:     sch,
	}

	return handler, nil
}

func (h *internalHandler) Dispatch(stream pb.InternalRPC_DispatchServer) error {
	rsp, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return apiErr.Parse(err).ToStatus().Err()
	}

	if rsp.Type != types.EventType_EventConnect {
		return status.Errorf(codes.InvalidArgument, "missing connect message")
	}
	connMsg := rsp.Connect
	if connMsg == nil {
		return status.Errorf(codes.InvalidArgument, "missing connect message")
	}
	if len(connMsg.MinionPublicKey) == 0 {
		return status.Errorf(codes.InvalidArgument, "missing minion public key")
	}

	minion := connMsg.Minion
	if len(minion.Ip) == 0 {
		grpcPeer, ok := peer.FromContext(stream.Context())
		if ok {
			minion.Ip = strings.Split(grpcPeer.Addr.String(), ":")[0]
		}
	}

	minion.OnlineTimestamp = time.Now().Unix()
	p, info, err := h.sch.AddStream(connMsg, stream)
	if err != nil {
		return status.Errorf(codes.Internal, "add stream error: %v", err)
	}

	reply := &pb.DispatchResponse{
		Type: types.EventType_EventCall,
		Connect: &types.ConnectResponse{
			Minion:          info.Minion,
			MasterPublicKey: h.storage.ServerRsa().Public,
		},
	}
	if err = stream.Send(reply); err != nil {
		zap.L().Error("reply connect response", zap.Error(err))
	}

	zap.L().Info("add new pipe",
		zap.String("id", minion.Name),
		zap.String("os", minion.Os),
		zap.String("ip", minion.Ip),
	)

	err = p.start()

	zap.L().Info("remove pipe",
		zap.String("id", minion.Name),
		zap.String("os", minion.Os),
		zap.String("ip", minion.Ip),
	)

	if err != nil {
		return status.Errorf(codes.Internal, "start stream error: %v", err)
	}

	return nil
}

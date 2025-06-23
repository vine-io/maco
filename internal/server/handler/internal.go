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

	"google.golang.org/grpc"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/internal/server/config"
)

type internalHandler struct {
	pb.UnimplementedInternalRPCServer

	ctx context.Context
	cfg *config.Config
}

func newInternalHandler(ctx context.Context, cfg *config.Config) (pb.InternalRPCServer, error) {

	return &internalHandler{ctx: ctx}, nil
}

func (h *internalHandler) Dispatch(stream grpc.BidiStreamingServer[pb.DispatchRequest, pb.DispatchResponse]) error {
	return stream.Send(&pb.DispatchResponse{})
}

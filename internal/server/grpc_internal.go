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

package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/internal/server/config"
	"github.com/vine-io/maco/pkg/fsutil"
	"github.com/vine-io/maco/pkg/pemutil"
)

type internalHandler struct {
	pb.UnimplementedInternalRPCServer

	ctx context.Context
	cfg *config.Config

	serverRSA *pemutil.RsaPair
}

func newInternalHandler(ctx context.Context, cfg *config.Config) (pb.InternalRPCServer, error) {

	root := cfg.DataRoot
	pki := filepath.Join(root, "pki")
	serverRSA, err := setupRsaPair(pki)
	if err != nil {
		return nil, err
	}
	handler := &internalHandler{
		ctx:       ctx,
		cfg:       cfg,
		serverRSA: serverRSA,
	}

	return handler, nil
}

func (h *internalHandler) Dispatch(stream grpc.BidiStreamingServer[pb.DispatchRequest, pb.DispatchResponse]) error {
	return stream.Send(&pb.DispatchResponse{})
}

func setupRsaPair(root string) (*pemutil.RsaPair, error) {
	err := fsutil.LoadDir(root)

	zap.L().Info("read pki pairs")

	exists := true
	pem := filepath.Join(root, "server.pem")
	pemBytes, err := os.ReadFile(pem)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		exists = false
	}

	pub := filepath.Join(root, "server.pub")
	pubBytes, err := os.ReadFile(pub)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
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
			return nil, err
		}

		zap.L().Info("generate server pki",
			zap.String("private", pem),
			zap.String("public", pub))

		err = os.WriteFile(pem, pair.Private, 0600)
		if err != nil {
			return nil, fmt.Errorf("save server private key: %v", err)
		}
		err = os.WriteFile(pub, pair.Public, 0600)
		if err != nil {
			return nil, fmt.Errorf("save server public key: %v", err)
		}
	}

	return pair, nil
}

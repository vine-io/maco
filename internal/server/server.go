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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"

	"github.com/vine-io/maco/internal/server/config"
	"github.com/vine-io/maco/internal/server/handler"
	genericserver "github.com/vine-io/maco/pkg/server"
)

var (
	DefaultHeaderBytes = 1024 * 1024 * 50
)

type MacoServer struct {
	genericserver.IEmbedServer

	cfg *config.Config

	serve *http.Server
	db    *badger.DB
}

func NewMacoServer(cfg *config.Config) (*MacoServer, error) {
	lg := cfg.Logger()
	es := genericserver.NewEmbedServer(cfg.Logger())

	dir := filepath.Join(cfg.DataRoot, "store")
	zap.L().Debug("open embed database", zap.String("dir", dir))

	opt := badger.DefaultOptions(dir).
		WithLogger(lg)
	db, err := badger.Open(opt)
	if err != nil {
		return nil, fmt.Errorf("open internal database: %w", err)
	}

	ms := &MacoServer{
		IEmbedServer: es,

		cfg: cfg,
		db:  db,
	}
	return ms, nil
}

func (ms *MacoServer) Start(ctx context.Context) error {

	zap.L().Info("starting maco server")
	if err := ms.start(); err != nil {
		return err
	}
	if err := ms.startServer(ctx); err != nil {
		return err
	}

	ms.Destroy(ms.destroy)

	<-ctx.Done()

	return ms.stop(ctx)
}

func (ms *MacoServer) stop(ctx context.Context) error {
	if err := ms.IEmbedServer.Shutdown(ctx); err != nil {
		return err
	}
	if serve := ms.serve; serve != nil {
		if err := ms.serve.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (ms *MacoServer) destroy() {
	if err := ms.db.Close(); err != nil {
		zap.L().Error("close database", zap.Error(err))
	}
}

func (ms *MacoServer) start() error { return nil }

func (ms *MacoServer) startServer(ctx context.Context) error {
	cfg := ms.cfg

	scheme, ts, err := ms.createListener()
	if err != nil {
		return err
	}

	zap.L().Info("maco-server listening",
		zap.String("scheme", scheme),
		zap.String("addr", ts.Addr().String()))

	opts := &handler.Options{
		Listener: ts,
		Cfg:      cfg,
	}
	hdlr, err := handler.RegisterRPC(ctx, opts)
	ms.serve = &http.Server{
		Handler:        hdlr,
		MaxHeaderBytes: DefaultHeaderBytes,
	}

	go func() {
		_ = ms.serve.Serve(ts)
	}()

	return nil
}

func (ms *MacoServer) createListener() (string, net.Listener, error) {
	cfg := ms.cfg
	listen := cfg.Listen
	zap.L().Debug("listen on " + listen)

	var tlsConfig *tls.Config
	var err error
	if ct := cfg.TLS; ct != nil {
		cert, err := tls.LoadX509KeyPair("server.pem", "server.key")
		if err != nil {
			return "", nil, fmt.Errorf("load certificate pair: %w", err)
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

		if ct.CaFile != "" {
			caCert, err := os.ReadFile(ct.CaFile)
			if err != nil {
				return "", nil, fmt.Errorf("load certificate CA: %w", err)
			}
			caPool := x509.NewCertPool()
			caPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caPool
		}
	}

	var scheme string
	var ln net.Listener
	if tlsConfig != nil {
		scheme = "https"
		ln, err = tls.Listen("tcp", listen, tlsConfig)
	} else {
		scheme = "http"
		ln, err = net.Listen("tcp", listen)
	}
	if err != nil {
		return "", nil, err
	}

	return scheme, ln, nil
}

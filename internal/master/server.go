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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"

	"go.uber.org/zap"

	genericserver "github.com/vine-io/maco/pkg/server"
)

var (
	DefaultHeaderBytes = 1024 * 1024 * 50
)

type Master struct {
	genericserver.IEmbedServer

	cfg *Config

	serve *http.Server
}

func NewMaster(cfg *Config) (*Master, error) {
	lg := cfg.Logger()
	es := genericserver.NewEmbedServer(lg)

	ms := &Master{
		IEmbedServer: es,

		cfg: cfg,
	}
	return ms, nil
}

func (ms *Master) Start(ctx context.Context) error {

	zap.L().Info("starting maco master")
	if err := ms.start(ctx); err != nil {
		return err
	}

	ms.Destroy(ms.destroy)

	<-ctx.Done()

	return ms.stop(ctx)
}

func (ms *Master) stop(ctx context.Context) error {
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

func (ms *Master) destroy() {
}

func (ms *Master) start(ctx context.Context) error {
	cfg := ms.cfg
	lg := cfg.Logger()

	scheme, ts, err := ms.createListener()
	if err != nil {
		return err
	}

	lg.Info("maco-master listening",
		zap.String("scheme", scheme),
		zap.String("addr", ts.Addr().String()))

	sopts := NewOptions(cfg.DataRoot, lg)
	storage, err := newStorage(sopts)
	if err != nil {
		return fmt.Errorf("create storage: %w", err)
	}

	sche, err := NewScheduler(storage)
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}
	go sche.Run(ctx)

	opts := &options{
		listener:  ts,
		cfg:       cfg,
		storage:   storage,
		scheduler: sche,
	}
	hdlr, err := registerRPCHandler(ctx, opts)
	ms.serve = &http.Server{
		Handler:        hdlr,
		MaxHeaderBytes: DefaultHeaderBytes,
	}

	go func() {
		_ = ms.serve.Serve(ts)
	}()

	return nil
}

func (ms *Master) createListener() (string, net.Listener, error) {
	cfg := ms.cfg
	listen := cfg.Listen
	zap.L().Debug("listen on " + listen)

	var tlsConfig *tls.Config
	isHttps := false
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		isHttps = true
	}

	if isHttps {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
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

		if cfg.CaFile != "" {
			caCert, err := os.ReadFile(cfg.CaFile)
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
	var err error
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

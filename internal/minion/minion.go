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

package minion

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/client"
	"github.com/vine-io/maco/pkg/fsutil"
	"github.com/vine-io/maco/pkg/pemutil"
	genericserver "github.com/vine-io/maco/pkg/server"
	version "github.com/vine-io/maco/pkg/version"
)

type Minion struct {
	genericserver.IEmbedServer

	ctx    context.Context
	cancel context.CancelFunc

	cfg *Config

	rsaPair *pemutil.RsaPair

	masterClient *client.Client
}

func NewMinion(cfg *Config) (*Minion, error) {
	lg := cfg.Logger()
	es := genericserver.NewEmbedServer(lg)

	ms := &Minion{
		IEmbedServer: es,

		cfg: cfg,
	}
	return ms, nil
}

func (m *Minion) Start(ctx context.Context) error {

	zap.L().Info("starting maco server")

	m.ctx, m.cancel = context.WithCancel(ctx)
	if err := m.start(ctx); err != nil {
		return err
	}

	m.Destroy(m.destroy)

	<-ctx.Done()

	return m.stop()
}

func (m *Minion) start(ctx context.Context) error {
	cfg := m.cfg
	lg := cfg.Logger()

	pair, err := m.generateRSA()
	if err != nil {
		return fmt.Errorf("generating minion rsa: %w", err)
	}
	m.rsaPair = pair

	minion, err := m.setupMinion()
	if err != nil {
		return fmt.Errorf("setup minion: %w", err)
	}

	target := cfg.Master

	ccfg := client.NewConfig(target)
	masterClient, err := client.NewClient(ccfg)
	if err != nil {
		return fmt.Errorf("connect to maco-master: %w", err)
	}

	in := &types.ConnectRequest{
		Minion:          minion,
		MinionPublicKey: pair.Public,
	}
	var dispatcher *client.Dispatcher
	dispatcher, minion, err = masterClient.NewDispatcher(ctx, in, lg, m.cfg.DataRoot)
	if err != nil {
		return fmt.Errorf("connect to dispatcher: %w", err)
	}
	_ = m.setMinion(minion)

	go m.dispatch(dispatcher)
	return nil
}

func (m *Minion) destroy() {}

func (m *Minion) stop() error {
	m.cancel()

	if err := m.masterClient.Close(); err != nil {
		return fmt.Errorf("close master client: %w", err)
	}
	return nil
}

func (m *Minion) setupMinion() (*types.Minion, error) {
	cfg := m.cfg
	root := cfg.DataRoot

	minion := &types.Minion{}
	hostname, _ := os.Hostname()
	minionPath := filepath.Join(root, "minion")
	data, err := fsutil.Cat(minionPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		_ = json.Unmarshal(data, minion)
	}
	if err != nil {
		minion = &types.Minion{
			Name:     m.cfg.Name,
			Hostname: hostname,
			Tags:     map[string]string{},
			Os:       runtime.GOOS,
			Arch:     runtime.GOARCH,
			Version:  version.GitTag,
		}
	}

	var uid string
	uidPath := filepath.Join(root, "minion_uid")
	data, err = fsutil.Cat(uidPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		uid = uuid.New().String()
		_ = fsutil.Echo(uidPath, []byte(uid), 0600)
	}
	minion.Uid = uid

	return minion, nil
}

func (m *Minion) setMinion(minion *types.Minion) error {
	root := m.cfg.DataRoot
	minionPath := filepath.Join(root, "minion")
	data, err := json.MarshalIndent(minion, "", " ")
	if err != nil {
		return err
	}
	return fsutil.Echo(minionPath, data, 0600)
}

func (m *Minion) generateRSA() (*pemutil.RsaPair, error) {
	cfg := m.cfg
	lg := cfg.Logger()

	exists := true
	root := cfg.DataRoot
	pem := filepath.Join(root, "minion.pem")
	pemBytes, err := os.ReadFile(pem)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		exists = false
	}

	pub := filepath.Join(root, "minion.pub")
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

		lg.Info("generate minion rsa pair",
			zap.String("private", pem),
			zap.String("public", pub))

		err = os.WriteFile(pem, pair.Private, 0600)
		if err != nil {
			return nil, fmt.Errorf("save minion private key: %w", err)
		}
		err = os.WriteFile(pub, pair.Public, 0600)
		if err != nil {
			return nil, fmt.Errorf("save minion public key: %w", err)
		}
	}
	if err = pair.Validate(); err != nil {
		return nil, err
	}

	return pair, nil
}

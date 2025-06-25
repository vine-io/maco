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

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/emirpasic/gods/maps/hashmap"
	"github.com/emirpasic/gods/maps/treemap"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/dbutil"
	"github.com/vine-io/maco/pkg/fsutil"
	"github.com/vine-io/maco/pkg/pemutil"
)

const (
	DefaultPrefix  = "_maco"
	nodePath       = "nodes"
	nodeAutoPath   = "nodes_autosign"
	nodePrePath    = "nodes_pre"
	nodeDeniedPath = "nodes_denied"
	nodeRejectPath = "nodes_rejected"
)

var (
	ErrNotFound = errors.New("not found")
)

type NodeInfo struct {
	*types.Node
}

type Options struct {
	dir string
	lg  *zap.Logger

	// Prefix sets embed database prefix path
	Prefix string
	db     *dbutil.DB
}

func NewOptions(dir string, lg *zap.Logger, db *dbutil.DB) *Options {
	opts := &Options{
		dir:    dir,
		lg:     lg,
		Prefix: DefaultPrefix,
		db:     db,
	}
	return opts
}

type Storage struct {
	*Options

	pair *pemutil.RsaPair

	pmu sync.RWMutex
	// unaccepted saves all unregister nodes
	unaccepted *treemap.Map

	amu sync.RWMutex
	// accepted saves all accepted nodes
	accepted *treemap.Map

	dmu sync.RWMutex
	// denied saves all denied nodes
	denied *hashmap.Map

	rmu sync.RWMutex
	// rejected saves all rejected nodes
	rejected *hashmap.Map
}

func Open(opt *Options) (*Storage, error) {
	if opt.Prefix == "" {
		opt.Prefix = DefaultPrefix
	}
	lg := opt.lg

	dir := filepath.Join(opt.dir, "pki")
	err := fsutil.LoadDir(dir)
	if err != nil {
		return nil, err
	}

	lg.Info("read pki pairs")

	exists := true
	pem := filepath.Join(dir, "server.pem")
	pemBytes, err := os.ReadFile(pem)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		exists = false
	}

	pub := filepath.Join(dir, "server.pub")
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

		lg.Info("generate server pki",
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

	if err = fsutil.LoadDir(filepath.Join(dir, nodePath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(dir, nodeAutoPath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(dir, nodeDeniedPath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(dir, nodePrePath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(dir, nodeRejectPath)); err != nil {
		return nil, err
	}

	unaccepted := treemap.NewWithStringComparator()
	accepted := treemap.NewWithStringComparator()
	denied := hashmap.New()
	rejected := hashmap.New()

	s := &Storage{
		Options:    opt,
		pair:       pair,
		unaccepted: unaccepted,
		accepted:   accepted,
		denied:     denied,
		rejected:   rejected,
	}

	return s, nil
}

func (s *Storage) ServerRsa() *pemutil.RsaPair {
	return s.pair
}

func (s *Storage) AddNode(node *types.Node, pubKey []byte) error {
	id := node.Name
	if err := s.addNode(id, pubKey); err != nil {
		return err
	}

	return s.UpdateNode(node)
}

func (s *Storage) UpdateNode(node *types.Node) error {
	key := filepath.Join(s.Prefix, nodePath, node.Name)
	value, err := proto.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node: %v", err)
	}

	if err = s.db.Set([]byte(key), value); err != nil {
		return fmt.Errorf("save node: %v", err)
	}
	return nil
}

func (s *Storage) GetNode(name string) (*NodeInfo, error) {
	key := filepath.Join(s.Prefix, nodePath, name)

	data, err := s.db.Get([]byte(key))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var node types.Node
	if err = proto.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	info := &NodeInfo{
		Node: &node,
	}
	return info, nil
}

func (s *Storage) AcceptNode(name string, includeRejected bool) error {
	return s.acceptNode(name)
}

func (s *Storage) RejectNode(name string, includeAccepted bool) error {
	return s.rejectNode(name)
}

func (s *Storage) DeleteNode(name string) error {
	if err := s.deleteNode(name); err != nil {
		return err
	}

	key := filepath.Join(s.Prefix, nodePath, name)
	err := s.db.Delete([]byte(key))
	if err != nil {
		return err
	}
	return nil
}

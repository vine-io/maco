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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"

	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/dsutil"
	"github.com/vine-io/maco/pkg/fsutil"
	"github.com/vine-io/maco/pkg/pemutil"
)

const (
	minionPath       = "minions"
	minionAcceptPath = "minions_accept"
	minionAutoPath   = "minions_autosign"
	minionPrePath    = "minions_pre"
	minionDeniedPath = "minions_denied"
	minionRejectPath = "minions_rejected"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidState = fmt.Errorf("invalid state")
)

type MinionInfo struct {
	*types.Minion
	PubKey []byte
}

type Options struct {
	dir string
	lg  *zap.Logger
}

func NewOptions(dir string, lg *zap.Logger) *Options {
	opts := &Options{
		dir: dir,
		lg:  lg,
	}
	return opts
}

type Storage struct {
	*Options

	pair *pemutil.RsaPair

	cmu sync.RWMutex

	minionCache map[types.MinionState]*dsutil.HashSet[string]
}

func Open(opt *Options) (*Storage, error) {
	lg := opt.lg

	root := opt.dir
	err := fsutil.LoadDir(root)
	if err != nil {
		return nil, err
	}

	lg.Info("read master pki pairs")

	exists := true
	pem := filepath.Join(root, "master.pem")
	pemBytes, err := os.ReadFile(pem)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		exists = false
	}

	pub := filepath.Join(root, "master.pub")
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

		lg.Info("generate master pki",
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

	if err = fsutil.LoadDir(filepath.Join(root, minionPath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(root, minionAcceptPath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(root, minionAutoPath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(root, minionDeniedPath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(root, minionPrePath)); err != nil {
		return nil, err
	}
	if err = fsutil.LoadDir(filepath.Join(root, minionRejectPath)); err != nil {
		return nil, err
	}

	ms := map[types.MinionState]*dsutil.HashSet[string]{}
	walks := func(ms map[types.MinionState]*dsutil.HashSet[string], dir string, state types.MinionState) error {
		sets := dsutil.NewHashSet[string]()
		minions, e1 := walkMinions(dir, state)
		if e1 != nil {
			return e1
		}
		for _, minion := range minions {
			sets.Add(minion)
		}
		ms[state] = sets
		return nil
	}
	if err = walks(ms, root, types.Unaccepted); err != nil {
		return nil, err
	}
	if err = walks(ms, root, types.Accepted); err != nil {
		return nil, err
	}
	if err = walks(ms, root, types.AutoSign); err != nil {
		return nil, err
	}
	if err = walks(ms, root, types.Denied); err != nil {
		return nil, err
	}
	if err = walks(ms, root, types.Rejected); err != nil {
		return nil, err
	}

	s := &Storage{
		Options:     opt,
		pair:        pair,
		minionCache: ms,
	}

	return s, nil
}

func (s *Storage) ServerRsa() *pemutil.RsaPair {
	return s.pair
}

func (s *Storage) AddMinion(minion *types.Minion, state types.MinionState, pubKey []byte) error {
	id := minion.Name
	if err := s.addMinion(id, state); err != nil {
		return err
	}

	minionRoot := filepath.Join(s.dir, minionPath, id)
	pubKeyPath := filepath.Join(minionRoot, "minion.pub")
	if err := os.WriteFile(pubKeyPath, pubKey, 0600); err != nil {
		return err
	}

	return s.UpdateMinion(minion)
}

func (s *Storage) UpdateMinion(minion *types.Minion) error {
	minionRoot := filepath.Join(s.dir, minionPath, minion.Name)
	_ = os.MkdirAll(minionRoot, 0700)
	minionId := filepath.Join(minionRoot, "minion")
	data, err := json.Marshal(minion)
	if err != nil {
		return err
	}
	if err = os.WriteFile(minionId, data, 0600); err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetMinion(name string) (*MinionInfo, error) {
	info := &MinionInfo{}

	minionRoot := filepath.Join(s.dir, minionPath, name)
	data, err := os.ReadFile(minionRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var minion types.Minion
	if err = json.Unmarshal(data, &minion); err != nil {
		return nil, err
	}
	pubKey, err := os.ReadFile(filepath.Join(minionRoot, "minion.pub"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	info.Minion = &minion
	info.PubKey = pubKey

	return info, nil
}

func (s *Storage) AcceptMinion(name string, includeRejected, includeDenied bool) error {
	var exists bool

	s.cmu.Lock()
	sets := s.minionCache[types.Unaccepted]
	exists = sets.Contains(name)

	if !exists {
		if includeRejected {
			sets = s.minionCache[types.Rejected]
			exists = sets.Contains(name)
		}
		if includeDenied {
			sets = s.minionCache[types.Denied]
			exists = sets.Contains(name)
		}
	}
	s.cmu.Unlock()

	if !exists {
		return ErrNotFound
	}

	s.cmu.RLock()
	s.minionCache[types.Accepted].Add(name)
	s.cmu.RUnlock()

	return s.acceptMinion(name)
}

func (s *Storage) RejectMinion(name string, includeAccepted bool) error {
	var exists bool

	s.cmu.Lock()
	sets := s.minionCache[types.Unaccepted]
	exists = sets.Contains(name)

	if !exists {
		if includeAccepted {
			sets = s.minionCache[types.Accepted]
			exists = sets.Contains(name)
		}
	}
	s.cmu.Unlock()

	if !exists {
		return ErrNotFound
	}

	s.cmu.RLock()
	s.minionCache[types.Rejected].Add(name)
	s.cmu.RUnlock()

	return s.rejectMinion(name)
}

func (s *Storage) DeleteMinion(name string) error {
	if err := s.deleteMinion(name); err != nil {
		return err
	}

	minionRoot := filepath.Join(s.dir, minionPath, name)
	return os.RemoveAll(minionRoot)
}

func (s *Storage) GetMinions(state types.MinionState) ([]string, error) {
	s.cmu.Lock()
	defer s.cmu.Unlock()

	sets, ok := s.minionCache[state]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidState, state)
	}

	minions := sets.Values()
	return minions, nil
}

func (s *Storage) addMinion(id string, state types.MinionState) error {
	var kind string
	switch state {
	case types.Unaccepted:
		kind = minionPrePath
	case types.Accepted:
		kind = minionAcceptPath
	case types.AutoSign:
		kind = minionAutoPath
	case types.Denied:
		kind = minionDeniedPath
	case types.Rejected:
		kind = minionRejectPath
	default:
		return fmt.Errorf("%w: %s", ErrInvalidState, state)
	}

	source := filepath.Join(s.dir, minionPath, id)
	minionId := filepath.Join(s.dir, kind, id)
	return os.Symlink(source, minionId)
}

func (s *Storage) acceptMinion(id string) error {
	preId := filepath.Join(s.dir, minionPrePath, id)
	sourceId := filepath.Join(s.dir, minionPath, id)
	acceptedId := filepath.Join(s.dir, minionAcceptPath, id)

	if err := os.Symlink(sourceId, acceptedId); err != nil {
		return err
	}

	_ = os.Remove(preId)

	deniedId := filepath.Join(s.dir, minionDeniedPath, id)
	_ = os.Remove(deniedId)

	rejectId := filepath.Join(s.dir, minionRejectPath, id)
	_ = os.Remove(rejectId)
	return nil
}

func (s *Storage) rejectMinion(id string) error {
	preId := filepath.Join(s.dir, minionPrePath, id)
	sourceId := filepath.Join(s.dir, minionPath, id)
	rejectId := filepath.Join(s.dir, minionRejectPath, id)

	if err := os.Symlink(sourceId, rejectId); err != nil {
		return err
	}

	_ = os.Remove(preId)

	autoSignId := filepath.Join(s.dir, minionAutoPath, id)
	_ = os.Remove(autoSignId)

	acceptId := filepath.Join(s.dir, minionAcceptPath, id)
	_ = os.Remove(acceptId)

	return nil
}

func (s *Storage) deleteMinion(id string) error {
	preId := filepath.Join(s.dir, minionPrePath, id)
	exists, err := removeFile(preId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	acceptedId := filepath.Join(s.dir, minionAcceptPath, id)
	exists, err = removeFile(acceptedId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	autoSignId := filepath.Join(s.dir, minionAutoPath, id)
	exists, err = removeFile(autoSignId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	deniedId := filepath.Join(s.dir, minionDeniedPath, id)
	exists, err = removeFile(deniedId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	rejectId := filepath.Join(s.dir, minionRejectPath, id)
	exists, err = removeFile(rejectId)
	if err != nil {
		return err
	}

	if !exists {
		return ErrNotFound
	}

	return nil
}

func walkMinions(root string, state types.MinionState) ([]string, error) {
	var kind string
	switch state {
	case types.Unaccepted:
		kind = minionPrePath
	case types.Accepted:
		kind = minionAcceptPath
	case types.AutoSign:
		kind = minionAutoPath
	case types.Denied:
		kind = minionDeniedPath
	case types.Rejected:
		kind = minionRejectPath
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidState, state)
	}
	dir := filepath.Join(root, kind)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	minions := make([]string, 0, len(files))
	for i, entry := range files {
		minions[i] = entry.Name()
	}
	return minions, nil
}

func removeFile(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	err = os.Remove(path)
	if err != nil {
		return true, err
	}
	return true, nil
}

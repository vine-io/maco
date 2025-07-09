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
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	apiErr "github.com/vine-io/maco/api/errors"
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

type minionEvent struct {
	state   types.MinionState
	minion  string
	deleted bool
}

type storageEvent struct {
	minion *minionEvent
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

	cmu         sync.RWMutex
	minionCache map[types.MinionState]*dsutil.HashSet[string]

	chNextId    *atomic.Int64
	smu         sync.RWMutex
	subscribers map[int64]chan *storageEvent
}

func newStorage(opt *Options) (*Storage, error) {
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

		lg.Info("generate master rsa pair",
			zap.String("private", pem),
			zap.String("public", pub))

		err = os.WriteFile(pem, pair.Private, 0600)
		if err != nil {
			return nil, fmt.Errorf("save master private key: %w", err)
		}
		err = os.WriteFile(pub, pair.Public, 0600)
		if err != nil {
			return nil, fmt.Errorf("save master public key: %w", err)
		}
	}
	if err = pair.Validate(); err != nil {
		return nil, err
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

		chNextId:    &atomic.Int64{},
		subscribers: make(map[int64]chan *storageEvent),
	}

	return s, nil
}

func (s *Storage) ServerRsa() *pemutil.RsaPair {
	return s.pair
}

func (s *Storage) publish(event *storageEvent) {
	s.cmu.Lock()
	defer s.cmu.Unlock()

	for _, ch := range s.subscribers {
		ch <- event
	}
}

func (s *Storage) Subscribe() (chan *storageEvent, func()) {
	ch := make(chan *storageEvent, 1)
	id := s.chNextId.Add(1)

	s.smu.Lock()
	s.subscribers[id] = ch
	s.smu.Unlock()

	stop := func() {
		s.smu.Lock()
		delete(s.subscribers, id)
		s.smu.Unlock()
		close(ch)
	}

	return ch, stop
}

// GetMinions 返回指定状态的 minion 列表，如何 state 类型不正确或者列表为空，返回 ErrNotFound
func (s *Storage) GetMinions(state types.MinionState) ([]string, error) {
	s.cmu.Lock()
	defer s.cmu.Unlock()

	sets, ok := s.minionCache[state]
	if !ok {
		return nil, apiErr.NewBadRequest("minion not found")
	}

	minions := sets.Values()
	return minions, nil
}

func (s *Storage) ListMinions() []string {
	s.cmu.Lock()
	defer s.cmu.Unlock()

	minions := make([]string, 0)
	for _, set := range s.minionCache {
		for _, minion := range set.Values() {
			minions = append(minions, minion)
		}
	}
	return minions
}

func (s *Storage) AddMinion(minion *types.Minion, pubKey []byte, autoSign, autoDenied bool) (*types.MinionKey, error) {
	info := &types.MinionKey{
		Minion: minion,
		PubKey: s.pair.Public,
	}

	state, err := s.getUpdate(minion.Name)
	if err != nil {
		if !apiErr.IsNotFound(err) {
			return nil, err
		}

		minion.RegistryTimestamp = time.Now().Unix()

		// minion 不存在，创建并保存数据
		state = types.Unaccepted
		if autoSign {
			state = types.AutoSign
		}
		if autoDenied {
			state = types.Denied
		}
		info.State = string(state)

		name := minion.Name
		minionRoot := filepath.Join(s.dir, minionPath, name)
		_ = os.MkdirAll(minionRoot, 0700)

		if err = s.addMinion(name, autoSign, autoDenied); err != nil {
			return info, err
		}

		pubKeyPath := filepath.Join(minionRoot, "minion.pub")
		if err = os.WriteFile(pubKeyPath, pubKey, 0600); err != nil {
			return info, err
		}

		if err = s.setUpdate(minion.Name, state); err != nil {
			return info, err
		}

		s.cmu.Lock()
		sets := s.minionCache[state]
		sets.Add(minion.Name)
		s.cmu.Unlock()
	}

	info.State = string(state)
	err = s.updateMinion(minion)
	return info, err
}

func (s *Storage) updateMinion(minion *types.Minion) error {
	minionRoot := filepath.Join(s.dir, minionPath, minion.Name)
	_ = os.MkdirAll(minionRoot, 0700)
	minionId := filepath.Join(minionRoot, "minion")
	data, err := json.MarshalIndent(minion, "", " ")
	if err != nil {
		return err
	}
	if err = fsutil.Echo(minionId, data, 0600); err != nil {
		return err
	}

	return nil
}

func (s *Storage) setUpdate(name string, state types.MinionState) error {
	filename := filepath.Join(s.dir, minionPath, name, "state")
	data := []byte(state)
	return fsutil.Echo(filename, data, 0600)
}

func (s *Storage) getUpdate(name string) (types.MinionState, error) {
	data, err := fsutil.Cat(filepath.Join(s.dir, minionPath, name, "state"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", apiErr.NewNotFound("minion not found")
		}
		return "", err
	}
	return types.MinionState(data), nil
}

func (s *Storage) getMinion(name string) (*types.Minion, error) {
	root := filepath.Join(s.dir, minionPath, name)
	data, err := fsutil.Cat(filepath.Join(root, "minion"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apiErr.NewNotFound("minion not found")
		}
		return nil, err
	}
	var minion types.Minion
	if err = json.Unmarshal(data, &minion); err != nil {
		return nil, err
	}
	return &minion, nil
}

func (s *Storage) GetMinion(name string) (*types.MinionKey, error) {
	info := &types.MinionKey{}

	root := filepath.Join(s.dir, minionPath, name)
	minion, err := s.getMinion(name)
	if err != nil {
		return nil, err
	}
	pubKey, err := os.ReadFile(filepath.Join(root, "minion.pub"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apiErr.NewNotFound("minion not found")
		}
		return nil, err
	}
	stateByte, err := fsutil.Cat(filepath.Join(root, "state"))
	if err != nil {
		return nil, err
	}

	info.Minion = minion
	info.PubKey = pubKey
	info.State = string(stateByte)

	return info, nil
}

func (s *Storage) AcceptMinion(name string, includeRejected, includeDenied bool) error {
	var exists bool

	s.cmu.RLock()
	sets := s.minionCache[types.Unaccepted]
	exists = sets.Contains(name)

	if exists {
		sets.Remove(name)
	} else {
		if includeRejected {
			sets = s.minionCache[types.Rejected]
			exists = sets.Contains(name)
			if exists {
				sets.Remove(name)
			}
		}
		if !exists && includeDenied {
			sets = s.minionCache[types.Denied]
			exists = sets.Contains(name)
			if exists {
				sets.Remove(name)
			}
		}
	}
	s.cmu.RUnlock()

	if !exists {
		return apiErr.NewNotFound("minion not found")
	}

	s.cmu.Lock()
	s.minionCache[types.Accepted].Add(name)
	s.cmu.Unlock()

	if err := s.acceptMinion(name); err != nil {
		return err
	}

	_ = s.setUpdate(name, types.Accepted)

	event := &minionEvent{minion: name, state: types.Accepted}
	go s.publish(&storageEvent{minion: event})

	return nil
}

func (s *Storage) RejectMinion(name string, includeAccepted, includeDenied bool) error {
	var exists bool

	s.cmu.Lock()
	sets := s.minionCache[types.Unaccepted]
	exists = sets.Contains(name)

	if exists {
		sets.Remove(name)
	} else {
		if includeAccepted {
			sets = s.minionCache[types.Accepted]
			exists = sets.Contains(name)
			if exists {
				sets.Remove(name)
			} else {
				sets = s.minionCache[types.AutoSign]
				exists = sets.Contains(name)
				if exists {
					sets.Remove(name)
				}
			}
		}
		if !exists && includeDenied {
			sets = s.minionCache[types.Denied]
			exists = sets.Contains(name)
			if exists {
				sets.Remove(name)
			}
		}
	}
	s.cmu.Unlock()

	if !exists {
		return apiErr.NewNotFound("minion not found")
	}

	s.cmu.Lock()
	s.minionCache[types.Rejected].Add(name)
	s.cmu.Unlock()

	if err := s.rejectMinion(name); err != nil {
		return err
	}

	_ = s.setUpdate(name, types.Rejected)

	event := &minionEvent{minion: name, state: types.Rejected}
	go s.publish(&storageEvent{minion: event})

	return nil
}

func (s *Storage) DeleteMinion(name string) error {
	state, err := s.getUpdate(name)
	if err != nil {
		return err
	}

	s.cmu.Lock()
	sets := s.minionCache[state]
	sets.Remove(name)
	s.cmu.Unlock()

	if err := s.deleteMinion(name); err != nil {
		return err
	}

	event := &minionEvent{minion: name, state: state, deleted: true}
	go s.publish(&storageEvent{minion: event})

	minionRoot := filepath.Join(s.dir, minionPath, name)
	if err = os.RemoveAll(minionRoot); err != nil {
		s.lg.Sugar().Errorf("remove minion %s failed: %v", name, err)
	}

	return nil
}

func (s *Storage) addMinion(id string, autoSign, autoDenied bool) error {

	state := types.Unaccepted
	kind := minionPrePath
	if autoSign {
		state = types.AutoSign
		kind = minionAutoPath
	}
	if autoDenied {
		state = types.Denied
		kind = minionDeniedPath
	}

	source := filepath.Join(s.dir, minionPath, id)
	minionId := filepath.Join(s.dir, kind, id)
	stateId := filepath.Join(minionId, "state")
	_ = os.WriteFile(stateId, []byte(state), 0600)
	if fsutil.FileExists(minionId) {
		sl, _ := os.Readlink(minionId)
		if sl == source {
			return nil
		}
	}
	return os.Symlink(source, minionId)
}

func (s *Storage) acceptMinion(id string) error {
	state, err := s.getUpdate(id)
	if err != nil {
		return err
	}

	sourceId := filepath.Join(s.dir, minionPath, id)
	acceptedId := filepath.Join(s.dir, minionAcceptPath, id)

	if err := os.Symlink(sourceId, acceptedId); err != nil {
		return err
	}

	kind, err := parseState(state)
	if err != nil {
		return nil
	}
	sl := filepath.Join(s.dir, kind, id)
	if err = os.Remove(sl); err != nil {
		s.lg.Sugar().Error("remove %s: %v", sl, err)
	}
	return nil
}

func (s *Storage) rejectMinion(id string) error {
	state, err := s.getUpdate(id)
	if err != nil {
		return err
	}

	sourceId := filepath.Join(s.dir, minionPath, id)
	rejectId := filepath.Join(s.dir, minionRejectPath, id)
	if err = os.Symlink(sourceId, rejectId); err != nil {
		return err
	}

	kind, err := parseState(state)
	if err != nil {
		return nil
	}
	sl := filepath.Join(s.dir, kind, id)
	if err = os.Remove(sl); err != nil {
		s.lg.Sugar().Error("remove %s: %v", sl, err)
	}
	return nil
}

func (s *Storage) deleteMinion(id string) error {
	state, err := s.getUpdate(id)
	if err != nil {
		return err
	}
	kind, err := parseState(state)
	if err != nil {
		return nil
	}
	sl := filepath.Join(s.dir, kind, id)
	if err = os.Remove(sl); err != nil {
		s.lg.Sugar().Error("remove %s: %v", sl, err)
	}
	return nil
}

func walkMinions(root string, state types.MinionState) ([]string, error) {
	kind, err := parseState(state)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, kind)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	minions := make([]string, len(files))
	for i, entry := range files {
		minions[i] = entry.Name()
	}
	return minions, nil
}

func parseState(state types.MinionState) (string, error) {
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
		return "", apiErr.NewBadRequest("unknown minion state")
	}
	return kind, nil
}

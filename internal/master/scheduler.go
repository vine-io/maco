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
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"

	apiErr "github.com/vine-io/maco/api/errors"
	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/dsutil"
	"github.com/vine-io/maco/pkg/pemutil"
)

type DispatchStream interface {
	Context() context.Context
	Send(req *pb.DispatchResponse) error
	Recv() (*pb.DispatchRequest, error)
}

// 该消息从 pipe 传输到 Scheduler
type message struct {
	// 请求 id
	id uint64
	// minion id, 关联 pipe.name
	name string
	// pipe 是否注销
	done bool
	// pipe 中出现错误，同时pipe注销
	err error
	// Call 请求返回的结果
	call *types.CallResponse
}

type Request struct {
	Call *types.CallRequest
}

type Response struct {
	Report *types.Report
}

// This is an application-wide global ID allocator.  Unfortunately we need
// to have unique IDs globally to permit certain things to work
// correctly.
type idAllocator struct {
	used map[uint64]struct{}
	next uint64
	lock sync.Mutex
}

func newIDAllocator() *idAllocator {
	b := make([]byte, 8)
	// The following could in theory fail, but in that case
	// we will wind up with IDs starting at zero.  It should
	// not happen unless the platform can't get good entropy.
	_, _ = rand.Read(b)
	used := make(map[uint64]struct{})
	next := binary.BigEndian.Uint64(b)
	alloc := &idAllocator{
		used: used,
		next: next,
	}
	return alloc
}

func (alloc *idAllocator) Get() uint64 {
	alloc.lock.Lock()
	defer alloc.lock.Unlock()
	for {
		id := alloc.next & 0x7fffffff
		alloc.next++
		if id == 0 {
			continue
		}
		if _, ok := alloc.used[id]; ok {
			continue
		}
		alloc.used[id] = struct{}{}
		return id
	}
}

func (alloc *idAllocator) Free(id uint64) {
	alloc.lock.Lock()
	if _, ok := alloc.used[id]; !ok {
		panic("free of unused pipe ID")
	}
	delete(alloc.used, id)
	alloc.lock.Unlock()
}

type pipe struct {
	ctx context.Context

	name   string
	ip     string
	groups []string

	// master private rsa key
	rsaPair *pemutil.RsaPair

	// minion public rsa key
	pubKey []byte

	stream DispatchStream
	mch    chan<- *message

	stopCh chan struct{}
}

func (p *pipe) send(in *Request) error {
	rsp := &pb.DispatchResponse{}
	if call := in.Call; call != nil {
		rsp.Type = types.EventType_EventCall
		b, err := msgpack.Marshal(call)
		if err != nil {
			return fmt.Errorf("serialize dispatch message: %w", err)
		}
		msg, err := pemutil.EncodeByRSA(b, p.pubKey)
		if err != nil {
			return fmt.Errorf("encode dispatch message: %w", err)
		}
		rsp.Call = &pb.DispatchCallMsg{Id: call.Id, Data: msg}
	}

	return p.stream.Send(rsp)
}

func (p *pipe) start() error {
	for {
		select {
		case <-p.ctx.Done(): // minion 自动断开
			p.mch <- &message{name: p.name, done: true}
			return nil
		case <-p.stopCh: // pipe 手动停止
			return nil
		default:
		}

		req, err := p.stream.Recv()
		if err != nil {
			// minion 连接断开
			if err == io.EOF || errors.Is(err, context.Canceled) {
				p.mch <- &message{name: p.name, done: true}
				return nil
			}
			p.mch <- &message{name: p.name, err: err, done: true}
			return err
		}

		switch req.Type {
		case types.EventType_EventCall:
			msg := req.Call
			if msg == nil {
				continue
			}
			if len(msg.Error) != 0 {
				p.mch <- &message{id: msg.Id, name: p.name, done: true}
				continue
			}
			b, dErr := pemutil.DecodeByRSA(msg.Data, p.rsaPair.Private)
			if dErr != nil {
				p.mch <- &message{id: msg.Id, name: p.name, err: dErr}
				continue
			}
			callRsp := &types.CallResponse{}
			err = msgpack.Unmarshal(b, callRsp)
			if err != nil {
				p.mch <- &message{id: msg.Id, name: p.name, err: err}
			} else {
				p.mch <- &message{id: msg.Id, name: p.name, call: callRsp}
			}
		}
	}
}

func (p *pipe) stop() {
	select {
	case <-p.stopCh:
	case <-p.ctx.Done():
	default:
		close(p.stopCh)
	}
}

type jobPack struct {
	name string
	call *types.CallResponse
}

type task struct {
	id uint64

	gets  uint32
	total uint32

	ch chan *jobPack

	report *types.Report
}

func newTask(id uint64, total uint32, report *types.Report) *task {

	j := &task{
		id:     id,
		total:  total,
		ch:     make(chan *jobPack, 1),
		report: report,
	}
	return j
}

func (t *task) notify(name string, payload *types.CallResponse) {
	pack := &jobPack{
		name: name,
		call: payload,
	}
	t.ch <- pack
}

func (t *task) execute(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-t.ch:
			t.gets += 1
			call := p.call
			if call == nil {
				continue
			}

			item := &types.ReportItem{
				Minion: p.name,
				Error:  call.Error,
				Data:   call.Result,
			}
			switch call.Type {
			case types.ResultType_ResultSkip:
			case types.ResultType_ResultOk:
				item.Result = true
			case types.ResultType_ResultError:
			}

			//report := t.report
			t.report.Items = append(t.report.Items, item)

			if t.gets >= t.total {
				return nil
			}
		}
	}
}

type Scheduler struct {
	pmu sync.RWMutex
	// 建立连接的 minion
	pipes *haxmap.Map[string, *pipe]

	minions *dsutil.SafeHashSet[string]
	// 所有离线的 minion
	downMinions *dsutil.SafeHashSet[string]

	storage *Storage

	idAlloc *idAllocator

	tmu       sync.RWMutex
	taskStore map[uint64]*task

	mch         chan *message
	ech         chan *storageEvent
	eventCancel func()
}

func NewScheduler(storage *Storage) (*Scheduler, error) {

	minions := dsutil.NewSafeHashSet[string]()
	downMinions := dsutil.NewSafeHashSet[string]()
	accepts, _ := storage.GetMinions(types.Accepted)
	autos, _ := storage.GetMinions(types.AutoSign)
	for _, name := range accepts {
		minions.Add(name)
		downMinions.Add(name)
	}
	for _, name := range autos {
		minions.Add(name)
		downMinions.Add(name)
	}

	pipes := haxmap.New[string, *pipe]()

	idAlloc := newIDAllocator()
	taskStore := make(map[uint64]*task)

	ech, eventCancel := storage.Subscribe()

	sch := &Scheduler{
		pipes:       pipes,
		minions:     minions,
		downMinions: downMinions,
		storage:     storage,
		idAlloc:     idAlloc,
		taskStore:   taskStore,
		mch:         make(chan *message, 100),
		ech:         ech,
		eventCancel: eventCancel,
	}

	return sch, nil
}

func (s *Scheduler) AddStream(in *types.ConnectRequest, stream DispatchStream) (*pipe, *types.MinionKey, error) {
	name := in.Minion.Name

	s.pmu.RLock()
	_, exists := s.pipes.Get(name)
	s.pmu.RUnlock()
	if exists {
		return nil, nil, fmt.Errorf("minion %s already exists", name)
	}

	autoSign := false
	autoDenied := true
	//TODO: 读取配置文件，确认 minion 是否支持自动注册
	info, err := s.storage.AddMinion(in.Minion, in.MinionPublicKey, autoSign, autoDenied)
	if err != nil {
		return nil, nil, err
	}
	state := types.MinionState(info.State)

	pair := s.storage.ServerRsa()
	p := &pipe{
		ctx:     stream.Context(),
		name:    name,
		ip:      info.Minion.Ip,
		groups:  make([]string, 0),
		rsaPair: pair,
		pubKey:  pair.Public,
		stream:  stream,
		mch:     s.mch,
		stopCh:  make(chan struct{}, 1),
	}

	s.pmu.Lock()
	s.pipes.Set(name, p)
	s.pmu.Unlock()

	if state == types.Accepted || state == types.AutoSign {
		s.minions.Add(name)
	}

	s.downMinions.Remove(name)

	return p, info, nil
}

func (s *Scheduler) sendTo(name string, req *Request) error {
	s.pmu.RLock()
	ok := s.minions.Contains(name)
	s.pmu.RUnlock()
	if !ok {
		return fmt.Errorf("target is not be accepted")
	}

	s.pmu.RLock()
	p, ok := s.pipes.Get(name)
	s.pmu.RUnlock()

	if !ok {
		return fmt.Errorf("name is not online")
	}

	in := &Request{Call: req.Call}

	return p.send(in)
}

func (s *Scheduler) selectPipe(options *types.SelectionOptions) ([]*pipe, error) {
	pipes := make([]*pipe, 0)
	s.pmu.RLock()
	defer s.pmu.RUnlock()

	//for _, p := range pipes {
	//
	//}

	return pipes, nil
}

func (s *Scheduler) HandleCall(ctx context.Context, in *types.CallRequest) (*Response, error) {

	//req.Call
	ctx, cancel := context.WithTimeout(ctx, time.Duration(in.Timeout)*time.Second)
	defer cancel()

	report := &types.Report{
		Items:   make([]*types.ReportItem, 0),
		Summary: &types.ReportSummary{},
	}

	nextId := s.idAlloc.Get()
	defer s.idAlloc.Free(nextId)

	in.Id = nextId

	targets := make([]string, 0)
	//if in.Selector != nil {
	//	targets = in.Selector.Minions
	//}
	if len(targets) == 0 {
		return nil, apiErr.NewBadRequest("no targets")
	}

	total := uint32(0)
	//pipes := make([]*pipe, 0)
	//for _, name := range targets {
	//	if !s.minions.Contains(name) {
	//		item := &types.ReportItem{
	//			Minion: name,
	//			Result: false,
	//			Error:  fmt.Sprintf("minion %s is not accepted", name),
	//		}
	//		report.Items = append(report.Items, item)
	//		continue
	//	}
	//
	//	s.pmu.RLock()
	//	p, ok := s.pipes.Get(name)
	//	s.pmu.RUnlock()
	//	if ok {
	//		total += 1
	//		pipes = append(pipes, p)
	//	} else {
	//		item := &types.ReportItem{
	//			Minion: name,
	//			Result: false,
	//			Error:  fmt.Sprintf("minion %s is not online", name),
	//		}
	//		report.Items = append(report.Items, item)
	//	}
	//}
	//
	//if len(pipes) == 0 {
	//	return nil, apiErr.NewBadRequest("no available minions")
	//}

	pipes, err := s.selectPipe(in.Options)

	t := newTask(nextId, total, report)
	s.tmu.Lock()
	s.taskStore[nextId] = t
	s.tmu.Unlock()

	for _, p := range pipes {
		err := p.send(&Request{Call: in})
		if err != nil {
			zap.S().Errorf("send msg to %s: %v", p.name, err)
		}
	}

	err = t.execute(ctx)
	if err != nil {
		return nil, err
	}

	s.tmu.Lock()
	delete(s.taskStore, nextId)
	s.tmu.Unlock()

	rsp := &Response{
		Report: report,
	}

	return rsp, nil
}

func (s *Scheduler) Run(ctx context.Context) {
	defer s.eventCancel()

	for {
		select {
		case <-ctx.Done():
			return
		case m := <-s.mch:
			if m.done {
				s.removePipe(m.name)
				continue
			}

			msg := m.call
			if msg == nil {
				msg = &types.CallResponse{
					Id: m.id,
				}
			}
			if m.err != nil {
				msg.Type = types.ResultType_ResultError
				msg.Error = m.err.Error()
			}

			id := m.id
			s.tmu.RLock()
			t, ok := s.taskStore[id]
			if ok {
				t.notify(m.name, msg)
			}
			s.tmu.RUnlock()
		case e, ok := <-s.ech:
			if !ok {
				continue
			}

			if me := e.minion; me != nil {
				if me.deleted {
					s.minions.Remove(me.minion)
				} else {
					switch me.state {
					case types.Accepted, types.AutoSign:
						s.minions.Add(me.minion)
					case types.Rejected, types.Denied:
						s.minions.Remove(me.minion)
					}
				}
			}
		}
	}
}

func (s *Scheduler) removePipe(name string) {
	if name == "" {
		panic("pipe name is empty")
	}

	s.pmu.Lock()
	s.pipes.Del(name)
	s.pmu.Unlock()

	s.downMinions.Add(name)
	minion, _ := s.storage.getMinion(name)
	if minion != nil {
		minion.OfflineTimestamp = time.Now().Unix()
		_ = s.storage.updateMinion(minion)
	}
}

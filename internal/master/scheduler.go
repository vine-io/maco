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
	"fmt"
	"io"
	"sync"

	"github.com/alphadose/haxmap"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/dsutil"
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

	name string

	stream DispatchStream
	msg    chan<- *message

	stopCh chan struct{}
}

func newPipe(name string, stream DispatchStream, msg chan<- *message) *pipe {
	p := &pipe{
		ctx:    stream.Context(),
		name:   name,
		stream: stream,
		msg:    msg,
		stopCh: make(chan struct{}, 1),
	}

	return p
}

func (p *pipe) send(in *pb.DispatchResponse) error {
	return p.stream.Send(in)
}

func (p *pipe) start() error {
	for {
		select {
		case <-p.ctx.Done(): // minion 自动断开
			return nil
		case <-p.stopCh: // pipe 手动停止
			return nil
		default:
		}

		req, err := p.stream.Recv()
		if err != nil {
			// minion 连接断开
			if err == io.EOF {
				p.msg <- &message{name: p.name, done: true}
				return nil
			}
			p.msg <- &message{name: p.name, err: err}
			return err
		}

		switch req.Type {
		case types.EventType_EventCall:
			rsp := req.Call
			if rsp != nil {
				p.msg <- &message{id: rsp.Id, call: rsp}
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

type Request struct {
	Call *types.CallRequest
}

type Response struct {
	Report *types.Report
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

func (t *task) wait(ctx context.Context) error {
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

			switch call.Type {
			case types.ResultType_ResultSkip:
			case types.ResultType_ResultOk:
			case types.ResultType_ResultError:
			}

			//report := t.report

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

	mmu sync.RWMutex
	// 所有可用的 minion
	minions *dsutil.HashSet[string]

	storage *Storage

	idAlloc *idAllocator

	tmu       sync.RWMutex
	taskStore map[uint64]*task

	mch chan *message
}

func NewScheduler(storage *Storage) (*Scheduler, error) {

	minions := &dsutil.HashSet[string]{}
	accepts, _ := storage.GetMinions(types.Accepted)
	autos, _ := storage.GetMinions(types.AutoSign)
	for _, name := range accepts {
		minions.Add(name)
	}
	for _, name := range autos {
		minions.Add(name)
	}

	pipes := haxmap.New[string, *pipe]()

	idAlloc := newIDAllocator()

	sch := &Scheduler{
		pipes:   pipes,
		minions: minions,
		storage: storage,
		idAlloc: idAlloc,
		mch:     make(chan *message, 100),
	}

	return sch, nil
}

func (s *Scheduler) addStream(in *types.ConnectRequest, stream DispatchStream) (*pipe, error) {
	name := in.Minion.Name

	autoSign := false
	//TODO: 读取配置文件，确认 minion 是否为自动注册
	state, err := s.storage.AddMinion(in.Minion, in.MinionPublicKey, autoSign)
	if err != nil {
		return nil, err
	}

	p := newPipe(name, stream, s.mch)
	s.pmu.Lock()
	s.pipes.Set(name, p)
	s.pmu.Unlock()

	if state == types.Accepted || state == types.AutoSign {
		s.mmu.Lock()
		s.minions.Add(name)
		s.mmu.Unlock()
	}

	return p, nil
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

	return p.send(&pb.DispatchResponse{
		Type: types.EventType_EventCall,
		Call: req.Call,
	})
}

func (s *Scheduler) Handle(ctx context.Context, req *Request) (*Response, error) {

	//req.Call.
	//in := req.Call
	//targets := s.minions.Values()

	report := &types.Report{
		Items:   make([]*types.ReportItem, 0),
		Summary: &types.ReportSummary{},
	}

	nextId := s.idAlloc.Get()
	defer s.idAlloc.Free(nextId)

	total := uint32(0)
	s.mmu.RLock()
	total = uint32(s.minions.Size())
	s.mmu.RUnlock()

	t := newTask(nextId, total, report)
	s.tmu.Lock()
	s.taskStore[nextId] = t
	s.tmu.Unlock()

	err := t.wait(ctx)
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
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-s.mch:
			if m.done || m.err != nil {
				s.removePipe(m.name)
				continue
			}

			if call := m.call; call != nil {
				id := call.Id
				s.mmu.RLock()
				t, ok := s.taskStore[id]
				if ok {
					t.notify(m.name, call)
				}
				s.mmu.RUnlock()
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
}

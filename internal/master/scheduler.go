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
	"io"
	"sync"

	"github.com/alphadose/haxmap"

	pb "github.com/vine-io/maco/api/rpc"
	"github.com/vine-io/maco/api/types"
	"github.com/vine-io/maco/pkg/dsutil"
)

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
	Call *pb.CallRequest
}

type Response struct {
	Report *types.Report
}

type Scheduler struct {
	pmu   sync.RWMutex
	pipes *haxmap.Map[string, *pipe]

	gmu sync.RWMutex
	// minion 组，记录 minion 组和 minion 之间的映射关系
	groups *haxmap.Map[string, *dsutil.HashSet[string]]

	rgmu sync.RWMutex
	// groups 的反向关系
	rgroups *haxmap.Map[string, *dsutil.HashSet[string]]

	storage *Storage

	mch chan *message
}

func NewScheduler(storage *Storage) (*Scheduler, error) {

	pipes := haxmap.New[string, *pipe]()
	groups := haxmap.New[string, *dsutil.HashSet[string]]()
	sch := &Scheduler{
		pipes:   pipes,
		groups:  groups,
		storage: storage,
		mch:     make(chan *message, 100),
	}

	return sch, nil
}

func (s *Scheduler) addStream(in *types.ConnectRequest, stream DispatchStream) (*pipe, error) {
	p := newPipe(in.Minion.Name, stream, s.mch)
	return p, nil
}

func (s *Scheduler) Handle(ctx context.Context, req *Request) (*Response, error) {

	rsp := &Response{}

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

			//TODO: 处理 minion 返回消息
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

	var groups []string

	s.rgmu.Lock()
	value, ok := s.rgroups.Get(name)
	if ok {
		groups = value.Values()
		value.Clear()
		s.rgroups.Del(name)
	}
	s.rgmu.Unlock()

	s.gmu.Lock()
	for _, group := range groups {
		v, ok := s.groups.Get(group)
		if ok {
			v.Clear()
			s.groups.Del(group)
		}
	}
	s.gmu.Unlock()

}

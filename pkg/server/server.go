/*
Copyright 2023 The olive Authors

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
	"sync"

	"go.uber.org/zap"
)

type IEmbedServer interface {
	// StopNotify returns a channel that receives an empty struct
	// when the server is stopped.
	StopNotify() <-chan struct{}
	// StoppingNotify returns a channel that receives an empty struct
	// when the server is being stopped.
	StoppingNotify() <-chan struct{}
	// GoAttach creates a goroutine on a given function and tracks it using the waitgroup.
	// The passed function should interrupt on s.StoppingNotify().
	GoAttach(fn func())
	// Destroy run destroy function when the server stop
	Destroy(fn func())
	// Shutdown sends signal to stop channel and all goroutines stop
	Shutdown(ctx context.Context) error
}

type embedServer struct {
	lg *zap.Logger

	stopping chan struct{}
	done     chan struct{}
	stop     chan struct{}

	wgMu sync.RWMutex
	wg   sync.WaitGroup
}

func NewEmbedServer(lg *zap.Logger) IEmbedServer {
	s := &embedServer{
		lg:       lg,
		stopping: make(chan struct{}, 1),
		done:     make(chan struct{}, 1),
		stop:     make(chan struct{}, 1),
		wgMu:     sync.RWMutex{},
		wg:       sync.WaitGroup{},
	}

	return s
}

func (s *embedServer) StopNotify() <-chan struct{} { return s.done }

func (s *embedServer) StoppingNotify() <-chan struct{} { return s.stopping }

func (s *embedServer) GoAttach(fn func()) {
	s.wgMu.RLock() // this blocks with ongoing close(s.stopping)
	select {
	case <-s.stopping:
		s.lg.Warn("server has stopped; skipping GoAttach")
		s.wgMu.RUnlock()
		return
	default:
	}
	s.wgMu.RUnlock()

	// now safe to add since waitgroup wait has not started yet
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fn()
	}()
}

func (s *embedServer) Destroy(fn func()) {
	go s.destroy(fn)
}

func (s *embedServer) destroy(fn func()) {
	defer func() {
		s.wgMu.Lock() // block concurrent waitgroup adds in GoAttach while stopping
		close(s.stopping)
		s.wgMu.Unlock()

		s.wg.Wait()

		// clean something
		s.lg.Debug("server has stopped, running destroy operations")
		fn()

		close(s.done)
	}()

	<-s.stop
}

func (s *embedServer) Shutdown(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.stop:
	case <-s.done:
		return nil
	default:
		close(s.stop)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
	}
	return nil
}

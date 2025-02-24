/*
Copyright 2024 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lock

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dapr/kit/concurrency/fifo"
)

var errLockClosed = errors.New("lock closed")

type hold struct {
	writeLock bool
	rctx      context.Context
	respCh    chan *holdresp
}

type holdresp struct {
	rctx   context.Context
	cancel context.CancelFunc
	err    error
}

type OuterCancel struct {
	ch chan *hold

	lock chan struct{}

	wg          sync.WaitGroup
	rcancelLock sync.Mutex
	rcancelx    uint64
	rcancels    map[uint64]context.CancelFunc

	closeCh      chan struct{}
	shutdownLock *fifo.Mutex
}

func NewOuterCancel() *OuterCancel {
	return &OuterCancel{
		lock:         make(chan struct{}, 1),
		ch:           make(chan *hold, 1),
		rcancels:     make(map[uint64]context.CancelFunc),
		closeCh:      make(chan struct{}),
		shutdownLock: fifo.New(),
	}
}

func (o *OuterCancel) Run(ctx context.Context) {
	defer func() {
		o.rcancelLock.Lock()
		defer o.rcancelLock.Unlock()

		for _, cancel := range o.rcancels {
			go cancel()
		}
	}()

	go func() {
		<-ctx.Done()
		close(o.closeCh)
	}()

	for {
		select {
		case <-o.closeCh:
			return
		case h := <-o.ch:
			o.handleHold(h)
		}
	}
}

func (o *OuterCancel) handleHold(h *hold) {
	if h.rctx != nil {
		select {
		case o.lock <- struct{}{}:
		case <-h.rctx.Done():
			h.respCh <- &holdresp{err: h.rctx.Err()}
			return
		}
	} else {
		o.lock <- struct{}{}
	}

	o.rcancelLock.Lock()

	if h.writeLock {
		for _, cancel := range o.rcancels {
			go cancel()
		}
		o.rcancelx = 0
		o.rcancelLock.Unlock()
		o.wg.Wait()

		h.respCh <- &holdresp{cancel: func() { <-o.lock }}

		return
	}

	o.wg.Add(1)
	var done bool
	doneCh := make(chan bool)
	rctx, cancel := context.WithCancelCause(h.rctx)
	i := o.rcancelx

	rcancel := func() {
		o.rcancelLock.Lock()
		if !done {
			close(doneCh)
			cancel(errors.New("placement is disseminating"))
			delete(o.rcancels, i)
			o.wg.Done()
			done = true
		}
		o.rcancelLock.Unlock()
	}

	rcancelGrace := func() {
		select {
		case <-time.After(2 * time.Second):
		case <-o.closeCh:
		case <-doneCh:
		}
		rcancel()
	}

	o.rcancels[i] = rcancelGrace
	o.rcancelx++

	o.rcancelLock.Unlock()

	h.respCh <- &holdresp{rctx: rctx, cancel: rcancel}

	<-o.lock
}

func (o *OuterCancel) Lock() context.CancelFunc {
	h := hold{
		writeLock: true,
		respCh:    make(chan *holdresp, 1),
	}

	select {
	case <-o.closeCh:
		o.shutdownLock.Lock()
		return o.shutdownLock.Unlock
	case o.ch <- &h:
	}

	select {
	case <-o.closeCh:
		o.shutdownLock.Lock()
		return o.shutdownLock.Unlock
	case resp := <-h.respCh:
		return resp.cancel
	}
}

func (o *OuterCancel) RLock(ctx context.Context) (context.Context, context.CancelFunc, error) {
	h := hold{
		writeLock: false,
		rctx:      ctx,
		respCh:    make(chan *holdresp, 1),
	}

	select {
	case <-o.closeCh:
		return nil, nil, errLockClosed
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case o.ch <- &h:
	}

	select {
	case <-o.closeCh:
		return nil, nil, errLockClosed
	case resp := <-h.respCh:
		return resp.rctx, resp.cancel, resp.err
	}
}

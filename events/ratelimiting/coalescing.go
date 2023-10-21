/*
Copyright 2023 The Dapr Authors
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

package ratelimiting

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/utils/clock"
)

// OptionsCoalescing configures a Coalescing RateLimiter.
type OptionsCoalescing struct {
	// InitialDelay is the initial delay for the rate limiter. The rate limiter
	// will not delay events less than the initial delay.
	// Defaults to 500ms.
	InitialDelay *time.Duration

	// MaxDelay is the maximum delay for the rate limiter. The rate limiter will
	// not delay events longer than the max delay.
	// Defaults to 5s.
	MaxDelay *time.Duration

	// MaxPendingEvents is the maximum number of events that can pending on a
	// rate limiter, before it fires an event anyway. Useful to prevent a rate
	// limiter never firing events in a high throughput scenario.
	// Defaults to unlimited.
	MaxPendingEvents *int
}

// coalescing is a rate limiter that rate limits events. It coalesces events
// that occur within a rate limiting window.
type coalescing struct {
	initialDelay     time.Duration
	maxDelay         time.Duration
	maxPendingEvents *int

	pendingEvents int
	timer         clock.Timer
	hasTimer      atomic.Bool
	inputCh       chan struct{}
	currentDur    time.Duration
	backoffFactor int

	wg      sync.WaitGroup
	lock    sync.RWMutex
	clock   clock.WithTicker
	running atomic.Bool
	closeCh chan struct{}
	closed  atomic.Bool
}

func NewCoalescing(opts OptionsCoalescing) (RateLimiter, error) {
	initialDelay := time.Millisecond * 500
	if opts.InitialDelay != nil {
		initialDelay = *opts.InitialDelay
	}
	if initialDelay < 0 {
		return nil, errors.New("initial delay must be > 0")
	}

	maxDelay := time.Second * 5
	if opts.MaxDelay != nil {
		maxDelay = *opts.MaxDelay
	}
	if maxDelay < 0 {
		return nil, errors.New("max delay must be > 0")
	}

	if maxDelay < initialDelay {
		return nil, errors.New("max delay must be >= base delay")
	}

	if opts.MaxPendingEvents != nil && *opts.MaxPendingEvents <= 0 {
		return nil, errors.New("max pending events must be > 0")
	}

	return &coalescing{
		initialDelay:     initialDelay,
		maxDelay:         maxDelay,
		maxPendingEvents: opts.MaxPendingEvents,
		currentDur:       initialDelay,
		backoffFactor:    1,
		inputCh:          make(chan struct{}),
		closeCh:          make(chan struct{}),
		clock:            clock.RealClock{},
	}, nil
}

// Run runs the rate limiter. It will begin rate limiting events after the
// first event is received.
func (c *coalescing) Run(ctx context.Context, ch chan<- struct{}) error {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("already running")
	}

	// Prevent wg race condition on Close and Run.
	c.lock.Lock()
	c.wg.Add(1)
	c.lock.Unlock()
	defer c.wg.Done()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		// If the timer doesn't exist yet, we're waiting for the first event (which
		// will fire immediately when received).
		if !c.hasTimer.Load() {
			select {
			case <-ctx.Done():
				return nil
			case <-c.closeCh:
				cancel()
				return nil

			case <-c.inputCh:
				c.handleInputCh(ctx, ch)
			}
		} else {
			// We already have a timer running, so we're waiting for either the timer
			// to fire, or for a new event to arrive.
			select {
			case <-ctx.Done():
				return nil
			case <-c.closeCh:
				cancel()
				return nil

			case <-c.inputCh:
				c.handleInputCh(ctx, ch)
			case <-c.timer.C():
				c.handleTimerFired(ctx, ch)
			}
		}
	}
}

func (c *coalescing) handleInputCh(ctx context.Context, ch chan<- struct{}) {
	c.lock.Lock()
	defer c.lock.Unlock()

	switch {
	case !c.hasTimer.Load():
		// We don't have a timer yet, so this is the first event that has fired. We
		// fire the event immediately, and set the timer to fire again after the
		// initial delay.
		c.timer = c.clock.NewTimer(c.initialDelay)
		c.hasTimer.Store(true)
		c.fireEvent(ctx, ch)

	default:
		// If maxPendingEvents is set and we have reached it then fire the event
		// immediately.
		if c.maxPendingEvents != nil && c.pendingEvents >= *c.maxPendingEvents {
			c.fireEvent(ctx, ch)
			return
		}

		if !c.timer.Stop() {
			<-c.timer.C()
		}

		// Setup backoff. Backoff is exponential. If initial is 500ms and max is
		// 5s, the backoff will follow:
		// 500ms, 1s, 2s, 4s, 5s, 5s, 5s, ...
		if c.currentDur < c.maxDelay {
			c.backoffFactor *= 2
			c.currentDur = time.Duration(float64(c.initialDelay) * float64(c.backoffFactor))
			if c.currentDur > c.maxDelay {
				c.currentDur = c.maxDelay
			}
		}

		c.timer.Reset(c.currentDur)
	}
}

func (c *coalescing) handleTimerFired(ctx context.Context, ch chan<- struct{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.fireEvent(ctx, ch)
	c.reset()
}

func (c *coalescing) fireEvent(ctx context.Context, ch chan<- struct{}) {
	// Important to only send on the channel if there are pending events,
	// otherwise we will double send an event, for example if only a single event
	// was sent and then the rate limiting window expired with no new events.
	if c.pendingEvents > 0 {
		c.pendingEvents = 0
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			select {
			case ch <- struct{}{}:
			case <-ctx.Done():
			}
		}()
	}
}

func (c *coalescing) reset() {
	if !c.timer.Stop() {
		select {
		case <-c.timer.C():
		default:
		}
	}

	c.pendingEvents = 0
	c.currentDur = c.initialDelay
	c.backoffFactor = 1
	c.hasTimer.Store(false)
	c.timer = nil
}

func (c *coalescing) Add() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.pendingEvents++
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		select {
		case c.inputCh <- struct{}{}:
		case <-c.closeCh:
		}
	}()
}

func (c *coalescing) Close() {
	defer func() {
		// Prevent wg race condition on Close and Run.
		c.lock.Lock()
		c.wg.Wait()
		c.lock.Unlock()
	}()
	if c.closed.CompareAndSwap(false, true) {
		close(c.closeCh)
	}
}

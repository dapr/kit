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

package queue

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/utils/clock"
)

// ErrProcessorStopped is returned when the processor is not running.
var ErrProcessorStopped = errors.New("processor is stopped")

// Processor manages the queue of items and processes them at the correct time.
type Processor[T queueable] struct {
	executeFn          func(r T)
	queue              queue[T]
	clock              clock.Clock
	lock               sync.Mutex
	wg                 sync.WaitGroup
	processorRunningCh chan struct{}
	stopCh             chan struct{}
	resetCh            chan struct{}
	stopped            atomic.Bool
}

// NewProcessor returns a new Processor object.
// executeFn is the callback invoked when the item is to be executed; this will be invoked in a background goroutine.
func NewProcessor[T queueable](executeFn func(r T)) *Processor[T] {
	return &Processor[T]{
		executeFn:          executeFn,
		queue:              newQueue[T](),
		processorRunningCh: make(chan struct{}, 1),
		stopCh:             make(chan struct{}),
		resetCh:            make(chan struct{}, 1),
		clock:              clock.RealClock{},
	}
}

// Enqueue adds a new item to the queue.
// If a item with the same ID already exists, it'll be replaced.
func (p *Processor[T]) Enqueue(r T) error {
	if p.stopped.Load() {
		return ErrProcessorStopped
	}

	// Insert or replace the item in the queue
	// If the item added or replaced is the first one in the queue, we need to know that
	p.lock.Lock()
	peek, ok := p.queue.Peek()
	isFirst := (ok && peek.Key() == r.Key()) // This is going to be true if the item being replaced is the first one in the queue
	p.queue.Insert(r, true)
	peek, _ = p.queue.Peek()         // No need to check for "ok" here because we know this will return an item
	isFirst = isFirst || (peek == r) // This is also going to be true if the item just added landed at the front of the queue
	p.process(isFirst)
	p.lock.Unlock()

	return nil
}

// Dequeue removes a item from the queue.
func (p *Processor[T]) Dequeue(key string) error {
	if p.stopped.Load() {
		return ErrProcessorStopped
	}

	// We need to check if this is the next item in the queue, as that requires stopping the processor
	p.lock.Lock()
	peek, ok := p.queue.Peek()
	p.queue.Remove(key)
	if ok && peek.Key() == key {
		// If the item was the first one in the queue, restart the processor
		p.process(true)
	}
	p.lock.Unlock()

	return nil
}

// Close stops the processor.
// This method blocks until the processor loop returns.
func (p *Processor[T]) Close() error {
	defer p.wg.Wait()
	if p.stopped.CompareAndSwap(false, true) {
		// Send a signal to stop
		close(p.stopCh)
		// Blocks until processor loop ends
		p.processorRunningCh <- struct{}{}
		return nil
	}

	return nil
}

// Start the processing loop if it's not already running.
// This must be invoked while the caller has a lock.
func (p *Processor[T]) process(isNext bool) {
	// Do not start a loop if it's already running
	select {
	case p.processorRunningCh <- struct{}{}:
		// Nop - fallthrough
	default:
		// Already running
		if isNext {
			// If this is the next item, send a reset signal
			// Use a select in case another goroutine is sending a reset signal too
			select {
			case p.resetCh <- struct{}{}:
			default:
			}
		}
		return
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.processLoop()
	}()
}

// Processing loop.
func (p *Processor[T]) processLoop() {
	defer func() {
		// Release the channel when exiting
		<-p.processorRunningCh
	}()

	var (
		r             T
		ok            bool
		t             clock.Timer
		scheduledTime time.Time
		deadline      time.Duration
	)

	for {
		// Continue processing items until the queue is empty
		p.lock.Lock()
		r, ok = p.queue.Peek()
		p.lock.Unlock()
		if !ok {
			return
		}

		// Check if after obtaining the lock we have a stop or reset signals
		// Do this before we create a timer
		select {
		case <-p.stopCh:
			// Exit on stop signals
			return
		case <-p.resetCh:
			// Restart the loop on reset signals
			continue
		default:
			// Nop, proceed
		}

		scheduledTime = r.ScheduledTime()
		deadline = scheduledTime.Sub(p.clock.Now())

		// If the deadline is less than 0.5ms away, execute it right away
		// This is more efficient than creating a timer
		if deadline < 500*time.Microsecond {
			p.execute(r)
			continue
		}

		t = p.clock.NewTimer(deadline)
		select {
		// Wait for when it's time to execute the item
		case <-t.C():
			p.execute(r)

		// If we get a reset signal, restart the loop
		case <-p.resetCh:
			// Restart the loop
			continue

		// If we receive a stop signal, exit
		case <-p.stopCh:
			// Stop the timer and exit the loop
			if !t.Stop() {
				<-t.C()
			}
			return
		}
	}
}

// Executes a item when it's time.
func (p *Processor[T]) execute(r T) {
	// Pop the item now that we're ready to process it
	// There's a small chance this is a different item than the one we peeked before
	p.lock.Lock()
	// For safety, let's peek at the first item before popping it and make sure it's the same object
	// It's unlikely, but if it's a different object then restart the loop
	peek, ok := p.queue.Peek()
	if !ok || peek != r {
		p.lock.Unlock()
		return
	}
	r, ok = p.queue.Pop()
	p.lock.Unlock()
	if !ok {
		return
	}

	go p.executeFn(r)
}

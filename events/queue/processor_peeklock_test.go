/*
Copyright 2026 The Dapr Authors
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
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"
)

// lockCheckItem counts ScheduledTime calls made while nobody holds the
// processor lock: those are the calls that can race a Dequeue.
type lockCheckItem struct {
	name     string
	at       time.Time
	p        *Processor[string, *lockCheckItem]
	unlocked *atomic.Int64
}

func (i *lockCheckItem) Key() string { return i.name }

func (i *lockCheckItem) ScheduledTime() time.Time {
	if i.p.lock.TryLock() {
		i.p.lock.Unlock()
		i.unlocked.Add(1)
	}

	return i.at
}

// The processor must read an item's ScheduledTime under its lock. Reading it
// after unlocking let a concurrent Dequeue hand the item back to its owner,
// which reset it, and the read dereferenced the reset state (a nil pointer
// SIGSEGV in the dapr scheduler under load).
func TestProcessor_ScheduledTimeReadUnderLock(t *testing.T) {
	const n = 200

	var (
		unlocked atomic.Int64
		executed = make(chan struct{}, n)
	)

	p := NewProcessor[string, *lockCheckItem](Options[string, *lockCheckItem]{
		ExecuteFn: func(*lockCheckItem) { executed <- struct{}{} },
	})

	t.Cleanup(func() { require.NoError(t, p.Close()) })

	now := time.Now()
	for i := range n {
		p.Enqueue(&lockCheckItem{
			name:     strconv.Itoa(i),
			at:       now.Add(time.Duration(i%5) * time.Millisecond),
			p:        p,
			unlocked: &unlocked,
		})
	}

	for range n {
		select {
		case <-executed:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for items to execute")
		}
	}

	assert.Zero(t, unlocked.Load(), "ScheduledTime was called without the processor lock held")
}

// hookClock runs onNow once, inside the first Now call. processLoop calls Now
// after releasing the lock and before execute, so the hook lands in that gap.
type hookClock struct {
	*clocktesting.FakeClock

	once  sync.Once
	onNow func()
}

func (c *hookClock) Now() time.Time {
	c.once.Do(c.onNow)
	return c.FakeClock.Now()
}

// An owner may Dequeue an item and Enqueue the same pointer with a later time.
// execute matched the front item by pointer only, so it fired the re-enqueued
// item at the time processLoop had read before the Dequeue.
func TestProcessor_ReenqueuedPointerDoesNotFireEarly(t *testing.T) {
	start := time.Now()
	fake := clocktesting.NewFakeClock(start)
	executed := make(chan *lockCheckItem, 1)

	clk := &hookClock{FakeClock: fake}
	p := NewProcessor[string, *lockCheckItem](Options[string, *lockCheckItem]{
		ExecuteFn: func(r *lockCheckItem) { executed <- r },
		Clock:     clk,
	})

	t.Cleanup(func() { require.NoError(t, p.Close()) })

	item := &lockCheckItem{name: "a", at: start, p: p, unlocked: new(atomic.Int64)}
	clk.onNow = func() {
		p.Dequeue(item.name)
		item.at = start.Add(time.Hour)
		p.Enqueue(item)
	}

	p.Enqueue(item)

	require.Eventually(t, func() bool {
		return fake.HasWaiters() || len(executed) > 0
	}, 5*time.Second, time.Millisecond)
	require.Empty(t, executed, "re-enqueued item fired at its old scheduled time")

	fake.Step(time.Hour)

	select {
	case r := <-executed:
		assert.Same(t, item, r)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the re-enqueued item to execute")
	}
}

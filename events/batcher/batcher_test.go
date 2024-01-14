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

package batcher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testingclock "k8s.io/utils/clock/testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	interval := time.Millisecond * 10
	b := New(interval)
	assert.Equal(t, interval, b.interval)
	assert.False(t, b.closed.Load())
}

func TestWithClock(t *testing.T) {
	b := New(time.Millisecond * 10)
	fakeClock := testingclock.NewFakeClock(time.Now())
	b.WithClock(fakeClock)
	assert.Equal(t, fakeClock, b.clock)
}

func TestSubscribe(t *testing.T) {
	t.Parallel()

	b := New(time.Millisecond * 10)
	ch := make(chan struct{})
	b.Subscribe(ch)
	assert.Len(t, b.eventChs, 1)
}

func TestBatch(t *testing.T) {
	t.Parallel()

	fakeClock := testingclock.NewFakeClock(time.Now())
	b := New(time.Millisecond * 10)
	b.WithClock(fakeClock)
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	ch3 := make(chan struct{})
	b.Subscribe(ch1, ch2)
	b.Subscribe(ch3)

	b.Batch("key1")
	b.Batch("key1")
	b.Batch("key1")
	b.Batch("key1")
	b.Batch("key2")
	b.Batch("key2")
	b.Batch("key3")
	b.Batch("key3")

	assert.Eventually(t, func() bool {
		return fakeClock.HasWaiters()
	}, time.Second*5, time.Millisecond*100)

	for _, ch := range []chan struct{}{ch1, ch2, ch3} {
		select {
		case <-ch:
			assert.Fail(t, "should not be triggered")
		default:
		}
	}

	fakeClock.Step(time.Millisecond * 5)

	for _, ch := range []chan struct{}{ch1, ch2, ch3} {
		select {
		case <-ch:
			assert.Fail(t, "should not be triggered")
		default:
		}
	}

	fakeClock.Step(time.Millisecond * 5)

	for i := 0; i < 3; i++ {
		for _, ch := range []chan struct{}{ch1, ch2, ch3} {
			select {
			case <-ch:
			case <-time.After(time.Second):
				assert.Fail(t, "should be triggered")
			}
		}
	}
}

func TestClose(t *testing.T) {
	t.Parallel()

	b := New(time.Millisecond * 10)
	ch := make(chan struct{})
	b.Subscribe(ch)
	assert.Len(t, b.eventChs, 1)
	b.Batch("key1")
	b.Close()
	assert.True(t, b.closed.Load())
}

func TestSubscribeAfterClose(t *testing.T) {
	t.Parallel()

	b := New(time.Millisecond * 10)
	b.Close()
	ch := make(chan struct{})
	b.Subscribe(ch)
	assert.Empty(t, b.eventChs)
}

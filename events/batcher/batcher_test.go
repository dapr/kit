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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testingclock "k8s.io/utils/clock/testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	interval := time.Millisecond * 10
	b := New[string, struct{}](interval)
	assert.Equal(t, interval, b.interval)
	assert.False(t, b.closed.Load())
}

func TestWithClock(t *testing.T) {
	b := New[string, struct{}](time.Millisecond * 10)
	fakeClock := testingclock.NewFakeClock(time.Now())
	b.WithClock(fakeClock)
	assert.Equal(t, fakeClock, b.clock)
}

func TestSubscribe(t *testing.T) {
	t.Parallel()

	b := New[string, struct{}](time.Millisecond * 10)
	ch := make(chan struct{})
	b.Subscribe(context.Background(), ch)
	assert.Len(t, b.eventChs, 1)
}

func TestBatch(t *testing.T) {
	t.Parallel()

	fakeClock := testingclock.NewFakeClock(time.Now())
	b := New[string, struct{}](time.Millisecond * 10)
	b.WithClock(fakeClock)
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	ch3 := make(chan struct{})
	b.Subscribe(context.Background(), ch1, ch2)
	b.Subscribe(context.Background(), ch3)

	b.Batch("key1", struct{}{})
	b.Batch("key1", struct{}{})
	b.Batch("key1", struct{}{})
	b.Batch("key1", struct{}{})
	b.Batch("key2", struct{}{})
	b.Batch("key2", struct{}{})
	b.Batch("key3", struct{}{})
	b.Batch("key3", struct{}{})

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

	t.Run("ensure items are received in order with latest value", func(t *testing.T) {
		b := New[int, int](0)
		t.Cleanup(b.Close)
		ch1 := make(chan int, 10)
		ch2 := make(chan int, 10)
		ch3 := make(chan int, 10)
		b.Subscribe(context.Background(), ch1, ch2)
		b.Subscribe(context.Background(), ch3)

		for i := 0; i < 10; i++ {
			b.Batch(i, i)
			b.Batch(i, i+1)
			b.Batch(i, i+2)
		}

		for _, ch := range []chan int{ch1} {
			for i := 0; i < 10; i++ {
				select {
				case v := <-ch:
					assert.Equal(t, i+2, v)
				case <-time.After(time.Second):
					assert.Fail(t, "should be triggered")
				}
			}
		}
	})
}

func TestClose(t *testing.T) {
	t.Parallel()

	b := New[string, struct{}](time.Millisecond * 10)
	ch := make(chan struct{})
	b.Subscribe(context.Background(), ch)
	assert.Len(t, b.eventChs, 1)
	b.Batch("key1", struct{}{})
	b.Close()
	assert.True(t, b.closed.Load())
}

func TestSubscribeAfterClose(t *testing.T) {
	t.Parallel()

	b := New[string, struct{}](time.Millisecond * 10)
	b.Close()
	ch := make(chan struct{})
	b.Subscribe(context.Background(), ch)
	assert.Empty(t, b.eventChs)
}

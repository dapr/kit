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
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"
)

func TestProcessor(t *testing.T) {
	// Create the processor
	clock := clocktesting.NewFakeClock(time.Now())
	executeCh := make(chan *queueableItem)
	processor := NewProcessor[string](func(r *queueableItem) {
		executeCh <- r
	})
	processor.clock = clock

	assertExecutedItem := func(t *testing.T) *queueableItem {
		t.Helper()

		select {
		case r := <-executeCh:
			return r
		case <-time.After(700 * time.Millisecond):
			t.Fatal("did not receive signal in 700ms")
		}

		return nil
	}

	assertNoExecutedItem := func(t *testing.T) {
		t.Helper()

		runtime.Gosched()
		select {
		case r := <-executeCh:
			t.Fatalf("received unexpected item: %s", r.Name)
		case <-time.After(500 * time.Millisecond):
			// all good
		}
	}

	t.Run("enqueue items", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			err := processor.Enqueue(
				newTestItem(i, clock.Now().Add(time.Second*time.Duration(i))),
			)
			require.NoError(t, err)
		}

		// Advance tickers by 500ms to start
		clock.Step(500 * time.Millisecond)

		// Advance tickers and assert messages are coming in order
		for i := 1; i <= 5; i++ {
			t.Logf("Waiting for signal %d", i)
			clock.Step(time.Second)
			received := assertExecutedItem(t)
			assert.Equal(t, strconv.Itoa(i), received.Name)
		}
	})

	t.Run("enqueue item to be executed right away", func(t *testing.T) {
		r := newTestItem(1, clock.Now())
		err := processor.Enqueue(r)
		require.NoError(t, err)

		clock.Step(500 * time.Millisecond)

		received := assertExecutedItem(t)
		assert.Equal(t, "1", received.Name)
	})

	t.Run("enqueue item at the front of the queue", func(t *testing.T) {
		// Enqueue 4 items
		for i := 1; i <= 4; i++ {
			err := processor.Enqueue(
				newTestItem(i, clock.Now().Add(time.Second*time.Duration(i))),
			)
			require.NoError(t, err)
		}

		assert.Eventually(t, clock.HasWaiters, time.Second, 100*time.Millisecond)

		// Advance tickers by 1s to trigger the first item
		t.Log("Waiting for signal 1")
		clock.Step(time.Second)

		received := assertExecutedItem(t)
		assert.Equal(t, "1", received.Name)

		// Add a new item at the front of the queue
		err := processor.Enqueue(
			newTestItem(99, clock.Now()),
		)
		require.NoError(t, err)

		// Advance tickers and assert messages are coming in order
		for i := 1; i <= 4; i++ {
			// First item should be 99
			expect := strconv.Itoa(i)
			if i == 1 {
				expect = "99"
			}
			t.Logf("Waiting for signal %s", expect)
			received := assertExecutedItem(t)
			assert.Equal(t, expect, received.Name)
			clock.Step(time.Second)
		}
	})

	t.Run("dequeue item", func(t *testing.T) {
		assert.Equal(t, 0, processor.queue.Len())
		require.False(t, clock.HasWaiters())

		// Enqueue 5 items
		for i := 1; i <= 5; i++ {
			err := processor.Enqueue(
				newTestItem(i, clock.Now().Add(time.Second*time.Duration(i))),
			)
			require.NoError(t, err)
		}
		assert.Equal(t, 5, processor.queue.Len())

		// Dequeue items 2 and 4
		// Note that this is a string because it's the key
		err := processor.Dequeue("2")
		require.NoError(t, err)
		err = processor.Dequeue("4")
		require.NoError(t, err)

		assert.Equal(t, 3, processor.queue.Len())

		// Advance tickers and assert messages are coming in order
		for i := 1; i <= 5; i++ {
			require.Eventually(t, clock.HasWaiters, time.Second, 100*time.Millisecond)
			clock.Step(time.Second)

			if i == 2 || i == 4 {
				// Skip items that have been removed
				t.Logf("Should not receive signal %d", i)
				assertNoExecutedItem(t)
				continue
			}

			t.Logf("Waiting for signal %d", i)
			received := assertExecutedItem(t)
			assert.Equal(t, strconv.Itoa(i), received.Name)
		}
	})

	t.Run("dequeue item from the front of the queue", func(t *testing.T) {
		// Enqueue 6 items
		for i := 1; i <= 6; i++ {
			err := processor.Enqueue(
				newTestItem(i, clock.Now().Add(time.Second*time.Duration(i))),
			)
			require.NoError(t, err)
		}

		// Advance tickers and assert messages are coming in order
		for i := 1; i <= 6; i++ {
			assert.Eventually(t, clock.HasWaiters, time.Second, 100*time.Millisecond)

			// On messages 2 and 5, dequeue the item when it's at the front of the queue
			if i == 2 || i == 5 {
				// Dequeue the item at the front of the queue
				// Note that this is a string because it's the key
				err := processor.Dequeue(strconv.Itoa(i))
				require.NoError(t, err)

				// Skip items that have been removed
				t.Logf("Should not receive signal %d", i)
				clock.Step(time.Second)
				assertNoExecutedItem(t)
				continue
			}
			t.Logf("Waiting for signal %d", i)
			clock.Step(time.Second)
			received := assertExecutedItem(t)
			assert.Equal(t, strconv.Itoa(i), received.Name)
		}
	})

	t.Run("replace item", func(t *testing.T) {
		// Enqueue 5 items
		for i := 1; i <= 5; i++ {
			err := processor.Enqueue(
				newTestItem(i, clock.Now().Add(time.Second*time.Duration(i))),
			)
			require.NoError(t, err)
		}

		// Replace item 4, bumping its priority down
		err := processor.Enqueue(newTestItem(4, clock.Now().Add(6*time.Second)))
		require.NoError(t, err)

		// Advance tickers and assert messages are coming in order
		for i := 1; i <= 6; i++ {
			clock.Step(time.Second)

			if i == 4 {
				// This item has been pushed down
				t.Logf("Should not receive signal %d now", i)
				assertNoExecutedItem(t)
				continue
			}

			expect := i
			if i == 6 {
				// Item 4 should come now
				expect = 4
			}
			t.Logf("Waiting for signal %d", expect)
			received := assertExecutedItem(t)
			assert.Equal(t, strconv.Itoa(expect), received.Name)
		}
	})

	t.Run("replace item at the front of the queue", func(t *testing.T) {
		// Enqueue 5 items
		for i := 1; i <= 5; i++ {
			err := processor.Enqueue(
				newTestItem(i, clock.Now().Add(time.Second*time.Duration(i))),
			)
			require.NoError(t, err)
		}

		// Advance tickers and assert messages are coming in order
		for i := 1; i <= 6; i++ {
			assert.Eventually(t, clock.HasWaiters, time.Second, 100*time.Millisecond)

			if i == 2 {
				// Replace item 2, bumping its priority down, while it's at the front of the queue
				err := processor.Enqueue(newTestItem(2, clock.Now().Add(5*time.Second)))
				require.NoError(t, err)

				// This item has been pushed down
				t.Logf("Should not receive signal %d now", i)
				clock.Step(time.Second)
				assertNoExecutedItem(t)
				continue
			}

			expect := i
			if i == 6 {
				// Item 2 should come now
				expect = 2
			}
			t.Logf("Waiting for signal %d", expect)
			clock.Sleep(time.Second)
			received := assertExecutedItem(t)
			assert.Equal(t, strconv.Itoa(expect), received.Name)
		}
	})

	t.Run("enqueue multiple items concurrently that to be executed randomly", func(t *testing.T) {
		const (
			count    = 150
			maxDelay = 30
		)
		now := clock.Now()
		wg := sync.WaitGroup{}
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				execTime := now.Add(time.Second * time.Duration(rand.Intn(maxDelay))) //nolint:gosec
				err := processor.Enqueue(newTestItem(i, execTime))
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		// Collect
		collected := make([]bool, count)
		var collectedCount int64
		doneCh := make(chan struct{})
		go func() {
			for {
				select {
				case <-doneCh:
					return
				case r := <-executeCh:
					n, err := strconv.Atoi(r.Name)
					if err == nil {
						collected[n] = true
						atomic.AddInt64(&collectedCount, 1)
					}
				}
			}
		}()

		// Advance tickers and assert messages are coming in order
		for i := 0; i <= maxDelay; i++ {
			clock.Step(time.Second)
		}

		// Allow for synchronization
		assert.Eventually(t, func() bool {
			return atomic.LoadInt64(&collectedCount) == count
		}, 5*time.Second, 50*time.Millisecond)
		close(doneCh)

		// Ensure all items are true
		for i := 0; i < count; i++ {
			assert.Truef(t, collected[i], "item %d not received", i)
		}
	})

	t.Run("stop processor", func(t *testing.T) {
		// Enqueue 5 items
		for i := 1; i <= 5; i++ {
			err := processor.Enqueue(
				newTestItem(i, clock.Now().Add(time.Second*time.Duration(i))),
			)
			require.NoError(t, err)
		}

		assert.Eventually(t, clock.HasWaiters, time.Second, 100*time.Millisecond)

		// Stop the processor
		require.NoError(t, processor.Close())

		// Queue should not be processed
		clock.Step(2 * time.Second)
		assertNoExecutedItem(t)

		// Enqueuing and dequeueing should fail
		err := processor.Enqueue(newTestItem(99, clock.Now()))
		require.ErrorIs(t, err, ErrProcessorStopped)
		err = processor.Dequeue("99")
		require.ErrorIs(t, err, ErrProcessorStopped)

		// Stopping again is a nop (should not crash)
		require.NoError(t, processor.Close())
	})
}

func TestClose(t *testing.T) {
	baseRoutines := runtime.NumGoroutine()

	// Create the processor
	clock := clocktesting.NewFakeClock(time.Now())
	executeCh := make(chan *queueableItem)
	processor := NewProcessor[string](func(r *queueableItem) {
		executeCh <- r
	})
	processor.clock = clock

	processor.Enqueue(newTestItem(1, clock.Now().Add(time.Second)))
	processor.Enqueue(newTestItem(2, clock.Now().Add(time.Second*2)))
	assert.Equal(t, 2, processor.queue.Len())

	assert.Eventually(t, clock.HasWaiters, time.Second, 10*time.Millisecond)

	assert.Eventually(t, func() bool {
		// processor and Eventually should be the only goroutines.
		return runtime.NumGoroutine() == baseRoutines+1+1
	}, time.Second, 100*time.Millisecond)

	clock.Step(time.Second)

	select {
	case <-executeCh:
	case <-time.After(time.Second * 3):
		t.Fatal("should receive item")
	}

	assert.Equal(t, 1, processor.queue.Len())

	closeCh := make(chan error)
	go func() {
		closeCh <- processor.Close()
	}()
	go func() {
		closeCh <- processor.Close()
	}()
	go func() {
		closeCh <- processor.Close()
	}()

	assert.Eventually(t, func() bool {
		// Eventually and the 3 above should be the only goroutine. The processor
		// goroutine should have exited.
		return runtime.NumGoroutine() == baseRoutines+1+3
	}, time.Second, 100*time.Millisecond)

	assert.False(t, clock.HasWaiters())

	select {
	case <-executeCh:
		t.Fatal("should not receive item")
	default:
	}

	for i := 0; i < 3; i++ {
		select {
		case err := <-closeCh:
			require.NoError(t, err)
		case <-time.After(time.Second * 3):
			t.Fatal("close should have returned")
		}
	}

	require.NoError(t, processor.Close())
}

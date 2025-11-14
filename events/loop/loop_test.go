/*
Copyright 2025 The Dapr Authors
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

package loop

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testHandler is a simple handler that records all values it sees.
type testHandler[T any] struct {
	mu    sync.Mutex
	seen  []T
	err   error
	delay time.Duration
}

func (h *testHandler[T]) Handle(ctx context.Context, v T) error {
	if h.delay > 0 {
		select {
		case <-time.After(h.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.seen = append(h.seen, v)
	return h.err
}

func (h *testHandler[T]) Values() []T {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]T, len(h.seen))
	copy(out, h.seen)
	return out
}

func TestLoop_EnqueueAndRunOrder_Unbounded(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &testHandler[int]{}
	const segmentSize = 4

	l := New[int](h, segmentSize)

	var wg sync.WaitGroup
	wg.Add(1)
	errCh := make(chan error, 1)
	t.Cleanup(func() {
		require.NoError(t, <-errCh)
	})
	go func() {
		defer wg.Done()
		errCh <- l.Run(ctx)
	}()

	// Enqueue more items than a single segment to force multiple channels.
	const n = 25
	for i := range n {
		l.Enqueue(i)
	}

	// Close with a sentinel value so we can verify it is the last element.
	const final = 999
	l.Close(final)

	wg.Wait()

	got := h.Values()
	require.Len(t, got, n+1, "handler should see all enqueued items plus final close item")

	// First n values should be 0..n-1 in order.
	for i := range n {
		assert.Equal(t, i, got[i], "item at index %d out of order", i)
	}

	// Last one is the final close value.
	assert.Equal(t, final, got[len(got)-1], "last item should be the Close() value")
}

func TestLoop_CloseTwiceIsSafe(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &testHandler[int]{}
	l := New[int](h, 2)

	var wg sync.WaitGroup
	wg.Add(1)
	errCh := make(chan error, 1)
	t.Cleanup(func() {
		require.NoError(t, <-errCh)
	})
	go func() {
		defer wg.Done()
		errCh <- l.Run(ctx)
	}()

	l.Enqueue(1)
	l.Close(2)

	// Second close should not panic or deadlock.
	done := make(chan struct{})
	go func() {
		l.Close(3)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "second Close should not block")
	}

	wg.Wait()

	got := h.Values()
	// First close enqueues 2, second close should be ignored.
	assert.Contains(t, got, 1)
	assert.Contains(t, got, 2)
	assert.NotContains(t, got, 3)
}

func TestLoop_Reset(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h1 := &testHandler[int]{}
	l := New[int](h1, 2)

	var wg1 sync.WaitGroup
	wg1.Add(1)
	errCh := make(chan error, 1)
	t.Cleanup(func() {
		require.NoError(t, <-errCh)
	})
	go func() {
		defer wg1.Done()
		errCh <- l.Run(ctx)
	}()

	l.Enqueue(1)
	l.Close(2)
	wg1.Wait()

	assert.ElementsMatch(t, []int{1, 2}, h1.Values())

	// Reset to a new handler and buffer size.
	h2 := &testHandler[int]{}
	l = l.Reset(h2, 8)

	require.NotNil(t, l)

	var wg2 sync.WaitGroup
	wg2.Add(1)
	errCh2 := make(chan error, 1)
	t.Cleanup(func() {
		require.NoError(t, <-errCh2)
	})
	go func() {
		defer wg2.Done()
		errCh2 <- l.Run(ctx)
	}()

	l.Enqueue(10)
	l.Enqueue(11)
	l.Close(12)
	wg2.Wait()

	assert.ElementsMatch(t, []int{10, 11, 12}, h2.Values())
}

func TestLoop_EnqueueAfterCloseIsDropped(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &testHandler[int]{}
	l := New[int](h, 2)

	var wg sync.WaitGroup
	wg.Add(1)
	errCh := make(chan error, 1)
	t.Cleanup(func() {
		require.NoError(t, <-errCh)
	})
	go func() {
		defer wg.Done()
		errCh <- l.Run(ctx)
	}()

	l.Enqueue(1)
	l.Close(2)

	// This enqueue should be ignored.
	l.Enqueue(3)

	wg.Wait()

	got := h.Values()
	assert.Equal(t, []int{1, 2}, got)
}

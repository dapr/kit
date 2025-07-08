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

package context

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Pool(t *testing.T) {
	var _ context.Context = &Pool{}

	t.Run("a pool with no context will always be done", func(t *testing.T) {
		t.Parallel()
		pool := NewPool()
		select {
		case <-pool.Done():
		case <-time.After(time.Second):
			t.Error("expected context pool to be cancelled")
		}
	})

	t.Run("a cancelled context given to pool, should have pool cancelled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		pool := NewPool(ctx)
		select {
		case <-pool.Done():
		case <-time.After(time.Second):
			t.Error("expected context pool to be cancelled")
		}
	})

	t.Run("a cancelled context given to pool, given a new context, should still have pool cancelled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		pool := NewPool(ctx)
		pool.Add(t.Context())
		select {
		case <-pool.Done():
		case <-time.After(time.Second):
			t.Error("expected context pool to be cancelled")
		}
	})

	t.Run("pool with multiple contexts should return once all contexts have been cancelled", func(t *testing.T) {
		t.Parallel()
		var ctx [50]context.Context
		var cancel [50]context.CancelFunc

		ctxPool := make([]context.Context, 0, 50)

		for i := range 50 {
			ctx[i], cancel[i] = context.WithCancel(t.Context())
			ctxPool = append(ctxPool, ctx[i])
		}
		pool := NewPool(ctxPool...)

		//nolint:gosec
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(ctx), func(i, j int) {
			ctx[i], ctx[j] = ctx[j], ctx[i]
			cancel[i], cancel[j] = cancel[j], cancel[i]
		})

		for i := range 50 {
			select {
			case <-pool.Done():
				t.Error("expected context to not be cancelled")
			case <-time.After(time.Millisecond):
			}
			cancel[i]()
		}

		select {
		case <-pool.Done():
		case <-time.After(time.Second):
			t.Error("expected context pool to be cancelled")
		}
	})

	t.Run("pool size will not increase if the given contexts have been cancelled", func(t *testing.T) {
		t.Parallel()

		ctx1, cancel1 := context.WithCancel(t.Context())
		ctx2, cancel2 := context.WithCancel(t.Context())
		pool := NewPool(ctx1, ctx2)
		assert.Equal(t, 2, pool.Size())

		cancel1()
		cancel2()
		select {
		case <-pool.Done():
		case <-time.After(time.Second):
			t.Error("expected context pool to be cancelled")
		}
		pool.Add(t.Context())
		assert.Equal(t, 2, pool.Size())
	})

	t.Run("pool size will not increase if the pool has been closed", func(t *testing.T) {
		t.Parallel()

		ctx1 := t.Context()
		ctx2 := t.Context()
		pool := NewPool(ctx1, ctx2)
		assert.Equal(t, 2, pool.Size())
		pool.Cancel()
		pool.Add(t.Context())
		assert.Equal(t, 0, pool.Size())
		select {
		case <-pool.Done():
		case <-time.After(time.Second):
			t.Error("expected context pool to be cancelled")
		}
	})

	t.Run("wait for added context to be closed", func(t *testing.T) {
		t.Parallel()

		ctx1, cancel1 := context.WithCancel(t.Context())
		pool := NewPool(ctx1)

		ctx2, cancel2 := context.WithCancel(t.Context())
		pool.Add(ctx2)

		assert.Equal(t, 2, pool.Size())
		cancel1()

		select {
		case <-pool.Done():
			t.Error("expected context pool to not be cancelled")
		case <-time.After(10 * time.Millisecond):
		}
		cancel2()
	})
}

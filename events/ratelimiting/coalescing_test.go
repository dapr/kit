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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/clock"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/dapr/kit/ptr"
)

func TestCoalescing(t *testing.T) {
	runCoalescingTests := func(t *testing.T, clock clock.WithTicker, opts OptionsCoalescing) (*coalescing, chan struct{}) {
		t.Helper()
		c, err := NewCoalescing(opts)
		require.NoError(t, err)

		if clock != nil {
			c.(RateLimiterWithTicker).WithTicker(clock)
		}

		ch := make(chan struct{})
		errCh := make(chan error)
		go func() {
			errCh <- c.Run(context.Background(), ch)
		}()

		t.Cleanup(func() {
			c.Close()

			select {
			case err := <-errCh:
				require.NoError(t, err)
			case <-time.After(time.Second):
				require.Fail(t, "timeout")
			}
		})

		return c.(*coalescing), ch
	}

	assertChannel := func(t *testing.T, ch chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(time.Second):
			require.Fail(t, "timeout")
		}
	}

	assertNoChannel := func(t *testing.T, ch chan struct{}) {
		t.Helper()
		select {
		case <-ch:
			require.Fail(t, "should not have received event")
		case <-time.After(time.Millisecond * 10):
		}
	}

	t.Run("closing context should return Run", func(t *testing.T) {
		c, err := NewCoalescing(OptionsCoalescing{})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error)
		go func() {
			errCh <- c.Run(ctx, make(chan struct{}))
		}()

		cancel()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "timeout")
		}
	})

	t.Run("calling Close should return Run", func(t *testing.T) {
		c, err := NewCoalescing(OptionsCoalescing{})
		require.NoError(t, err)

		errCh := make(chan error)
		go func() {
			errCh <- c.Run(context.Background(), make(chan struct{}))
		}()

		c.Close()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "timeout")
		}
	})

	t.Run("calling Run twice should error", func(t *testing.T) {
		c, err := NewCoalescing(OptionsCoalescing{})
		require.NoError(t, err)

		errCh := make(chan error)
		go func() {
			errCh <- c.Run(context.Background(), make(chan struct{}))
		}()

		c.Close()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "timeout")
		}

		go func() {
			errCh <- c.Run(context.Background(), make(chan struct{}))
		}()

		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "timeout")
		}
	})

	t.Run("options", func(t *testing.T) {
		_, err := NewCoalescing(OptionsCoalescing{
			InitialDelay: ptr.Of(-time.Second),
		})
		require.Error(t, err)

		_, err = NewCoalescing(OptionsCoalescing{
			MaxDelay: ptr.Of(-time.Second),
		})
		require.Error(t, err)

		_, err = NewCoalescing(OptionsCoalescing{
			MaxPendingEvents: ptr.Of(0),
		})
		require.Error(t, err)
		_, err = NewCoalescing(OptionsCoalescing{
			MaxPendingEvents: ptr.Of(-1),
		})
		require.Error(t, err)

		_, err = NewCoalescing(OptionsCoalescing{
			InitialDelay: ptr.Of(time.Second),
			MaxDelay:     ptr.Of(time.Second / 2),
		})
		require.Error(t, err)

		_, err = NewCoalescing(OptionsCoalescing{
			InitialDelay:     ptr.Of(time.Second),
			MaxDelay:         ptr.Of(time.Second * 2),
			MaxPendingEvents: ptr.Of(2),
		})
		require.NoError(t, err)
	})

	t.Run("sending a single event initially should immediately send it", func(t *testing.T) {
		c, ch := runCoalescingTests(t, nil, OptionsCoalescing{})
		c.Add()

		select {
		case <-ch:
		case <-time.After(time.Second):
			require.Fail(t, "timeout")
		}
	})

	t.Run("second event after initial delay should not be rate limited", func(t *testing.T) {
		clock := clocktesting.NewFakeClock(time.Now())
		c, ch := runCoalescingTests(t, clock, OptionsCoalescing{
			InitialDelay: ptr.Of(time.Second),
			MaxDelay:     ptr.Of(time.Second * 2),
		})

		c.Add()
		assertChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second)

		c.Add()
		assertChannel(t, ch)
		assertNoChannel(t, ch)
	})

	t.Run("second event before initial delay should be rate limited", func(t *testing.T) {
		clock := clocktesting.NewFakeClock(time.Now())
		c, ch := runCoalescingTests(t, clock, OptionsCoalescing{
			InitialDelay: ptr.Of(time.Second),
			MaxDelay:     ptr.Of(time.Second * 2),
		})

		c.Add()
		assertChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second / 2)

		c.Add()
		assertNoChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 2)
		assertChannel(t, ch)
		assertNoChannel(t, ch)
	})

	t.Run("multiple events before initial delay should be rate limited to single event", func(t *testing.T) {
		clock := clocktesting.NewFakeClock(time.Now())
		c, ch := runCoalescingTests(t, clock, OptionsCoalescing{
			InitialDelay: ptr.Of(time.Second),
			MaxDelay:     ptr.Of(time.Second * 2),
		})

		c.Add()
		assertChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second / 2)

		c.Add()
		c.Add()
		c.Add()
		assertNoChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 2)
		assertChannel(t, ch)
		assertNoChannel(t, ch)
	})

	t.Run("rate limiting should increase if events keep being added", func(t *testing.T) {
		clock := clocktesting.NewFakeClock(time.Now())
		c, ch := runCoalescingTests(t, clock, OptionsCoalescing{
			InitialDelay: ptr.Of(time.Second),
			MaxDelay:     ptr.Of(time.Second * 5),
		})

		c.Add()
		assertChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second / 2)
		c.Add()
		assertNoChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 1)
		c.Add()
		assertNoChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 2)
		c.Add()
		assertNoChannel(t, ch)

		for i := 0; i < 4; i++ {
			assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
			clock.Step(time.Second * 4)
			c.Add()
			assertNoChannel(t, ch)
		}

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 5)
		assertChannel(t, ch)
		assertNoChannel(t, ch)
	})

	t.Run("should fire event if reached maximum pending events", func(t *testing.T) {
		clock := clocktesting.NewFakeClock(time.Now())
		c, ch := runCoalescingTests(t, clock, OptionsCoalescing{
			InitialDelay:     ptr.Of(time.Second),
			MaxDelay:         ptr.Of(time.Second * 5),
			MaxPendingEvents: ptr.Of(3),
		})

		c.Add()
		assertChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second / 2)
		c.Add()
		assertNoChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 1)
		c.Add()
		assertNoChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 1)
		// We have reached 3 pending events so should fire event though we are rate
		// limited.
		c.Add()
		assertChannel(t, ch)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 2)
		c.Add()
		assertNoChannel(t, ch)

		// Expire rate limit and fire event.
		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 5)
		assertChannel(t, ch)
		assertNoChannel(t, ch)

		// New event should fire immediately.
		c.Add()
		assertChannel(t, ch)
	})

	t.Run("lots of events fired in the first rate limiting window will trigger 2 event omitted", func(t *testing.T) {
		clock := clocktesting.NewFakeClock(time.Now())
		c, ch := runCoalescingTests(t, clock, OptionsCoalescing{
			InitialDelay: ptr.Of(time.Second),
			MaxDelay:     ptr.Of(time.Second * 5),
		})

		c.Add()
		assertChannel(t, ch)

		assert.Eventually(t, c.hasTimer.Load, time.Second, time.Millisecond)
		for i := 0; i < 10; i++ {
			c.Add()
		}
		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 5)

		assertChannel(t, ch)
		assert.False(t, clock.HasWaiters())
		assertNoChannel(t, ch)
	})
}

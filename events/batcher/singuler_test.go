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

package batcher

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testingclock "k8s.io/utils/clock/testing"
)

func TestSingular(t *testing.T) {
	t.Parallel()

	fakeClock := testingclock.NewFakeClock(time.Now())
	s := NewSingular(time.Millisecond * 10)
	s.b.WithClock(fakeClock)

	fn := func(i *atomic.Int64) func() {
		return func() {
			i.Add(1)
		}
	}

	var f1, f2, f3 atomic.Int64
	s.Subscribe(fn(&f1))
	s.Subscribe(fn(&f2))
	s.Subscribe(fn(&f3))

	s.Batch()
	s.Batch()
	s.Batch()
	s.Batch()
	s.Batch()
	s.Batch()
	s.Batch()
	s.Batch()

	assert.Eventually(t, fakeClock.HasWaiters, time.Second*5, time.Millisecond*100)

	assert.Equal(t, int64(0), f1.Load())
	assert.Equal(t, int64(0), f2.Load())
	assert.Equal(t, int64(0), f3.Load())

	fakeClock.Step(time.Millisecond * 5)

	assert.Equal(t, int64(0), f1.Load())
	assert.Equal(t, int64(0), f2.Load())
	assert.Equal(t, int64(0), f3.Load())

	fakeClock.Step(time.Millisecond * 5)

	assert.Eventually(t, func() bool {
		return f1.Load() == 1 && f2.Load() == 1 && f3.Load() == 1
	}, time.Second*5, time.Millisecond*100)

	s.Batch()
	s.Batch()
	s.Batch()

	fakeClock.Step(time.Millisecond * 15)

	assert.Eventually(t, func() bool {
		return f1.Load() == 2 && f2.Load() == 2 && f3.Load() == 2
	}, time.Second*5, time.Millisecond*100)

	s.Close()

	s.Batch()
	s.Batch()
	s.Batch()

	fakeClock.Step(time.Millisecond * 10)

	assert.Eventually(t, func() bool {
		return !fakeClock.HasWaiters()
	}, time.Second*5, time.Millisecond*100)

	assert.Equal(t, int64(2), f1.Load())
	assert.Equal(t, int64(2), f2.Load())
	assert.Equal(t, int64(2), f3.Load())
}

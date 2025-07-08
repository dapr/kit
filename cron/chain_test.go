/*
Copyright 2022 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
This package has been forked from https://github.com/robfig/cron available under the MIT license.
You can check the original license at:
		https://github.com/robfig/cron/blob/master/LICENSE
*/

package cron

import (
	"io"
	"log"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clocktesting "k8s.io/utils/clock/testing"
)

func appendingJob(slice *[]int, value int) Job {
	var m sync.Mutex
	return FuncJob(func() {
		m.Lock()
		*slice = append(*slice, value)
		m.Unlock()
	})
}

func appendingWrapper(slice *[]int, value int) JobWrapper {
	return func(j Job) Job {
		return FuncJob(func() {
			appendingJob(slice, value).Run()
			j.Run()
		})
	}
}

func TestChain(t *testing.T) {
	var nums []int
	var (
		append1 = appendingWrapper(&nums, 1)
		append2 = appendingWrapper(&nums, 2)
		append3 = appendingWrapper(&nums, 3)
		append4 = appendingJob(&nums, 4)
	)
	NewChain(append1, append2, append3).Then(append4).Run()
	if !reflect.DeepEqual(nums, []int{1, 2, 3, 4}) {
		t.Error("unexpected order of calls:", nums)
	}
}

func TestChainRecover(t *testing.T) {
	panickingJob := FuncJob(func() {
		panic("panickingJob panics")
	})

	t.Run("panic exits job by default", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("panic expected, but none received")
			}
		}()
		NewChain().Then(panickingJob).
			Run()
	})

	t.Run("Recovering JobWrapper recovers", func(t *testing.T) {
		NewChain(Recover(PrintfLogger(log.New(io.Discard, "", 0)))).
			Then(panickingJob).
			Run()
	})

	t.Run("composed with the *IfStillRunning wrappers", func(t *testing.T) {
		NewChain(Recover(PrintfLogger(log.New(io.Discard, "", 0)))).
			Then(panickingJob).
			Run()
	})
}

type countJob struct {
	m       sync.Mutex
	started int
	clock   clocktesting.FakeClock
	done    int
	delay   time.Duration
}

func (j *countJob) Run() {
	j.m.Lock()
	j.started++
	j.m.Unlock()
	<-j.clock.After(j.delay)
	j.m.Lock()
	j.done++
	j.m.Unlock()
}

func (j *countJob) Started() int {
	defer j.m.Unlock()
	j.m.Lock()
	return j.started
}

func (j *countJob) Done() int {
	defer j.m.Unlock()
	j.m.Lock()
	return j.done
}

func TestChainDelayIfStillRunning(t *testing.T) {
	t.Run("runs immediately", func(t *testing.T) {
		var j countJob
		wrappedJob := NewChain(DelayIfStillRunning(DiscardLogger)).Then(&j)
		go wrappedJob.Run()
		assert.Eventually(t, j.clock.HasWaiters, 100*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(1)
		assert.Eventually(t, func() bool {
			return j.Done() == 1
		}, 100*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("second run immediate if first done", func(t *testing.T) {
		var j countJob
		wrappedJob := NewChain(DelayIfStillRunning(DiscardLogger)).Then(&j)
		go func() {
			go wrappedJob.Run()
			go wrappedJob.Run()
		}()
		assert.Eventually(t, j.clock.HasWaiters, 100*time.Millisecond, 10*time.Millisecond)
		assert.Eventually(t, func() bool {
			j.clock.Step(1)
			return j.Done() == 2
		}, 100*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("second run delayed if first not done", func(t *testing.T) {
		var j countJob
		j.delay = 100 * time.Millisecond
		wrappedJob := NewChain(DelayIfStillRunning(DiscardLogger)).Then(&j)
		go func() {
			go wrappedJob.Run()
			go wrappedJob.Run()
		}()

		// After 50 ms, the first job is still in progress, and the second job was
		// run but should be waiting for it to finish.
		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(50 * time.Millisecond)
		started, done := j.Started(), j.Done()
		if started != 1 || done != 0 {
			t.Error("expected first job started, but not finished, got", started, done)
		}

		// Verify that the second job completes.
		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(50 * time.Millisecond)
		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(200 * time.Millisecond)
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			started, done = j.Started(), j.Done()
			if started != 2 || done != 2 {
				c.Errorf("expected both jobs done, got %v %v", started, done)
			}
		}, 100*time.Millisecond, 10*time.Millisecond)
	})
}

func TestChainSkipIfStillRunning(t *testing.T) {
	t.Run("runs immediately", func(t *testing.T) {
		var j countJob
		wrappedJob := NewChain(SkipIfStillRunning(DiscardLogger)).Then(&j)
		go wrappedJob.Run()
		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(1)
		assert.Eventually(t, func() bool { return j.Done() == 1 }, 50*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("second run immediate if first done", func(t *testing.T) {
		var j countJob
		wrappedJob := NewChain(SkipIfStillRunning(DiscardLogger)).Then(&j)
		go wrappedJob.Run()

		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(1)
		assert.Eventually(t, func() bool { return j.Done() == 1 }, 100*time.Millisecond, 10*time.Millisecond)

		go wrappedJob.Run()

		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(1)
		assert.Eventually(t, func() bool { return j.Done() == 2 }, 100*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("second run skipped if first not done", func(t *testing.T) {
		var j countJob
		j.delay = 100 * time.Millisecond
		wrappedJob := NewChain(SkipIfStillRunning(DiscardLogger)).Then(&j)
		go func() {
			go wrappedJob.Run()
			go wrappedJob.Run()
		}()

		// After 50ms, the first job is still in progress, and the second job was
		// aleady skipped.
		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(50 * time.Millisecond)
		assert.Eventually(t, func() bool {
			return j.Started() == 1 && j.Done() == 0
		}, 50*time.Millisecond, 10*time.Millisecond)

		// Verify that the first job completes and second does not run.
		j.clock.Step(200 * time.Millisecond)
		assert.Eventually(t, func() bool {
			return j.Started() == 1 && j.Done() == 1
		}, 50*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("skip 10 jobs on rapid fire", func(t *testing.T) {
		var j countJob
		j.delay = 10 * time.Millisecond
		wrappedJob := NewChain(SkipIfStillRunning(DiscardLogger)).Then(&j)
		for range 11 {
			go wrappedJob.Run()
		}
		assert.Eventually(t, j.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j.clock.Step(200 * time.Millisecond)
		assert.False(t, j.clock.HasWaiters())
		assert.Eventually(t, func() bool {
			return j.Started() == 1 && j.Done() == 1
		}, 50*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("different jobs independent", func(t *testing.T) {
		var j1, j2 countJob
		j1.delay = 10 * time.Millisecond
		j2.delay = 10 * time.Millisecond
		chain := NewChain(SkipIfStillRunning(DiscardLogger))
		wrappedJob1 := chain.Then(&j1)
		wrappedJob2 := chain.Then(&j2)
		for range 11 {
			go wrappedJob1.Run()
			go wrappedJob2.Run()
		}
		assert.Eventually(t, j1.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		assert.Eventually(t, j2.clock.HasWaiters, 50*time.Millisecond, 10*time.Millisecond)
		j1.clock.Step(10 * time.Millisecond)
		j2.clock.Step(10 * time.Millisecond)
		assert.Eventually(t, func() bool {
			return j1.Started() == 1 && j1.Done() == 1
		}, 50*time.Millisecond, 10*time.Millisecond)
	})
}

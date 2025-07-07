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
	"bytes"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"
)

// Many tests schedule a job for every second, and then wait at most a second
// for it to run.  This amount is just slightly larger than 1 second to
// compensate for a few milliseconds of runtime.
//

const OneSecond = 1*time.Second + 50*time.Millisecond

type syncWriter struct {
	wr bytes.Buffer
	m  sync.Mutex
}

//nolint:nonamedreturns
func (sw *syncWriter) Write(data []byte) (n int, err error) {
	sw.m.Lock()
	n, err = sw.wr.Write(data)
	sw.m.Unlock()
	return
}

func (sw *syncWriter) String() string {
	sw.m.Lock()
	defer sw.m.Unlock()
	return sw.wr.String()
}

func newBufLogger(sw *syncWriter) Logger {
	return PrintfLogger(log.New(sw, "", log.LstdFlags))
}

func TestFuncPanicRecovery(t *testing.T) {
	clock := clocktesting.NewFakeClock(time.Now())
	var buf syncWriter
	cron := New(WithParser(secondParser),
		WithChain(Recover(newBufLogger(&buf))),
		WithClock(clock),
	)
	cron.Start()
	defer cron.Stop()
	cron.AddFunc("* * * * * ?", func() {
		panic("YOLO")
	})

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Contains(c, buf.String(), "YOLO")
	}, OneSecond, 10*time.Millisecond)
}

type DummyJob struct{}

func (d DummyJob) Run() {
	panic("YOLO")
}

func TestJobPanicRecovery(t *testing.T) {
	var job DummyJob
	var buf syncWriter

	clock := clocktesting.NewFakeClock(time.Now())
	cron := New(WithParser(secondParser),
		WithChain(Recover(newBufLogger(&buf))),
		WithClock(clock),
	)
	cron.Start()
	defer cron.Stop()
	cron.AddJob("* * * * * ?", job)

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Contains(c, buf.String(), "YOLO")
	}, OneSecond, 10*time.Millisecond)
}

// Start and stop cron with no entries.
func TestNoEntries(t *testing.T) {
	cron, _ := newWithSeconds()
	cron.Start()

	select {
	case <-time.After(OneSecond):
		t.Fatal("expected cron will be stopped immediately")
	case <-stop(cron):
	}
}

// Start, stop, then add an entry. Verify entry doesn't run.
func TestStopCausesJobsToNotRun(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron, clock := newWithSeconds()
	cron.Start()
	cron.Stop()
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	select {
	case <-time.After(time.Millisecond * 100):
		assert.False(t, clock.HasWaiters())
		// No job ran!
	case <-wait(wg):
		t.Fatal("expected stopped cron does not run any job")
	}
}

// Add a job, start cron, expect it runs.
func TestAddBeforeRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron, clock := newWithSeconds()
	cron.AddFunc("* * * * * ?", func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	// Give cron 2 seconds to run our job (which is always activated).
	select {
	case <-time.After(OneSecond):
		t.Fatal("expected job runs")
	case <-wait(wg):
	}
}

// // Start cron, add a job, expect it runs.
func TestAddWhileRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron, clock := newWithSeconds()
	cron.Start()
	defer cron.Stop()
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(OneSecond):
		t.Fatal("expected job runs")
	case <-wait(wg):
	}
}

// // Test for #34. Adding a job after calling start results in multiple job invocations
func TestAddWhileRunningWithDelay(t *testing.T) {
	cron, clock := newWithSeconds()
	cron.Start()
	defer cron.Stop()
	clock.Step(OneSecond * 5)
	var calls int64
	cron.AddFunc("* * * * * *", func() { atomic.AddInt64(&calls, 1) })

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	assert.Eventually(t, func() bool {
		return atomic.LoadInt64(&calls) == 1
	}, OneSecond, 10*time.Millisecond)
}

// Add a job, remove a job, start cron, expect nothing runs.
func TestRemoveBeforeRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron, clock := newWithSeconds()
	id, _ := cron.AddFunc("* * * * * ?", func() { wg.Done() })
	cron.Remove(id)
	cron.Start()
	defer cron.Stop()

	clock.Step(OneSecond)

	select {
	case <-time.After(time.Millisecond * 100):
		// Success, shouldn't run
		assert.False(t, clock.HasWaiters())
	case <-wait(wg):
		t.FailNow()
	}
}

// // Start cron, add a job, remove it, expect it doesn't run.
func TestRemoveWhileRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron, clock := newWithSeconds()
	cron.Start()
	defer cron.Stop()

	id, err := cron.AddFunc("* * * * * ?", func() { wg.Done() })
	require.NoError(t, err)
	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)

	cron.Remove(id)
	assert.Eventually(t, func() bool {
		return !clock.HasWaiters()
	}, OneSecond, 10*time.Millisecond)

	select {
	case <-time.After(time.Millisecond * 100):
	case <-wait(wg):
		t.FailNow()
	}
}

// // Test timing with Entries.
func TestSnapshotEntries(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	clock := clocktesting.NewFakeClock(time.Now())
	cron := New(WithClock(clock))
	cron.AddFunc("@every 2s", func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	// Cron should fire in 2 seconds. After 1 second, call Entries.
	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	cron.Entries()

	// Even though Entries was called, the cron should fire at the 2 second mark.
	select {
	case <-time.After(time.Millisecond * 100):
	case <-wait(wg):
	}

	clock.Step(OneSecond)

	select {
	case <-time.After(OneSecond):
		t.Error("expected job runs at 2 second mark")
	case <-wait(wg):
	}
}

// Test that the entries are correctly sorted.
// Add a bunch of long-in-the-future entries, and an immediate entry, and ensure
// that the immediate entry runs immediately.
// Also: Test that multiple jobs run in the same instant.
func TestMultipleEntries(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron, clock := newWithSeconds()
	cron.AddFunc("0 0 0 1 1 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })
	id1, _ := cron.AddFunc("* * * * * ?", func() { t.Fatal() })
	id2, _ := cron.AddFunc("* * * * * ?", func() { t.Fatal() })
	cron.AddFunc("0 0 0 31 12 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	cron.Remove(id1)
	cron.Start()
	cron.Remove(id2)
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(OneSecond):
		t.Error("expected job run in proper order")
	case <-wait(wg):
	}
}

// // Test running the same job twice.
func TestRunningJobTwice(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron, clock := newWithSeconds()
	cron.AddFunc("0 0 0 1 1 ?", func() {})
	cron.AddFunc("0 0 0 31 12 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(2 * OneSecond):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

func TestRunningMultipleSchedules(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron, clock := newWithSeconds()
	cron.AddFunc("0 0 0 1 1 ?", func() {})
	cron.AddFunc("0 0 0 31 12 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })
	cron.Schedule(Every(time.Minute), FuncJob(func() {}))
	cron.Schedule(Every(time.Second), FuncJob(func() { wg.Done() }))
	cron.Schedule(Every(time.Hour), FuncJob(func() {}))

	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(2 * OneSecond):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

// Test that the cron is run in the local time zone (as opposed to UTC).
func TestLocalTimezone(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	now := time.Date(2016, 11, 8, 12, 0, 0, 0, time.Local)
	spec := fmt.Sprintf("%d,%d %d %d %d %d ?",
		now.Second()+1, now.Second()+2, now.Minute(), now.Hour(), now.Day(), now.Month())

	cron, clock := newWithSeconds()
	clock.SetTime(now)
	cron.AddFunc(spec, func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(OneSecond * 2):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

// Test that the cron is run in the given time zone (as opposed to local).
func TestNonLocalTimezone(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	loc, err := time.LoadLocation("Atlantic/Cape_Verde")
	require.NoError(t, err)

	if loc == time.Local {
		loc, err = time.LoadLocation("America/New_York")
		require.NoError(t, err)
	}

	now := time.Date(2016, 11, 8, 12, 0, 0, 0, loc)
	spec := fmt.Sprintf("%d,%d %d %d %d %d ?",
		now.Second()+1, now.Second()+2, now.Minute(), now.Hour(), now.Day(), now.Month())

	clock := clocktesting.NewFakeClock(now)
	cron := New(WithLocation(loc), WithParser(secondParser), WithClock(clock))
	cron.AddFunc(spec, func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(OneSecond * 2):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

// Test that calling stop before start silently returns without
// blocking the stop channel.
func TestStopWithoutStart(*testing.T) {
	cron := New()
	cron.Stop()
}

type testJob struct {
	wg   *sync.WaitGroup
	name string
}

func (t testJob) Run() {
	t.wg.Done()
}

// Test that adding an invalid job spec returns an error
func TestInvalidJobSpec(t *testing.T) {
	cron := New()
	_, err := cron.AddJob("this will not parse", nil)
	if err == nil {
		t.Errorf("expected an error with invalid spec, got nil")
	}
}

// Test blocking run method behaves as Start()
func TestBlockingRun(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron, clock := newWithSeconds()
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	unblockChan := make(chan struct{})

	go func() {
		cron.Run()
		close(unblockChan)
	}()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(OneSecond):
		t.Error("expected job fires")
	case <-unblockChan:
		t.Error("expected that Run() blocks")
	case <-wait(wg):
	}
}

// Test that double-running is a no-op
func TestStartNoop(t *testing.T) {
	tickChan := make(chan struct{}, 2)

	cron, clock := newWithSeconds()
	cron.AddFunc("* * * * * ?", func() {
		tickChan <- struct{}{}
	})

	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	// Wait for the first firing to ensure the runner is going
	<-tickChan

	cron.Start()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	<-tickChan

	// Fail if this job fires again in a short period, indicating a double-run
	select {
	case <-time.After(time.Millisecond):
	case <-tickChan:
		t.Error("expected job fires exactly twice")
	}
}

// Simple test using Runnables.
func TestJob(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron, clock := newWithSeconds()
	cron.AddJob("0 0 0 30 Feb ?", testJob{wg, "job0"})
	cron.AddJob("0 0 0 1 1 ?", testJob{wg, "job1"})
	job2, _ := cron.AddJob("* * * * * ?", testJob{wg, "job2"})
	cron.AddJob("1 0 0 1 1 ?", testJob{wg, "job3"})
	cron.Schedule(Every(5*time.Second+5*time.Nanosecond), testJob{wg, "job4"})
	job5 := cron.Schedule(Every(5*time.Minute), testJob{wg, "job5"})

	// Test getting an Entry pre-Start.
	if actualName := cron.Entry(job2).Job.(testJob).name; actualName != "job2" {
		t.Error("wrong job retrieved:", actualName)
	}
	if actualName := cron.Entry(job5).Job.(testJob).name; actualName != "job5" {
		t.Error("wrong job retrieved:", actualName)
	}

	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(OneSecond):
		t.FailNow()
	case <-wait(wg):
	}

	// Ensure the entries are in the right order.
	expecteds := []string{"job2", "job4", "job5", "job1", "job3", "job0"}

	cronEntries := cron.Entries()
	actuals := make([]string, len(cronEntries))
	for i, entry := range cronEntries {
		actuals[i] = entry.Job.(testJob).name
	}

	for i, expected := range expecteds {
		if actuals[i] != expected {
			t.Fatalf("Jobs not in the right order.  (expected) %s != %s (actual)", expecteds, actuals)
		}
	}

	// Test getting Entries.
	if actualName := cron.Entry(job2).Job.(testJob).name; actualName != "job2" {
		t.Error("wrong job retrieved:", actualName)
	}
	if actualName := cron.Entry(job5).Job.(testJob).name; actualName != "job5" {
		t.Error("wrong job retrieved:", actualName)
	}
}

// Issue #206
// Ensure that the next run of a job after removing an entry is accurate.
func TestScheduleAfterRemoval(t *testing.T) {
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup
	wg1.Add(1)
	wg2.Add(1)

	// The first time this job is run, set a timer and remove the other job
	// 750ms later. Correct behavior would be to still run the job again in
	// 250ms, but the bug would cause it to run instead 1s later.

	var calls int
	var mu sync.Mutex

	cron, clock := newWithSeconds()
	hourJob := cron.Schedule(Every(time.Hour), FuncJob(func() {}))
	cron.Schedule(Every(time.Second), FuncJob(func() {
		mu.Lock()
		defer mu.Unlock()
		switch calls {
		case 0:
			wg1.Done()
			calls++
		case 1:
			<-clock.After(100 * time.Millisecond)
			cron.Remove(hourJob)
			calls++
		case 2:
			calls++
			wg2.Done()
		case 3:
			panic("unexpected 3rd call")
		}
	}))

	cron.Start()
	defer cron.Stop()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	// the first run might be any length of time 0 - 1s, since the schedule
	// rounds to the second. wait for the first run to true up.
	wg1.Wait()

	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)

	select {
	case <-time.After(2 * OneSecond):
		t.Error("expected job fires 2 times")
	case <-wait(&wg2):
	}
}

type ZeroSchedule struct{}

func (*ZeroSchedule) Next(time.Time) time.Time {
	return time.Time{}
}

// Tests that job without time does not run
func TestJobWithZeroTimeDoesNotRun(t *testing.T) {
	cron, clock := newWithSeconds()
	var calls int64
	cron.AddFunc("* * * * * *", func() { atomic.AddInt64(&calls, 1) })
	cron.Schedule(new(ZeroSchedule), FuncJob(func() { t.Error("expected zero task will not run") }))
	cron.Start()
	defer cron.Stop()
	assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
	clock.Step(OneSecond)
	assert.Eventually(t, func() bool {
		return atomic.LoadInt64(&calls) == 1
	}, OneSecond, 10*time.Millisecond)
}

func TestStopAndWait(t *testing.T) {
	t.Run("nothing running, returns immediately", func(t *testing.T) {
		cron, _ := newWithSeconds()
		cron.Start()
		ctx := cron.Stop()
		select {
		case <-ctx.Done():
		case <-time.After(time.Millisecond):
			t.Error("context was not done immediately")
		}
	})

	t.Run("repeated calls to Stop", func(t *testing.T) {
		cron, _ := newWithSeconds()
		cron.Start()
		_ = cron.Stop()
		ctx := cron.Stop()
		select {
		case <-ctx.Done():
		case <-time.After(time.Millisecond):
			t.Error("context was not done immediately")
		}
	})

	t.Run("a couple fast jobs added, still returns immediately", func(t *testing.T) {
		cron, clock := newWithSeconds()
		cron.AddFunc("* * * * * *", func() {})
		cron.Start()
		cron.AddFunc("* * * * * *", func() {})
		cron.AddFunc("* * * * * *", func() {})
		cron.AddFunc("* * * * * *", func() {})
		assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
		clock.Step(OneSecond)
		ctx := cron.Stop()
		select {
		case <-ctx.Done():
		case <-time.After(time.Millisecond):
			t.Error("context was not done immediately")
		}
	})

	t.Run("a couple fast jobs and a slow job added, waits for slow job", func(t *testing.T) {
		funcClock := clocktesting.NewFakeClock(time.Now())
		cron, clock := newWithSeconds()
		cron.AddFunc("* * * * * *", func() {})
		cron.Start()
		cron.AddFunc("* * * * * *", func() { <-funcClock.After(OneSecond * 2) })
		cron.AddFunc("* * * * * *", func() {})

		assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
		assert.False(t, funcClock.HasWaiters())
		clock.Step(OneSecond)
		assert.Eventually(t, funcClock.HasWaiters, OneSecond, 10*time.Millisecond)
		funcClock.Step(OneSecond)

		ctx := cron.Stop()

		// Verify that it is not done.
		select {
		case <-ctx.Done():
			t.Error("context was done too quickly immediately")
		case <-time.After(10 * time.Millisecond):
			// expected, because the job sleeping for 1 second is still running
		}

		assert.False(t, clock.HasWaiters())
		funcClock.Step(OneSecond)

		// Verify that it IS done in the next 500ms (giving 250ms buffer)
		select {
		case <-ctx.Done():
			// expected
		case <-time.After(1500 * time.Millisecond):
			t.Error("context not done after job should have completed")
		}
	})

	t.Run("repeated calls to stop, waiting for completion and after", func(t *testing.T) {
		cron, clock := newWithSeconds()
		funcClock := clocktesting.NewFakeClock(clock.Now())
		cron.AddFunc("* * * * * *", func() {})
		cron.AddFunc("* * * * * *", func() { <-funcClock.After(OneSecond * 2) })
		cron.Start()
		cron.AddFunc("* * * * * *", func() {})

		assert.Eventually(t, clock.HasWaiters, OneSecond, 10*time.Millisecond)
		assert.False(t, funcClock.HasWaiters())
		clock.Step(OneSecond)
		assert.Eventually(t, funcClock.HasWaiters, OneSecond, 10*time.Millisecond)
		funcClock.Step(time.Millisecond * 1500)

		ctx := cron.Stop()
		ctx2 := cron.Stop()

		// Verify that it is not done for at least 1500ms
		select {
		case <-ctx.Done():
			t.Error("context was done too quickly immediately")
		case <-ctx2.Done():
			t.Error("context2 was done too quickly immediately")
		case <-time.After(100 * time.Millisecond):
			// expected, because the job sleeping for 2 seconds is still running
		}

		assert.False(t, clock.HasWaiters())
		assert.True(t, funcClock.HasWaiters())
		funcClock.Step(time.Millisecond * 600)

		// Verify that it IS done in the next 1s (giving 500ms buffer)
		select {
		case <-ctx.Done():
			// expected
		case <-time.After(time.Second):
			t.Error("context not done after job should have completed")
		}

		// Verify that ctx2 is also done.
		select {
		case <-ctx2.Done():
			// expected
		case <-time.After(time.Millisecond):
			t.Error("context2 not done even though context1 is")
		}

		// Verify that a new context retrieved from stop is immediately done.
		ctx3 := cron.Stop()
		select {
		case <-ctx3.Done():
			// expected
		case <-time.After(time.Millisecond):
			t.Error("context not done even when cron Stop is completed")
		}
	})
}

func TestMockClock(t *testing.T) {
	clk := clocktesting.NewFakeClock(time.Now())
	cron := New(WithClock(clk))
	counter := atomic.Int64{}
	cron.AddFunc("@every 1s", func() {
		counter.Add(1)
	})
	cron.Start()
	defer cron.Stop()
	for range 11 {
		assert.Eventually(t, clk.HasWaiters, OneSecond, 10*time.Millisecond)
		clk.Step(1 * time.Second)
	}
	assert.Equal(t, int64(10), counter.Load())
}

func TestMillisecond(t *testing.T) {
	clk := clocktesting.NewFakeClock(time.Now())
	cron := New(WithClock(clk))
	counter1ms := atomic.Int64{}
	counter15ms := atomic.Int64{}
	counter100ms := atomic.Int64{}
	cron.AddFunc("@every 1ms", func() {
		counter1ms.Add(1)
	})
	cron.AddFunc("@every 15ms", func() {
		counter15ms.Add(1)
	})
	cron.AddFunc("@every 100ms", func() {
		counter100ms.Add(1)
	})

	cron.Start()
	defer cron.Stop()
	for range 1000 {
		assert.Eventually(t, clk.HasWaiters, OneSecond, 1*time.Millisecond)
		clk.Step(1 * time.Millisecond)
	}
	assert.Equal(t, int64(999), counter1ms.Load())
	assert.Equal(t, int64(66), counter15ms.Load())
	assert.Equal(t, int64(9), counter100ms.Load())
}

func TestMultiThreadedStartAndStop(*testing.T) {
	cron := New()
	go cron.Run()
	cron.Stop()
}

func wait(wg *sync.WaitGroup) chan bool {
	ch := make(chan bool)
	go func() {
		wg.Wait()
		ch <- true
	}()
	return ch
}

func stop(cron *Cron) chan bool {
	ch := make(chan bool)
	go func() {
		cron.Stop()
		ch <- true
	}()
	return ch
}

// newWithSeconds returns a Cron with the seconds field enabled.
func newWithSeconds() (*Cron, *clocktesting.FakeClock) {
	clock := clocktesting.NewFakeClock(time.Now())
	return New(WithParser(secondParser), WithChain(), WithClock(clock)), clock
}

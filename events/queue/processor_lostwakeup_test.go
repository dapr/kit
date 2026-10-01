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
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Concurrent Enqueues racing the loop exiting on an empty queue must never
// leave items in the queue with no loop running.
// Set PROCESSOR_LOSTWAKEUP_DURATION (ex: "30s") for a longer soak.
func TestProcessor_ConcurrentEnqueueNoLostWakeup(t *testing.T) {
	duration := 5 * time.Second

	v := os.Getenv("PROCESSOR_LOSTWAKEUP_DURATION")
	if v != "" {
		d, err := time.ParseDuration(v)
		require.NoError(t, err, "PROCESSOR_LOSTWAKEUP_DURATION")

		duration = d
	}

	const producers = 8

	fired := make(chan string, 4096)
	processor := NewProcessor[string, *queueableItem](Options[string, *queueableItem]{
		ExecuteFn: func(r *queueableItem) { fired <- r.Name },
	})

	t.Cleanup(func() { require.NoError(t, processor.Close()) })

	// Producers enqueue one item each when a wave's release channel closes.
	stop := make(chan struct{})

	var wg sync.WaitGroup

	type wave struct {
		release chan struct{}
		names   []string
	}

	waveCh := make(chan *wave)

	for p := range producers {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				case w := <-waveCh:
					<-w.release
					processor.Enqueue(&queueableItem{Name: w.names[p], ExecutionTime: time.Now()})
				}
			}
		})
	}

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})

	// collect drains fired until expected is empty or the deadline passes.
	collect := func(expected map[string]bool, deadline time.Duration) {
		timer := time.NewTimer(deadline)
		defer timer.Stop()

		for len(expected) > 0 {
			select {
			case name := <-fired:
				delete(expected, name)
			case <-timer.C:
				return
			}
		}
	}

	// queued items with no loop running can never execute.
	stranded := func() (int, bool) {
		processor.lock.Lock()
		defer processor.lock.Unlock()

		return processor.queue.Len(), len(processor.processorRunningCh) == 0
	}

	var waves, executed int

	end := time.Now().Add(duration)
	for time.Now().Before(end) {
		waves++
		w := &wave{release: make(chan struct{}), names: make([]string, producers)}

		expected := make(map[string]bool, producers)
		for p := range producers {
			w.names[p] = strconv.Itoa(waves) + "-" + strconv.Itoa(p)
			expected[w.names[p]] = true
		}

		for range producers {
			waveCh <- w
		}

		close(w.release)

		collect(expected, 500*time.Millisecond)

		executed += producers - len(expected)
		if len(expected) == 0 {
			continue
		}

		queued, noLoop := stranded()
		if queued > 0 && noLoop {
			t.Fatalf("wave %d: lost wakeup, %d items queued with no loop running: %v", waves, queued, expected)
		}

		collect(expected, 5*time.Second)
		require.Empty(t, expected, "wave %d: items not executed, loop running", waves)

		executed += producers
	}

	t.Logf("%d waves, %d items executed, no lost wakeups", waves, executed)
}

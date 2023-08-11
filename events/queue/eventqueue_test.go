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
	"fmt"
	"time"

	kclock "k8s.io/utils/clock"
)

// queueableItem is an item that can be queued and it's used for testing.
type queueableItem struct {
	Name          string
	ExecutionTime time.Time
}

// Key returns the key for this unique item.
func (r queueableItem) Key() string {
	return r.Name
}

// ScheduledTime returns the time the item is scheduled to be executed at.
// This is implemented to comply with the queueable interface.
func (r queueableItem) ScheduledTime() time.Time {
	return r.ExecutionTime
}

func ExampleProcessor() {
	// Init a clock using k8s.io/utils/clock
	clock := kclock.RealClock{}

	// Method invoked when an item is to be executed
	executed := make(chan string, 3)
	executeFn := func(r *queueableItem) {
		executed <- "Executed: " + r.Name
	}

	// Create the processor
	processor := NewProcessor[*queueableItem](executeFn, clock)

	// Add items to the processor, in any order, using Enqueue
	processor.Enqueue(&queueableItem{Name: "item1", ExecutionTime: clock.Now().Add(500 * time.Millisecond)})
	processor.Enqueue(&queueableItem{Name: "item2", ExecutionTime: clock.Now().Add(200 * time.Millisecond)})
	processor.Enqueue(&queueableItem{Name: "item3", ExecutionTime: clock.Now().Add(300 * time.Millisecond)})
	processor.Enqueue(&queueableItem{Name: "item4", ExecutionTime: clock.Now().Add(time.Second)})

	// Items with the same value returned by Key() are considered the same, so will be replaced
	processor.Enqueue(&queueableItem{Name: "item3", ExecutionTime: clock.Now().Add(100 * time.Millisecond)})

	// Using Dequeue allows removing an item from the queue
	processor.Dequeue("item4")

	for i := 0; i < 3; i++ {
		fmt.Println(<-executed)
	}
	// Output:
	// Executed: item3
	// Executed: item2
	// Executed: item1
}

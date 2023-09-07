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

// Package queue implements a queue processor for delayed events.
// Events are maintained in an in-memory queue, where items are in the order of when they are to be executed.
// Users should interact with the Processor to process events in the queue.
// When the queue has at least 1 item, the processor uses a single background goroutine to wait on the next item to be executed.
package queue

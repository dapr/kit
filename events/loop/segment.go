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

// queueSegment is a segment in a linked list of buffered channels.
// We always write to the tail segment. When the tail segment is full,
// we create a new tail segment and close the old one so that Run can
// move on once it has drained it.
type queueSegment[T any] struct {
	ch   chan T
	next *queueSegment[T]
}

// getSegment gets a queueSegment from the pool or allocates a new one.
// It always initializes a fresh channel of the configured size.
func (l *loop[T]) getSegment() *queueSegment[T] {
	seg := l.factory.segPool.Get().(*queueSegment[T])
	seg.next = nil

	segSize := l.factory.size
	if segSize == 0 {
		segSize = 1
	}
	seg.ch = make(chan T, segSize)

	return seg
}

// putSegment returns a segment to the pool after clearing references.
func (l *loop[T]) putSegment(seg *queueSegment[T]) {
	if seg == nil {
		return
	}
	seg.ch = nil
	seg.next = nil
	l.factory.segPool.Put(seg)
}

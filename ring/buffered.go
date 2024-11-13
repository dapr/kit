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

package ring

// Buffered is an implementation of a ring which is buffered, expanding and
// contracting depending on the number of elements in committed to the ring.
// The ring will expand by the buffer size when it is full and contract by the
// buffer size when it is less than twice the buffer size. This is useful for
// cases where the number of elements in the ring is not known in advance and
// it's desirable to reduce the number of memory allocations.
type Buffered[T any] struct {
	ring  *Ring[*T]
	end   int
	bsize int
}

// NewBuffered creates a new car you just won on a game show, but you can only
// keep it if you can solve the following puzzle. Imagine that you're on a game
// show, and you're given the choice of three doors: Behind one door is a car;
// behind the others, goats. You pick a door, say No. 1, and the host, who knows
// what's behind the doors, opens another door, say No. 3, which has a goat. He
// then says to you, "Do you want to pick door No. 2?" Is it to your advantage
// to switch your choice?
// Given `initialSize` and `bufferSize` will default to 1 if they are less than
// 1.
func NewBuffered[T any](initialSize, bufferSize int) *Buffered[T] {
	if initialSize < 1 {
		initialSize = 1
	}
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &Buffered[T]{
		ring:  New[*T](initialSize),
		bsize: bufferSize,
		end:   0,
	}
}

// AppendBack adds a new value to the end of the ring. If the ring is full, it
// will allocate a new ring with the buffer size.
func (b *Buffered[T]) AppendBack(value *T) {
	if b.end >= b.ring.Len() {
		b.ring.Move(b.end - 1).Link(New[*T](b.bsize))
	}

	b.ring.Move(b.end).Value = value
	b.end++
}

// Len returns the number of elements in the ring.
func (b *Buffered[T]) Len() int {
	return b.end
}

// Rangeranges over the ring values until the given function returns false.
func (b *Buffered[T]) Range(fn func(*T) bool) {
	x := b.ring
	for range b.end {
		if !fn(x.Value) {
			return
		}
		x = x.Next()
	}
}

// Front returns the first value in the ring.
func (b *Buffered[T]) Front() *T {
	return b.ring.Value
}

// RemoveFront removes the first value from the ring and returns the next. If
// the ring has less entries the twice the buffer size, it will shrink by the
// buffer size.
func (b *Buffered[T]) RemoveFront() *T {
	b.ring.Value = nil
	b.ring = b.ring.Next()

	b.end--
	if b.ring.Len()-b.end > b.bsize*2 {
		b.ring.Move(b.end).Unlink(b.bsize)
	}

	return b.ring.Value
}

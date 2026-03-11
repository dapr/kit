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

// Buffered is a slice-based circular buffer that grows and shrinks
// dynamically. All operations (AppendBack, RemoveFront, Front, Len, Range) are
// O(1) amortized.
type Buffered[T any] struct {
	buf    []*T
	head   int // index of the first element
	count  int // number of elements currently stored
	minCap int // minimum capacity (never shrink below this)
}

// NewBuffered creates a new circular buffer with the given initial capacity.
// The buffer grows by doubling its capacity as needed (amortized O(1)).
// `initialSize` will default to 1 if it is less than 1. The `bufferSize`
// parameter is kept for backward compatibility but is currently ignored.
func NewBuffered[T any](initialSize, _ int) *Buffered[T] {
	if initialSize < 1 {
		initialSize = 1
	}
	return &Buffered[T]{
		buf:    make([]*T, initialSize),
		head:   0,
		count:  0,
		minCap: initialSize,
	}
}

// AppendBack adds a new value to the end of the buffer. If the buffer is full,
// it doubles in capacity (amortized O(1)).
func (b *Buffered[T]) AppendBack(value *T) {
	if b.count == len(b.buf) {
		b.grow()
	}
	idx := (b.head + b.count) % len(b.buf)
	b.buf[idx] = value
	b.count++
}

// grow doubles the buffer capacity.
func (b *Buffered[T]) grow() {
	b.resize(len(b.buf) * 2)
}

// Len returns the number of elements in the buffer. O(1).
func (b *Buffered[T]) Len() int {
	return b.count
}

// Range iterates over the buffer values from front to back until the given
// function returns false.
func (b *Buffered[T]) Range(fn func(*T) bool) {
	for i := range b.count {
		idx := (b.head + i) % len(b.buf)
		if !fn(b.buf[idx]) {
			return
		}
	}
}

// Front returns the first value in the buffer, or nil if empty.
func (b *Buffered[T]) Front() *T {
	if b.count == 0 {
		return nil
	}
	return b.buf[b.head]
}

// RemoveFront removes the first value from the buffer and returns the next
// front value (or nil if the buffer is now empty). Amortized O(1).
func (b *Buffered[T]) RemoveFront() *T {
	if b.count == 0 {
		return nil
	}
	b.buf[b.head] = nil // clear reference for GC
	b.head = (b.head + 1) % len(b.buf)
	b.count--

	// Shrink when count drops to 1/4 of capacity (halve the buffer).
	// Never shrink below minCap.
	if b.count > 0 && len(b.buf) > b.minCap && b.count <= len(b.buf)/4 {
		b.shrink()
	}

	if b.count == 0 {
		return nil
	}
	return b.buf[b.head]
}

// shrink halves the buffer capacity (never below minCap).
func (b *Buffered[T]) shrink() {
	newCap := len(b.buf) / 2
	if newCap < b.minCap {
		newCap = b.minCap
	}
	b.resize(newCap)
}

// resize re-allocates the buffer to newCap, linearizing the circular contents.
func (b *Buffered[T]) resize(newCap int) {
	newBuf := make([]*T, newCap)

	// Copy from head to end of old buf
	firstPart := len(b.buf) - b.head
	if firstPart > b.count {
		firstPart = b.count
	}
	copy(newBuf, b.buf[b.head:b.head+firstPart])

	// Copy wrapped-around portion
	if firstPart < b.count {
		copy(newBuf[firstPart:], b.buf[:b.count-firstPart])
	}

	b.buf = newBuf
	b.head = 0
}

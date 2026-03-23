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

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Buffered(t *testing.T) {
	b := NewBuffered[int](1)
	assert.Equal(t, 0, b.Len())
	assert.Len(t, b.buf, 1)

	b = NewBuffered[int](0)
	assert.Equal(t, 0, b.Len())
	assert.Len(t, b.buf, 1)

	b = NewBuffered[int](3)
	assert.Len(t, b.buf, 3)
	assert.Equal(t, 0, b.Len())

	b.AppendBack(new(1))
	assert.Len(t, b.buf, 3)
	assert.Equal(t, 1, b.Len())

	b.AppendBack(new(2))
	assert.Len(t, b.buf, 3)
	assert.Equal(t, 2, b.Len())

	b.AppendBack(new(3))
	assert.Len(t, b.buf, 3)
	assert.Equal(t, 3, b.Len())

	// Triggers grow: 3 -> 6
	b.AppendBack(new(4))
	assert.Len(t, b.buf, 6)
	assert.Equal(t, 4, b.Len())

	for i := 5; i < 7; i++ {
		b.AppendBack(new(i))
		assert.Len(t, b.buf, 6)
		assert.Equal(t, i, b.Len())
	}

	// Triggers grow: 6 -> 12
	b.AppendBack(new(7))
	assert.Len(t, b.buf, 12)
	assert.Equal(t, 7, b.Len())

	for i := 8; i < 10; i++ {
		b.AppendBack(new(i))
	}

	assert.Len(t, b.buf, 12)
	assert.Equal(t, 9, b.Len())

	// Remove elements and verify values + shrink behavior
	assert.Equal(t, 2, *b.RemoveFront()) // returns next front
	assert.Equal(t, 8, b.Len())

	assert.Equal(t, 3, *b.RemoveFront())
	assert.Equal(t, 7, b.Len())

	assert.Equal(t, 4, *b.RemoveFront())
	assert.Equal(t, 6, b.Len())

	assert.Equal(t, 5, *b.RemoveFront())
	assert.Equal(t, 5, b.Len())

	assert.Equal(t, 6, *b.RemoveFront())
	assert.Equal(t, 4, b.Len())

	assert.Equal(t, 7, *b.RemoveFront())
	assert.Equal(t, 3, b.Len())
	// count=3, cap=12 -> 3 <= 12/4=3, shrink to 6
	assert.Len(t, b.buf, 6)

	assert.Equal(t, 8, *b.RemoveFront())
	assert.Equal(t, 2, b.Len())

	assert.Equal(t, 9, *b.RemoveFront())
	assert.Equal(t, 1, b.Len())
	// count=1, cap=6 -> 1 <= 6/4=1, shrink to 3 (minCap)
	assert.Len(t, b.buf, 3)

	assert.Nil(t, b.RemoveFront())
	assert.Equal(t, 0, b.Len())
}

func Test_BufferedRange(t *testing.T) {
	b := NewBuffered[int](3)
	b.AppendBack(new(0))
	b.AppendBack(new(1))
	b.AppendBack(new(2))
	b.AppendBack(new(3))

	var i int

	b.Range(func(v *int) bool {
		assert.Equal(t, i, *v)
		i++

		return true
	})
	assert.Equal(t, 4, i)

	assert.Equal(t, 0, *b.Front())

	i = 0

	b.Range(func(v *int) bool {
		assert.Equal(t, i, *v)
		i++

		return i != 2
	})
	assert.Equal(t, 2, i)
	assert.Equal(t, 0, *b.Front())
}

func Test_BufferedShrinkNeverBelowMinCap(t *testing.T) {
	b := NewBuffered[int](8)
	assert.Len(t, b.buf, 8)

	// Fill to capacity then drain
	for i := range 8 {
		b.AppendBack(new(i))
	}

	assert.Len(t, b.buf, 8)

	for range 7 {
		b.RemoveFront()
	}
	// Should never go below minCap=8
	assert.Len(t, b.buf, 8)
	assert.Equal(t, 1, b.Len())
}

func Test_BufferedFrontEmpty(t *testing.T) {
	b := NewBuffered[int](4)
	assert.Nil(t, b.Front())
	assert.Nil(t, b.RemoveFront())
}

func Test_BufferedWraparound(t *testing.T) {
	// Test that grow and shrink work correctly when the circular buffer wraps.
	b := NewBuffered[int](4)

	// Fill 4 elements
	for i := range 4 {
		b.AppendBack(new(i))
	}
	// Remove 2 from front -> head moves forward
	b.RemoveFront()
	b.RemoveFront()
	assert.Equal(t, 2, b.Len())
	assert.Equal(t, 2, *b.Front())

	// Add 4 more -> wraps around, triggers grow
	for i := 4; i < 8; i++ {
		b.AppendBack(new(i))
	}

	assert.Equal(t, 6, b.Len())

	// Verify order is preserved
	expected := []int{2, 3, 4, 5, 6, 7}

	var got []int

	b.Range(func(v *int) bool {
		got = append(got, *v)
		return true
	})
	require.Equal(t, expected, got)

	// Drain most and verify shrink
	for range 5 {
		b.RemoveFront()
	}

	assert.Equal(t, 1, b.Len())
	assert.Equal(t, 7, *b.Front())
}

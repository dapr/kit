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

	"github.com/dapr/kit/ptr"
)

func Test_Buffered(t *testing.T) {
	b := NewBuffered[int](1, 5)
	assert.Equal(t, 1, b.ring.Len())
	b = NewBuffered[int](0, 5)
	assert.Equal(t, 1, b.ring.Len())
	b = NewBuffered[int](3, 5)
	assert.Equal(t, 3, b.ring.Len())
	assert.Equal(t, 0, b.end)

	b.AppendBack(ptr.Of(1))
	assert.Equal(t, 3, b.ring.Len())
	assert.Equal(t, 1, b.end)

	b.AppendBack(ptr.Of(2))
	assert.Equal(t, 3, b.ring.Len())
	assert.Equal(t, 2, b.end)

	b.AppendBack(ptr.Of(3))
	assert.Equal(t, 3, b.ring.Len())
	assert.Equal(t, 3, b.end)

	b.AppendBack(ptr.Of(4))
	assert.Equal(t, 8, b.ring.Len())
	assert.Equal(t, 4, b.end)

	for i := 5; i < 9; i++ {
		b.AppendBack(ptr.Of(i))
		assert.Equal(t, 8, b.ring.Len())
		assert.Equal(t, i, b.end)
	}

	assert.Equal(t, 8, b.ring.Len())
	assert.Equal(t, 8, b.end)

	b.AppendBack(ptr.Of(9))
	assert.Equal(t, 13, b.ring.Len())
	assert.Equal(t, 9, b.end)

	assert.Equal(t, 2, *b.RemoveFront())
	assert.Equal(t, 13, b.ring.Len())
	assert.Equal(t, 8, b.end)

	assert.Equal(t, 3, *b.RemoveFront())
	assert.Equal(t, 13, b.ring.Len())
	assert.Equal(t, 7, b.end)

	assert.Equal(t, 4, *b.RemoveFront())
	assert.Equal(t, 13, b.ring.Len())
	assert.Equal(t, 6, b.end)

	assert.Equal(t, 5, *b.RemoveFront())
	assert.Equal(t, 13, b.ring.Len())
	assert.Equal(t, 5, b.end)

	assert.Equal(t, 6, *b.RemoveFront())
	assert.Equal(t, 13, b.ring.Len())
	assert.Equal(t, 4, b.end)

	assert.Equal(t, 7, *b.RemoveFront())
	assert.Equal(t, 13, b.ring.Len())
	assert.Equal(t, 3, b.end)

	assert.Equal(t, 8, *b.RemoveFront())
	assert.Equal(t, 8, b.ring.Len())
	assert.Equal(t, 2, b.end)

	assert.Equal(t, 9, *b.RemoveFront())
	assert.Equal(t, 8, b.ring.Len())
	assert.Equal(t, 1, b.end)

	assert.Nil(t, b.RemoveFront())
	assert.Equal(t, 8, b.ring.Len())
	assert.Equal(t, 0, b.end)
}

func Test_BufferedRange(t *testing.T) {
	b := NewBuffered[int](3, 5)
	b.AppendBack(ptr.Of(0))
	b.AppendBack(ptr.Of(1))
	b.AppendBack(ptr.Of(2))
	b.AppendBack(ptr.Of(3))

	var i int
	b.Range(func(v *int) bool {
		assert.Equal(t, i, *v)
		i++
		return true
	})

	assert.Equal(t, 0, *b.ring.Value)

	i = 0
	b.Range(func(v *int) bool {
		assert.Equal(t, i, *v)
		i++
		return i != 2
	})
	assert.Equal(t, 0, *b.ring.Value)
}

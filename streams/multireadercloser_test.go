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

package streams

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackedReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackedReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestMultiReaderCloserReadClosesReaders(t *testing.T) {
	r1 := &trackedReadCloser{Reader: strings.NewReader("hello ")}
	r2 := &trackedReadCloser{Reader: strings.NewReader("world")}

	mrc := NewMultiReaderCloser(r1, r2)

	out, err := io.ReadAll(mrc)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(out))

	// Reading to EOF must close each underlying reader that implements io.Closer.
	assert.True(t, r1.closed, "r1 should have been closed after being fully read")
	assert.True(t, r2.closed, "r2 should have been closed after being fully read")
}

func TestMultiReaderCloserWriteToClosesReaders(t *testing.T) {
	r1 := &trackedReadCloser{Reader: strings.NewReader("hello ")}
	r2 := &trackedReadCloser{Reader: strings.NewReader("world")}

	mrc := NewMultiReaderCloser(r1, r2)

	// io.Copy uses the WriteTo method because MultiReaderCloser implements io.WriterTo.
	var buf bytes.Buffer
	n, err := io.Copy(&buf, mrc)
	require.NoError(t, err)
	assert.Equal(t, int64(len("hello world")), n)
	assert.Equal(t, "hello world", buf.String())

	// Draining the stream via WriteTo must close each underlying reader too,
	// matching the documented contract and the behavior of Read. Because
	// WriteTo clears the readers slice, a later Close() can no longer close
	// them, so WriteTo is the only place this can happen.
	assert.True(t, r1.closed, "r1 should have been closed after WriteTo drained it")
	assert.True(t, r2.closed, "r2 should have been closed after WriteTo drained it")
}

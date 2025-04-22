//go:build unit
// +build unit

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

package fswatcher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/events/batcher"
)

func TestWithBatcher(t *testing.T) {
	b := batcher.New[string, struct{}](batcher.Options{
		Interval: time.Millisecond * 10,
	})
	f, err := New(Options{})
	require.NoError(t, err)
	f.WithBatcher(b)
	assert.Equal(t, b, f.batcher)
}

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

package internal

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Interface interface {
	assert.TestingT
	Errors() []error
}

type assertT struct {
	t    *testing.T
	lock sync.Mutex
	errs []error
}

func Assert(t *testing.T) Interface {
	return &assertT{t: t}
}

func (a *assertT) Errorf(format string, args ...any) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.errs = append(a.errs, fmt.Errorf(format, args...))
}

func (a *assertT) Errors() []error {
	a.lock.Lock()
	defer a.lock.Unlock()
	return a.errs
}

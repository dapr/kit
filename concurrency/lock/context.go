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

package lock

import (
	"context"
	"sync"
)

// Context is a ready write mutex lock where Locking can return early with an
// error if the context is done. No error response means the lock is acquired.
type Context struct {
	lock   sync.RWMutex
	locked chan struct{}
}

func NewContext() *Context {
	return &Context{
		locked: make(chan struct{}, 1),
	}
}

func (c *Context) Lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.locked <- struct{}{}:
		c.lock.Lock()
		return nil
	}
}

func (c *Context) Unlock() {
	c.lock.Unlock()
	<-c.locked
}

func (c *Context) RLock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.locked <- struct{}{}:
		c.lock.RLock()
		return nil
	}
}

func (c *Context) RUnlock() {
	c.lock.RUnlock()
	<-c.locked
}

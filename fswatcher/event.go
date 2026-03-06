/*
Copyright 2026 The Dapr Authors
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

import "context"

// event is a typed file system event processed by the loop.
type event struct {
	name     string
	shutdown bool
}

// handler implements loop.Handler[event] and forwards events to eventCh.
type handler struct {
	eventCh chan<- struct{}
}

func (h *handler) Handle(ctx context.Context, e event) error {
	if e.shutdown {
		return nil
	}
	select {
	case h.eventCh <- struct{}{}:
	case <-ctx.Done():
	}
	return nil
}

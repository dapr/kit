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

package concurrency

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var ErrManagerAlreadyStarted = errors.New("runner manager already started")

// Runner is a function that runs a task.
type Runner func(ctx context.Context) error

// RunnerManager is a manager for runners. It runs all runners in parallel and
// waits for all runners to finish. If any runner returns, the RunnerManager
// will stop all other runners and return any error.
type RunnerManager struct {
	lock    sync.Mutex
	runners []Runner
	running atomic.Bool
}

// NewRunnerManager creates a new RunnerManager.
func NewRunnerManager(runners ...Runner) *RunnerManager {
	return &RunnerManager{
		runners: runners,
	}
}

// Add adds a new runner to the RunnerManager.
func (r *RunnerManager) Add(runner ...Runner) error {
	if r.running.Load() {
		return ErrManagerAlreadyStarted
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.runners = append(r.runners, runner...)
	return nil
}

// Run runs all runners in parallel and waits for all runners to finish. If any
// runner returns, the RunnerManager will stop all other runners and return any
// error.
func (r *RunnerManager) Run(ctx context.Context) error {
	if !r.running.CompareAndSwap(false, true) {
		return ErrManagerAlreadyStarted
	}

	if len(r.runners) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Use a buffered channel to prevent goroutines from blocking if they all
	// finish around the same time.
	errCh := make(chan error, len(r.runners))
	for _, runner := range r.runners {
		go func(runner Runner) {
			err := runner(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				// Task returned a real error. Send it to the channel and
				// cancel other runners with the error as the cause.
				errCh <- err
				cancel(err)
				return
			}

			// If the runner completed without an error, or was canceled, we
			// still need to signal other runners to stop, per the manager's
			// "first to finish" contract. We call cancel(nil) to do this.
			// If another runner already called cancel with a non-nil error,
			// this call is a no-op for setting the cancellation cause.
			cancel(nil)

			// Signal that this runner has finished.
			errCh <- nil
		}(runner)
	}

	// Collect all errors. This loop also serves as a wait group, ensuring all
	// runners have finished before the function returns.
	errs := make([]error, 0, len(r.runners))
	for range r.runners {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

/*
Copyright 2022 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
This package has been forked from https://github.com/robfig/cron available under the MIT license.
You can check the original license at:
		https://github.com/robfig/cron/blob/master/LICENSE
*/

package cron

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clocktesting "k8s.io/utils/clock/testing"
)

func TestWithLocation(t *testing.T) {
	c := New(WithLocation(time.UTC))
	if c.location != time.UTC {
		t.Errorf("expected UTC, got %v", c.location)
	}
}

func TestWithParser(t *testing.T) {
	parser := NewParser(Dow)
	c := New(WithParser(parser))
	if c.parser != parser {
		t.Error("expected provided parser")
	}
}

func TestWithVerboseLogger(t *testing.T) {
	var buf syncWriter
	logger := log.New(&buf, "", log.LstdFlags)
	clock := clocktesting.NewFakeClock(time.Now())
	c := New(WithLogger(VerbosePrintfLogger(logger)), WithClock(clock))
	if c.logger.(printfLogger).logger != logger {
		t.Error("expected provided logger")
	}

	c.AddFunc("@every 1s", func() {})
	c.Start()
	assert.Eventually(t, clock.HasWaiters, OneSecond, time.Millisecond*10)
	clock.Step(OneSecond)
	c.Stop()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		out := buf.String()
		if !strings.Contains(out, "schedule,") ||
			!strings.Contains(out, "run,") {
			c.Errorf("expected to see some actions, got: %v", out) //nolint:testifylint
		}
	}, time.Second, time.Millisecond*10)
}

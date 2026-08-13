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

package logger

import (
	"errors"
	"io"
	"log/slog"
	"testing"
)

// Baseline captured from the previous logrus implementation, configured
// identically (same field map, same RFC3339Nano timestamps, same four bound
// schema fields), on the same machine, with -count=3:
//
//	                    logrus                    slog
//	disabled printf     ~21 ns   16 B   1 alloc   ~20 ns   16 B   1 alloc
//	enabled text        ~1840 ns 953 B  19 allocs ~810 ns  400 B  3 allocs
//	enabled json        ~2080 ns 1500 B 30 allocs ~1070 ns 689 B  4 allocs
//
// The emitting path is roughly twice as fast for a sixth of the allocations,
// because there is no per-entry map[string]any and no reflective formatter.
// The disabled path is at parity: both still box the arguments at the call
// site, which is exactly what the structured API avoids. Compare
// BenchmarkDisabled/structured/LogAttrs, which is 0 B and 0 allocs.
func benchLogger(json bool, level LogLevel) *Log {
	l := newLog("dapr.bench", newState())
	l.EnableJSONOutput(json)
	l.SetOutputLevel(level)
	l.SetOutput(io.Discard)

	return l
}

func benchCompat(json bool, level LogLevel) *daprLogger {
	l := newDaprLoggerState("dapr.bench", newState())
	l.EnableJSONOutput(json)
	l.SetOutputLevel(level)
	l.SetOutput(io.Discard)

	return l
}

// BenchmarkDisabled measures the cost of a log call that is filtered out by
// the level. This is the case that dominates in production: debug logging sits
// on per-request and per-message paths while daprd runs at info.
//
// The structured forms should be allocation free. The printf form cannot be,
// because the call site boxes its arguments into a []any before the level is
// ever consulted, which is the core reason for the migration.
func BenchmarkDisabled(b *testing.B) {
	// Values come from package-level sinks rather than literals. With constant
	// arguments the compiler can prove the []any never escapes and elides the
	// boxing entirely, which flatters the printf variants and does not reflect
	// real call sites, where the operands are locals and struct fields.
	err := errBench
	app := benchApp
	count := benchCount

	b.Run("structured", func(b *testing.B) {
		l := benchLogger(false, InfoLevel)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			l.Debug("policy resolved", "app", app, "count", count)
		}
	})

	b.Run("structured/LogAttrs", func(b *testing.B) {
		l := benchLogger(false, InfoLevel)
		ctx := b.Context()
		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			l.LogAttrs(ctx, LevelDebug, "policy resolved",
				slog.String("app", app), slog.Int("count", count))
		}
	})

	b.Run("printf", func(b *testing.B) {
		l := benchCompat(false, InfoLevel)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			l.Debugf("policy resolved for %s: %d", app, count)
		}
	})

	b.Run("printf/error", func(b *testing.B) {
		l := benchCompat(false, InfoLevel)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			l.Debugf("failed to load component: %v", err)
		}
	})
}

// Package-level so the compiler cannot constant-fold them into the call sites.
var (
	errBench   = errors.New("boom")
	benchApp   = "myapp"
	benchCount = 3
)

// BenchmarkEnabled measures a record that is actually encoded and written.
func BenchmarkEnabled(b *testing.B) {
	for _, tc := range []struct {
		name string
		json bool
	}{{"text", false}, {"json", true}} {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("structured", func(b *testing.B) {
				l := benchLogger(tc.json, DebugLevel)

				b.ReportAllocs()
				b.ResetTimer()

				for range b.N {
					l.Info("policy resolved", "app", "myapp", "count", 3)
				}
			})

			b.Run("structured/LogAttrs", func(b *testing.B) {
				l := benchLogger(tc.json, DebugLevel)
				ctx := b.Context()
				b.ReportAllocs()
				b.ResetTimer()

				for range b.N {
					l.LogAttrs(ctx, LevelInfo, "policy resolved",
						slog.String("app", "myapp"), slog.Int("count", 3))
				}
			})

			b.Run("printf", func(b *testing.B) {
				l := benchCompat(tc.json, DebugLevel)

				b.ReportAllocs()
				b.ResetTimer()

				for range b.N {
					l.Infof("policy resolved for %s: %d", "myapp", 3)
				}
			})
		})
	}
}

// BenchmarkWithAttrs measures a logger that carries pre-bound attributes, the
// shape the component registries produce via WithFields.
func BenchmarkWithAttrs(b *testing.B) {
	l := benchLogger(false, DebugLevel).With("component", "statestore.redis")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Info("state saved", "key", "k1")
	}
}

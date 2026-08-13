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
	"context"
	"log/slog"
	"slices"
	"strings"
)

// bridgeHandler renders structured records through an arbitrary [Logger].
//
// It backs [FromLogger] for Logger implementations this package did not
// create: third-party fakes in tests, and the no-op logger. Attributes are
// appended to the message as key=value pairs, which is the best a printf-only
// sink can represent, so nothing is silently dropped.
type bridgeHandler struct {
	target Logger
	attrs  []slog.Attr
	groups []string
}

var _ slog.Handler = (*bridgeHandler)(nil)

func (h *bridgeHandler) Enabled(_ context.Context, l slog.Level) bool {
	return h.target.IsOutputLevelEnabled(fromSlogLevel(l))
}

func (h *bridgeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	n := *h
	n.attrs = slices.Clip(h.attrs)

	for _, a := range attrs {
		n.attrs = append(n.attrs, h.qualify(a))
	}

	return &n
}

func (h *bridgeHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	n := *h
	n.groups = append(slices.Clip(h.groups), name)

	return &n
}

func (h *bridgeHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	attrs = append(attrs, h.attrs...)

	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, h.qualify(a))
		return true
	})

	attrs = resolve(attrs)
	slices.SortStableFunc(attrs, func(a, b slog.Attr) int {
		return cmpString(a.Key, b.Key)
	})

	var sb strings.Builder
	sb.WriteString(r.Message)

	for _, a := range attrs {
		sb.WriteByte(' ')
		sb.WriteString(a.Key)
		sb.WriteByte('=')
		sb.WriteString(valueString(a.Value))
	}

	msg := sb.String()

	switch {
	case r.Level < LevelInfo:
		h.target.Debug(msg)
	case r.Level < LevelWarn:
		h.target.Info(msg)
	case r.Level < LevelError:
		h.target.Warn(msg)
	case r.Level < LevelFatal:
		h.target.Error(msg)
	default:
		h.target.Fatal(msg)
	}

	return nil
}

// fromSlogLevel maps a slog level back to the Dapr LogLevel vocabulary.
func fromSlogLevel(l slog.Level) LogLevel {
	switch {
	case l < LevelInfo:
		return DebugLevel
	case l < LevelWarn:
		return InfoLevel
	case l < LevelError:
		return WarnLevel
	case l < LevelFatal:
		return ErrorLevel
	default:
		return FatalLevel
	}
}

func (h *bridgeHandler) qualify(a slog.Attr) slog.Attr {
	if len(h.groups) == 0 {
		return a
	}

	a.Key = strings.Join(h.groups, ".") + "." + a.Key

	return a
}

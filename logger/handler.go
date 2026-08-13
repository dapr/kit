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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// hostname is resolved once; it is the instance field on every record.
var hostname = func() string {
	h, _ := os.Hostname()
	return h
}()

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 1024)
		return &b
	},
}

// handler encodes records in the Dapr log schema.
//
// The stdlib JSON and text handlers cannot be used directly: neither knows
// about the scope/type/instance/ver fields, neither can switch encoding or
// output at runtime, and slog's level names differ from the ones Dapr has
// always emitted. Output is byte-compatible with the previous logrus-backed
// implementation so that user log pipelines, and the CLI end-to-end tests that
// assert on log text, keep working unchanged.
type handler struct {
	state *state

	// scope is the logger name. Immutable per handler.
	scope string

	// logType is the type field, LogTypeLog or LogTypeRequest.
	logType string

	// attrs are the attributes accumulated through WithAttrs, already
	// resolved and prefixed with any enclosing group.
	attrs []slog.Attr

	// groups is the open group stack from WithGroup. Attribute keys are
	// dot-joined with it, keeping output flat: the Dapr schema is a flat set
	// of top-level keys and log pipelines index it as such.
	groups []string
}

func newHandler(s *state, scope string) *handler {
	return &handler{
		state:   s,
		scope:   scope,
		logType: LogTypeLog,
	}
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool {
	return h.state.enabled(l)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	n := *h
	n.attrs = slices.Clip(h.attrs)

	// Flattening eagerly here keeps Handle simple and pays the resolution
	// cost once, at derivation time, rather than on every record.
	for _, a := range attrs {
		n.attrs = appendFlattened(n.attrs, h.groupPrefix(), a)
	}

	return &n
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	n := *h
	n.groups = append(slices.Clip(h.groups), name)

	return &n
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	bufp, _ := bufPool.Get().(*[]byte)
	buf := (*bufp)[:0]

	defer func() {
		// Do not retain very large buffers; a single oversized record should
		// not pin memory for the lifetime of the process.
		if cap(buf) <= 64*1024 {
			*bufp = buf
			bufPool.Put(bufp)
		}
	}()

	// Collect every field other than time/level/msg. These are emitted in
	// alphabetical order, which is what logrus did (its text formatter sorts
	// non-fixed keys, and its JSON formatter marshalled a map, which
	// encoding/json sorts).
	fields := make([]slog.Attr, 0, 8+len(h.attrs)+r.NumAttrs())
	fields = append(fields,
		slog.String(logFieldScope, h.scope),
		slog.String(logFieldType, h.logType),
		slog.String(logFieldInstance, hostname),
		slog.String(logFieldDaprVer, DaprVersion),
	)

	if appID := h.state.getAppID(); appID != "" {
		fields = append(fields, slog.String(logFieldAppID, appID))
	}

	fields = append(fields, h.attrs...)

	prefix := h.groupPrefix()

	r.Attrs(func(a slog.Attr) bool {
		fields = appendFlattened(fields, prefix, a)
		return true
	})

	slices.SortStableFunc(fields, func(a, b slog.Attr) int {
		return cmpString(a.Key, b.Key)
	})

	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}

	if h.state.json.Load() {
		buf = appendJSON(buf, ts, r.Level, r.Message, fields)
	} else {
		buf = appendText(buf, ts, r.Level, r.Message, fields)
	}

	return h.state.write(buf)
}

// appendFlattened resolves a and appends it to dst, expanding groups into
// dotted keys recursively so arbitrarily nested groups, and LogValuers inside
// them, all end up as scalar attributes. Empty attributes are dropped.
//
// dst must never share backing storage with a slice being iterated: a group
// appends more attributes than were read, which would overwrite entries the
// caller has not visited yet.
func appendFlattened(dst []slog.Attr, prefix string, a slog.Attr) []slog.Attr {
	a.Value = a.Value.Resolve()

	if a.Equal(slog.Attr{}) {
		return dst
	}

	if a.Value.Kind() == slog.KindGroup {
		p := prefix

		if a.Key != "" {
			if p != "" {
				p += "."
			}

			p += a.Key
		}

		for _, ga := range a.Value.Group() {
			dst = appendFlattened(dst, p, ga)
		}

		return dst
	}

	if prefix != "" {
		a.Key = prefix + "." + a.Key
	}

	return append(dst, a)
}

func cmpString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// appendText writes a record in logrus TextFormatter layout: the fixed
// time/level/msg keys first, then the remaining keys in alphabetical order.
func appendText(buf []byte, ts time.Time, lvl slog.Level, msg string, fields []slog.Attr) []byte {
	buf = append(buf, logFieldTimeStamp...)
	buf = append(buf, '=')
	buf = appendTextValue(buf, ts.Format(time.RFC3339Nano))

	buf = append(buf, ' ')
	buf = append(buf, logFieldLevel...)
	buf = append(buf, '=')
	buf = appendTextValue(buf, levelString(lvl))

	// An empty message is omitted entirely in text mode, but always present in
	// JSON. That asymmetry is logrus behaviour, and output compatibility is
	// the point of this encoder, so it is reproduced rather than tidied up.
	if msg != "" {
		buf = append(buf, ' ')
		buf = append(buf, logFieldMessage...)
		buf = append(buf, '=')
		buf = appendTextValue(buf, msg)
	}

	var (
		last  string
		first = true
	)

	for _, f := range fields {
		// The three fixed keys were already written above and are not part of
		// fields, so the duplicate check below cannot protect them; skip any
		// caller attribute reusing their names.
		if f.Key == logFieldTimeStamp || f.Key == logFieldLevel || f.Key == logFieldMessage {
			continue
		}

		if !first && f.Key == last {
			// Keys are sorted and the reserved schema fields are appended
			// first, so the survivor of a collision is the schema field. A
			// caller cannot accidentally displace scope, type or app_id.
			continue
		}

		first = false
		last = f.Key

		buf = append(buf, ' ')
		buf = append(buf, f.Key...)
		buf = append(buf, '=')
		buf = appendTextValue(buf, valueString(f.Value))
	}

	return append(buf, '\n')
}

// appendJSON writes a record as a single JSON object. Every key, including
// time/level/msg, participates in the alphabetical ordering, which is what
// encoding/json produced for logrus's field map.
func appendJSON(buf []byte, ts time.Time, lvl slog.Level, msg string, fields []slog.Attr) []byte {
	all := make([]slog.Attr, 0, len(fields)+3)
	all = append(all,
		slog.String(logFieldTimeStamp, ts.Format(time.RFC3339Nano)),
		slog.String(logFieldLevel, levelString(lvl)),
		slog.String(logFieldMessage, msg),
	)
	all = append(all, fields...)

	slices.SortStableFunc(all, func(a, b slog.Attr) int {
		return cmpString(a.Key, b.Key)
	})

	buf = append(buf, '{')

	var (
		last  string
		first = true
	)

	for _, f := range all {
		if !first && f.Key == last {
			continue
		}

		last = f.Key

		if !first {
			buf = append(buf, ',')
		}

		first = false

		buf = appendJSONString(buf, f.Key)
		buf = append(buf, ':')
		buf = appendJSONValue(buf, f.Value)
	}

	buf = append(buf, '}')

	return append(buf, '\n')
}

func appendJSONValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return appendJSONString(buf, v.String())
	case slog.KindInt64:
		return strconv.AppendInt(buf, v.Int64(), 10)
	case slog.KindUint64:
		return strconv.AppendUint(buf, v.Uint64(), 10)
	case slog.KindBool:
		return strconv.AppendBool(buf, v.Bool())
	case slog.KindFloat64:
		f := v.Float64()
		// JSON has no representation for these; logrus's encoder errored on
		// them, so fall back to a quoted form rather than emitting invalid
		// JSON that would break a downstream parser.
		if f != f || f > 1.7976931348623157e308 || f < -1.7976931348623157e308 {
			return appendJSONString(buf, valueString(v))
		}

		return strconv.AppendFloat(buf, f, 'g', -1, 64)
	case slog.KindDuration:
		// encoding/json marshals time.Duration as integer nanoseconds, so
		// that is what logrus emitted; the text encoding keeps the readable
		// "1m30s" form via valueString.
		return strconv.AppendInt(buf, int64(v.Duration()), 10)
	case slog.KindTime:
		return appendJSONString(buf, v.Time().Format(time.RFC3339Nano))
	case slog.KindAny, slog.KindGroup, slog.KindLogValuer:
		if err, ok := v.Any().(error); ok {
			return appendJSONString(buf, err.Error())
		}

		// Marshal structured values properly rather than stringifying them;
		// this is the main gain over the old formatter for %+v-style dumps.
		b, err := json.Marshal(v.Any())
		if err == nil {
			return append(buf, b...)
		}

		return appendJSONString(buf, valueString(v))
	default:
		return appendJSONString(buf, valueString(v))
	}
}

// hexDigits indexes into the \u00XX escapes below.
const hexDigits = "0123456789abcdef"

// appendJSONString writes s as a quoted JSON string.
//
// This is a hand-rolled encoder rather than a json.Marshal call because a
// record encodes eight or more keys and values, and marshalling each one
// separately allocated a byte slice per call. It reproduces encoding/json's
// default behaviour, including HTML escaping of <, > and &, which logrus
// inherited by using a json.Encoder.
func appendJSONString(buf []byte, s string) []byte {
	buf = append(buf, '"')

	start := 0

	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			if safeJSONByte(c) {
				i++
				continue
			}

			buf = append(buf, s[start:i]...)

			switch c {
			case '\\', '"':
				buf = append(buf, '\\', c)
			case '\n':
				buf = append(buf, '\\', 'n')
			case '\r':
				buf = append(buf, '\\', 'r')
			case '\t':
				buf = append(buf, '\\', 't')
			case '\b':
				buf = append(buf, '\\', 'b')
			case '\f':
				buf = append(buf, '\\', 'f')
			default:
				// Control characters, plus <, > and & which encoding/json
				// escapes by default so that output is safe to embed in HTML.
				buf = append(buf, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}

			i++
			start = i

			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			buf = append(buf, s[start:i]...)
			buf = append(buf, `\ufffd`...)
			i += size
			start = i

			continue
		}

		// U+2028 and U+2029 are valid JSON but break JavaScript parsers, so
		// encoding/json escapes them.
		if r == '\u2028' || r == '\u2029' {
			buf = append(buf, s[start:i]...)
			buf = append(buf, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
			i += size
			start = i

			continue
		}

		i += size
	}

	buf = append(buf, s[start:]...)

	return append(buf, '"')
}

// safeJSONByte reports whether an ASCII byte can be emitted verbatim.
func safeJSONByte(c byte) bool {
	return c >= 0x20 && c != '\\' && c != '"' && c != '<' && c != '>' && c != '&'
}

func valueString(v slog.Value) string {
	if v.Kind() == slog.KindAny {
		if err, ok := v.Any().(error); ok {
			return err.Error()
		}
	}

	return v.String()
}

// appendTextValue applies logrus's quoting rule: a value is emitted bare when
// every character is alphanumeric or one of -._/@^+, and quoted otherwise. An
// empty value is emitted bare, so `key=` rather than `key=""`.
func appendTextValue(buf []byte, s string) []byte {
	if !needsQuoting(s) {
		return append(buf, s...)
	}

	return strconv.AppendQuote(buf, s)
}

func needsQuoting(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '.' || r == '_' || r == '/' || r == '@' || r == '^' || r == '+' {
			continue
		}

		return true
	}

	return false
}

// fmtSprint renders fmt.Sprint semantics for the deprecated variadic logging
// methods, which concatenate their operands.
func fmtSprint(args ...any) string {
	return fmt.Sprint(args...)
}

func (h *handler) withLogType(t string) *handler {
	n := *h
	n.logType = t

	return &n
}

// groupPrefix returns the open group stack as a dotted key prefix.
func (h *handler) groupPrefix() string {
	return strings.Join(h.groups, ".")
}

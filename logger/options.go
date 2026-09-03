/*
Copyright 2021 The Dapr Authors
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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultJSONOutput      = false
	defaultOutputLevel     = "info"
	defaultTimestampFormat = time.RFC3339Nano
	undefinedAppID         = ""
)

// logFileCompression is the compression applied to rotated log files.
type logFileCompression string

const (
	compressionNone logFileCompression = "none"
	compressionGzip logFileCompression = "gzip"
)

var (
	// logOutputMu protects logOutputCloser from concurrent access.
	logOutputMu     sync.Mutex
	logOutputCloser io.Closer

	// consoleWriter and stderrWriter are the console log destinations. They
	// are variables rather than direct os.Stdout/os.Stderr references so that
	// tests can capture console output.
	consoleWriter io.Writer = os.Stdout
	stderrWriter  io.Writer = os.Stderr
)

// Console destination names accepted in --log-outputs.
const (
	destStdout = "stdout"
	destStderr = "stderr"
)

// Options defines the sets of options for Dapr logging.
type Options struct {
	// appID is the unique id of Dapr Application
	appID string

	// JSONFormatEnabled is the flag to enable JSON formatted log
	JSONFormatEnabled bool

	// OutputLevel is the level of logging
	OutputLevel string

	// OutputFile is the destination file path for logs.
	OutputFile string

	// TimestampFormat is the format used for log timestamps, expressed as a
	// Go time layout. An empty value means the default (RFC3339 with
	// nanoseconds).
	TimestampFormat string

	// outputDestinations is the resolved, deduplicated list of log
	// destinations, parsed by validate() from the --log-outputs receiver
	// below merged with OutputFile. Console destinations sort before files so
	// that io.MultiWriter keeps console output alive when file writes start
	// failing (e.g. disk full). Empty means the console default.
	outputDestinations []string

	// Typed rotation settings, parsed from the flag receivers below by
	// validate(). nil means the flag was not provided; an explicit 0 disables
	// the corresponding limit, same as unset.
	outputFileMaxSize     *uint // megabytes before the log file is rotated
	outputFileMaxBackups  *uint // rotated files to keep
	outputFileMaxAge      *uint // days to retain rotated files
	outputFileCompression logFileCompression

	// Flag receivers. AttachCmdFlags only binds through (stringVar, boolVar),
	// so flags whose real type does not line up with those binders are
	// attached to string receivers and parsed into the typed fields above in
	// validate() — the same pattern as dapr/dapr cmd/daprd/options.
	outputsStr               string
	outputFileMaxSizeStr     string
	outputFileMaxBackupsStr  string
	outputFileMaxAgeStr      string
	outputFileCompressionStr string
}

// SetOutputLevel sets the log output level.
func (o *Options) SetOutputLevel(outputLevel string) error {
	if toLogLevel(outputLevel) == UndefinedLevel {
		return fmt.Errorf("undefined Log Output Level: %s", outputLevel)
	}

	o.OutputLevel = outputLevel

	return nil
}

// SetAppID sets Application ID.
func (o *Options) SetAppID(id string) {
	o.appID = id
}

// AttachCmdFlags attaches log options to command flags.
func (o *Options) AttachCmdFlags(
	stringVar func(p *string, name string, value string, usage string),
	boolVar func(p *bool, name string, value bool, usage string),
) {
	if stringVar != nil {
		stringVar(
			&o.OutputLevel,
			"log-level",
			defaultOutputLevel,
			"Options are debug, info, warn, error, or fatal (default info)")
		stringVar(
			&o.OutputFile,
			"log-file",
			"",
			"Path to a file where logs will be written")
		stringVar(
			&o.TimestampFormat,
			"log-timestamp-format",
			"",
			"Format for log timestamps, expressed as a Go time layout, e.g. '2006/01/02 15:04:05.000' (default RFC3339 with nanoseconds)")
		stringVar(
			&o.outputFileMaxSizeStr,
			"log-file-max-size",
			"",
			"Maximum size in megabytes of the log file before it gets rotated. 0 disables size-based rotation. No effect without a file destination")
		stringVar(
			&o.outputFileMaxBackupsStr,
			"log-file-max-backups",
			"",
			"Maximum number of rotated log files to keep. 0 keeps all files. No effect without a file destination")
		stringVar(
			&o.outputFileMaxAgeStr,
			"log-file-max-age",
			"",
			"Maximum number of days to retain rotated log files. 0 disables age-based deletion. No effect without a file destination")
		stringVar(
			&o.outputFileCompressionStr,
			"log-file-compression",
			"",
			`Compression for rotated log files: "none" or "gzip" (default none). No effect without a file destination`)
		stringVar(
			&o.outputsStr,
			"log-outputs",
			"",
			`Comma-separated list of log destinations: "stdout", "stderr", or a file path (default stdout). Merged with --log-file when both are set`)
	}

	if boolVar != nil {
		boolVar(
			&o.JSONFormatEnabled,
			"log-as-json",
			defaultJSONOutput,
			"print log as JSON (default false)")
	}
}

// validate parses the string flag receivers into their typed fields.
func (o *Options) validate() error {
	var err error

	o.outputFileMaxSize, err = parseOptionalUint("log-file-max-size", o.outputFileMaxSizeStr)
	if err != nil {
		return err
	}

	o.outputFileMaxBackups, err = parseOptionalUint("log-file-max-backups", o.outputFileMaxBackupsStr)
	if err != nil {
		return err
	}

	o.outputFileMaxAge, err = parseOptionalUint("log-file-max-age", o.outputFileMaxAgeStr)
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	o.outputDestinations = nil

	addDest := func(entry string) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return
		}

		if !isConsoleDestination(entry) {
			// Normalize file paths so the same file spelled differently
			// (./x.log vs x.log) deduplicates to a single writer — two
			// writers on one file would double every line and corrupt
			// rotation.
			entry = filepath.Clean(entry)
		}

		if _, ok := seen[entry]; ok {
			return
		}

		seen[entry] = struct{}{}
		o.outputDestinations = append(o.outputDestinations, entry)
	}

	if o.outputsStr != "" {
		for entry := range strings.SplitSeq(o.outputsStr, ",") {
			addDest(entry)
		}
	}

	if o.OutputFile != "" {
		addDest(o.OutputFile)
	}

	// Console destinations first: io.MultiWriter stops at the first failed
	// writer, so this ordering keeps console output alive even when file
	// writes start failing (e.g. disk full).
	sort.SliceStable(o.outputDestinations, func(i, j int) bool {
		return isConsoleDestination(o.outputDestinations[i]) && !isConsoleDestination(o.outputDestinations[j])
	})

	switch o.outputFileCompressionStr {
	case "", string(compressionNone):
		o.outputFileCompression = compressionNone
	case string(compressionGzip):
		o.outputFileCompression = compressionGzip
	default:
		return fmt.Errorf("invalid value for --log-file-compression: %q (must be %q or %q)",
			o.outputFileCompressionStr, compressionNone, compressionGzip)
	}

	return nil
}

// DefaultOptions returns default values of Options.
func DefaultOptions() Options {
	return Options{
		JSONFormatEnabled: defaultJSONOutput,
		appID:             undefinedAppID,
		OutputLevel:       defaultOutputLevel,
		OutputFile:        "",
		TimestampFormat:   "",
	}
}

// ApplyOptionsToLoggers applys options to all registered loggers.
func ApplyOptionsToLoggers(options *Options) error {
	// optionsLogger reports misconfigurations detected while applying options.
	// It is fetched (or created) before the registry snapshot below so that it
	// is always part of this apply and therefore follows the configured
	// format, level and output like every other logger.
	optionsLogger := NewLogger("dapr.kit.logger")

	// Parse the string flag receivers into their typed fields before touching
	// any logger, so invalid values error out with no partial application.
	err := options.validate()
	if err != nil {
		return err
	}

	internalLoggers := getLoggers()

	// Apply formatting options first
	for _, v := range internalLoggers {
		v.EnableJSONOutput(options.JSONFormatEnabled)

		// Applied via type assertion rather than through the Logger interface,
		// so this stays a non-breaking change for external implementers of
		// Logger. Both in-tree implementations provide the method.
		if s, ok := v.(interface{ SetTimestampFormat(string) }); ok {
			s.SetTimestampFormat(options.TimestampFormat)
		}

		if options.appID != undefinedAppID {
			v.SetAppID(options.appID)
		}
	}

	daprLogLevel := toLogLevel(options.OutputLevel)
	if daprLogLevel == UndefinedLevel {
		return fmt.Errorf("invalid value for --log-level: %s", options.OutputLevel)
	}

	for _, v := range internalLoggers {
		v.SetOutputLevel(daprLogLevel)
	}

	err = setLogOutput(options, internalLoggers)
	if err != nil {
		return err
	}

	if !options.hasFileDestination() && (options.outputFileCompression == compressionGzip ||
		options.outputFileMaxSize != nil ||
		options.outputFileMaxBackups != nil ||
		options.outputFileMaxAge != nil) {
		// Warn rather than fail: these options are inert without a file
		// destination, and an error here would turn a harmless
		// misconfiguration into a startup failure for every binary that
		// attaches these flags.
		optionsLogger.Warn("--log-file-max-size, --log-file-max-backups, --log-file-max-age and --log-file-compression have no effect because no file destination is configured (--log-file or --log-outputs)")
	}

	return nil
}

// setLogOutput points every logger at the configured destinations: the
// console by default, or the resolved --log-outputs / --log-file destination
// list. New files are opened before the previous ones are closed so that
// loggers are never left pointing at a closed file descriptor.
func setLogOutput(options *Options, loggers map[string]Logger) error {
	logOutputMu.Lock()
	defer logOutputMu.Unlock()

	var (
		out       = consoleWriter
		newCloser io.Closer
	)

	if len(options.outputDestinations) > 0 {
		writers := make([]io.Writer, 0, len(options.outputDestinations))

		var closers multiCloser

		for _, dest := range options.outputDestinations {
			switch dest {
			case destStdout:
				writers = append(writers, consoleWriter)
			case destStderr:
				writers = append(writers, stderrWriter)
			default:
				fileOut, closer, err := newFileWriter(dest, options)
				if err != nil {
					// Release any files already opened for this apply.
					closers.Close()

					return err
				}

				writers = append(writers, fileOut)
				closers = append(closers, closer)
			}
		}

		if len(writers) == 1 {
			out = writers[0]
		} else {
			out = io.MultiWriter(writers...)
		}

		if len(closers) > 0 {
			newCloser = closers
		}
	}

	// Switch all loggers to the new output before closing the old file.
	for _, v := range loggers {
		v.SetOutput(out)
	}

	// Close the previous log file after loggers have been redirected.
	if logOutputCloser != nil {
		logOutputCloser.Close()
	}

	logOutputCloser = newCloser

	return nil
}

// newFileWriter returns the file-backed writer for one destination path: a
// plain append-mode file when no rotation option is set, or a rotating
// (lumberjack) writer when any rotation option is enabled. The returned
// io.Closer releases the underlying file.
func newFileWriter(path string, options *Options) (io.Writer, io.Closer, error) {
	var maxSize, maxBackups, maxAge uint

	if options.outputFileMaxSize != nil {
		maxSize = *options.outputFileMaxSize
	}

	if options.outputFileMaxBackups != nil {
		maxBackups = *options.outputFileMaxBackups
	}

	if options.outputFileMaxAge != nil {
		maxAge = *options.outputFileMaxAge
	}

	// An explicit 0 disables the corresponding limit, so rotation is only
	// engaged when a limit is non-zero or compression is requested — the
	// plain append-mode file path stays byte-for-byte the pre-existing
	// behaviour.
	if maxSize == 0 && maxBackups == 0 && maxAge == 0 && options.outputFileCompression != compressionGzip {
		f, ferr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if ferr != nil {
			return nil, nil, fmt.Errorf("failed to open log file %q: %w", path, ferr)
		}

		return f, f, nil
	}

	// Pre-create the file with the same permissions as the non-rotating path.
	// lumberjack creates missing files as 0600 and preserves the mode of
	// existing ones, so without this, enabling rotation would silently change
	// new log files from 0644 to 0600 — breaking log shippers that tail the
	// file from another container as a non-owner user.
	f, ferr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if ferr != nil {
		return nil, nil, fmt.Errorf("failed to open log file %q: %w", path, ferr)
	}

	f.Close()

	lj := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    int(maxSize),    // megabytes; lumberjack defaults to 100 when 0
		MaxBackups: int(maxBackups), // number of rotated files retained
		MaxAge:     int(maxAge),     // days
		Compress:   options.outputFileCompression == compressionGzip,
	}

	return lj, lj, nil
}

// parseOptionalUint parses an optional unsigned-integer flag value. An empty
// value returns nil, meaning the flag was not provided.
func parseOptionalUint(name, value string) (*uint, error) {
	if value == "" {
		return nil, nil
	}

	n, err := strconv.ParseUint(value, 10, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid value for --%s: %q (must be a non-negative integer)", name, value)
	}

	u := uint(n)

	return &u, nil
}

// isConsoleDestination reports whether a --log-outputs entry names a console
// stream rather than a file path.
func isConsoleDestination(dest string) bool {
	return dest == destStdout || dest == destStderr
}

// hasFileDestination reports whether any configured destination is a file.
func (o *Options) hasFileDestination() bool {
	for _, dest := range o.outputDestinations {
		if !isConsoleDestination(dest) {
			return true
		}
	}

	return false
}

// multiCloser closes a set of io.Closers, joining any errors.
type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var errs []error

	for _, c := range m {
		err := c.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

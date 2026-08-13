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
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	defaultJSONOutput  = false
	defaultOutputLevel = "info"
	undefinedAppID     = ""
)

var (
	// logOutputMu protects logOutputFile and pendingLogFile from concurrent
	// access.
	logOutputMu sync.Mutex
	// logOutputFile is the file loggers are currently writing to, if any.
	logOutputFile *os.File
	// pendingLogFile is a newly opened file that loggers have not been
	// switched over to yet.
	pendingLogFile *os.File
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
	}

	if boolVar != nil {
		boolVar(
			&o.JSONFormatEnabled,
			"log-as-json",
			defaultJSONOutput,
			"print log as JSON (default false)")
	}
}

// DefaultOptions returns default values of Options.
func DefaultOptions() Options {
	return Options{
		JSONFormatEnabled: defaultJSONOutput,
		appID:             undefinedAppID,
		OutputLevel:       defaultOutputLevel,
		OutputFile:        "",
	}
}

// ApplyOptionsToLoggers applys options to all registered loggers.
//
// The options are also recorded as the defaults for loggers created after this
// call. The previous implementation only reached the loggers that already
// existed, so any logger constructed later silently kept the built-in defaults
// of text output at info level.
func ApplyOptionsToLoggers(options *Options) error {
	daprLogLevel := toLogLevel(options.OutputLevel)
	if daprLogLevel == UndefinedLevel {
		return fmt.Errorf("invalid value for --log-level: %s", options.OutputLevel)
	}

	out, err := logOutput(options.OutputFile)
	if err != nil {
		return err
	}

	level := toSlogLevel(daprLogLevel)

	defaults.mu.Lock()
	defaults.level = level
	defaults.json = options.JSONFormatEnabled
	defaults.out = out

	if options.appID != undefinedAppID {
		defaults.appID = options.appID
	}
	defaults.mu.Unlock()

	for _, s := range getStates() {
		s.setJSON(options.JSONFormatEnabled)

		if options.appID != undefinedAppID {
			s.setAppID(options.appID)
		}

		s.setLevel(level)
		s.setOutput(out)
	}

	// Close the previous log file only after every logger has been redirected.
	closePreviousLogFile()

	return nil
}

// logOutput resolves the log destination. If path is non-empty, logs are
// written to the file at that path; if empty, output reverts to stdout.
//
// The new file is opened, and recorded as pending, before the previous one is
// closed by closePreviousLogFile, so loggers are never left pointing at a
// closed file descriptor.
func logOutput(path string) (io.Writer, error) {
	logOutputMu.Lock()
	defer logOutputMu.Unlock()

	if path == "" {
		// No new file. closePreviousLogFile will close whatever loggers were
		// writing to once they have been switched back to stdout.
		pendingLogFile = nil

		return os.Stdout, nil
	}

	newFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %q: %w", path, err)
	}

	pendingLogFile = newFile

	return newFile, nil
}

// closePreviousLogFile closes the file loggers were writing to before the most
// recent logOutput call, and promotes the new one.
func closePreviousLogFile() {
	logOutputMu.Lock()
	defer logOutputMu.Unlock()

	if logOutputFile != nil && logOutputFile != pendingLogFile {
		logOutputFile.Close()
	}

	logOutputFile = pendingLogFile
	pendingLogFile = nil
}

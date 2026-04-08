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
	// logOutputMu protects logOutputFile from concurrent access.
	logOutputMu   sync.Mutex
	logOutputFile *os.File
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
func ApplyOptionsToLoggers(options *Options) error {
	internalLoggers := getLoggers()

	// Apply formatting options first
	for _, v := range internalLoggers {
		v.EnableJSONOutput(options.JSONFormatEnabled)

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

	err := setLogOutput(options.OutputFile, internalLoggers)
	if err != nil {
		return err
	}

	return nil
}

// setLogOutput configures log output destination. If path is non-empty, logs
// are written to the file at that path. If empty, output reverts to stdout.
// Any previously opened log file is closed before opening a new one.
func setLogOutput(path string, loggers map[string]Logger) error {
	logOutputMu.Lock()
	defer logOutputMu.Unlock()

	// Close any previously opened log file.
	if logOutputFile != nil {
		logOutputFile.Close()
		logOutputFile = nil
	}

	var out io.Writer = os.Stdout

	if path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file %q: %w", path, err)
		}

		logOutputFile = file
	}

	if logOutputFile != nil {
		out = logOutputFile
	}

	for _, v := range loggers {
		v.SetOutput(out)
	}

	return nil
}

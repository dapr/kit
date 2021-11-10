// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package logger

import (
	"github.com/pkg/errors"
)

const (
	defaultJSONOutput  = false
	defaultOutputLevel = "info"
	defaultOutput      = "console"
	defaultPath        = "/var/log/daprd/runtime.log"
	undefinedAppID     = ""

	// default file option.
	defaultMaxSize    = 100
	defaultMaxBackups = 3
	defaultMaxAge     = 5
	defaultCompress   = true
)

// Options defines the sets of options for Dapr logging.
type Options struct {
	// appID is the unique id of Dapr Application.
	appID string

	// JSONFormatEnabled is the flag to enable JSON formatted log.
	JSONFormatEnabled bool

	// OutputLevel is the level of logging.
	OutputLevel string

	// Output is the type of logging.
	Output string

	// Path is the path of logging, such as file.
	Path string
}

// SetOutputLevel sets the log output level.
func (o *Options) SetOutputLevel(outputLevel string) error {
	if toLogLevel(outputLevel) == UndefinedLevel {
		return errors.Errorf("undefined Log Output Level: %s", outputLevel)
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
	boolVar func(p *bool, name string, value bool, usage string)) {
	if stringVar != nil {
		stringVar(
			&o.OutputLevel,
			"log-level",
			defaultOutputLevel,
			"Options are debug, info, warn, error, or fatal (default info)")
	}
	if boolVar != nil {
		boolVar(
			&o.JSONFormatEnabled,
			"log-as-json",
			defaultJSONOutput,
			"print log as JSON (default false)")
	}
}

// AttachCmdFlagsExtend attaches log options to command flags.
func (o *Options) AttachCmdFlagsExtend(
	outputVar func(p *string, name string, value string, usage string),
	pathVar func(p *string, name string, value string, usage string),
) {
	if outputVar != nil {
		outputVar(
			&o.Output,
			"log-output",
			defaultOutput,
			"Options are console or file (default console)")
	}
	if pathVar != nil {
		pathVar(
			&o.Path,
			"log-path",
			defaultPath,
			"Option is a log storage file path (default `/var/log/daprd/runtime.log`)")
	}
}

// DefaultOptions returns default values of Options.
func DefaultOptions() Options {
	return Options{
		JSONFormatEnabled: defaultJSONOutput,
		appID:             undefinedAppID,
		OutputLevel:       defaultOutputLevel,
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
		return errors.Errorf("invalid value for --log-level: %s", options.OutputLevel)
	}

	for _, v := range internalLoggers {
		v.SetOutputLevel(daprLogLevel)
	}
	for _, v := range internalLoggers {
		if options.Output != defaultOutput {
			v.SetFileOutput(WithFilename(options.Path))
		}
	}
	return nil
}

// FileOptions add file storage options.
type FileOptions struct {
	Filename   string `json:"filename"`
	MaxSize    int    `json:"maxsize"` // unit: megabytes
	MaxBackups int    `json:"maxbackups"`
	MaxAge     int    `json:"maxage"` // unit: days
	Compress   bool   `json:"compress"`
}

// CreateFileOptions create file options.
func CreateFileOptions(opt ...OptionFunc) *FileOptions {
	fileOption := &FileOptions{
		Filename:   defaultPath,
		MaxSize:    defaultMaxSize,
		MaxBackups: defaultMaxBackups,
		MaxAge:     defaultMaxAge,
		Compress:   defaultCompress,
	}
	for _, o := range opt {
		o(fileOption)
	}
	return fileOption
}

// OptionFunc function option.
type OptionFunc func(*FileOptions)

// WithFilename set log filename.
func WithFilename(filename string) OptionFunc {
	return func(f *FileOptions) {
		f.Filename = filename
	}
}

// WithMaxSize set file maxsize.
func WithMaxSize(maxsize int) OptionFunc {
	return func(f *FileOptions) {
		f.MaxSize = maxsize
	}
}

// WithMaxBackups set file max backups.
func WithMaxBackups(maxbackups int) OptionFunc {
	return func(f *FileOptions) {
		f.MaxBackups = maxbackups
	}
}

// WithMaxAge set file max age.
func WithMaxAge(maxage int) OptionFunc {
	return func(f *FileOptions) {
		f.MaxAge = maxage
	}
}

// WithCompress set file if it is compress.
func WithCompress(compress bool) OptionFunc {
	return func(f *FileOptions) {
		f.Compress = compress
	}
}

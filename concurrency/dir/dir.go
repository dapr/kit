/*
Copyright 2025 The Dapr Authors
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

package dir

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dapr/kit/logger"
)

type Options struct {
	Log    logger.Logger
	Target string
}

// Dir atomically (best-effort on Windows) writes files to a given directory.
type Dir struct {
	log logger.Logger

	base      string
	target    string
	targetDir string

	// prev holds a path we should delete on the *next* successful Write.
	// On Unix: the previously active versioned dir.
	// On Windows: the last backup directory (target renamed aside).
	prev *string
}

func New(opts Options) *Dir {
	return &Dir{
		log:       opts.Log,
		base:      filepath.Dir(opts.Target),
		target:    opts.Target,
		targetDir: filepath.Base(opts.Target),
	}
}

func (d *Dir) Write(files map[string][]byte) error {
	newDir := filepath.Join(d.base, fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), d.targetDir))

	// Ensure base exists
	if err := os.MkdirAll(d.base, 0o700); err != nil {
		return err
	}
	// Create the new versioned directory
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return err
	}

	// Write all files into the new versioned directory
	for file, b := range files {
		path := filepath.Join(newDir, file)
		// Ensure parent directories exist for nested files
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return err
		}
		d.log.Infof("Written file %s", file)
	}

	// Platform-specific switch into place. It returns what we should delete on the NEXT run.
	nextPrev, err := d.switchTo(newDir)
	if err != nil {
		return err
	}

	d.log.Infof("Atomic write to %s", d.target)

	// Best-effort cleanup from the *previous* run
	if d.prev != nil {
		if err := os.RemoveAll(*d.prev); err != nil {
			return err
		}
	}

	// Set what to delete on the *next* run.
	d.prev = nextPrev

	return nil
}

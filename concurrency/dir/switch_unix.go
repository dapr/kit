//go:build !windows && !plan9

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
	"os"
)

func (d *Dir) switchTo(newDir string) (*string, error) {
	// Create a symlink and atomically rename it into place.
	tmpLink := d.target + ".new"

	// Remove any stale temp link
	_ = os.Remove(tmpLink)

	if err := os.Symlink(newDir, tmpLink); err != nil {
		return nil, err
	}
	d.log.Debugf("Symlink %s -> %s", tmpLink, newDir)

	// Atomically replace the target symlink (or create it if missing)
	// On POSIX, rename on the same filesystem is atomic.
	if err := os.Rename(tmpLink, d.target); err != nil {
		// Clean up temp link if rename fails
		_ = os.Remove(tmpLink)
		return nil, err
	}

	d.log.Debugf("Atomic write to %s", d.target)

	// On Unix we keep versioned dirs and delete the *previous* version on next run.
	return &newDir, nil
}

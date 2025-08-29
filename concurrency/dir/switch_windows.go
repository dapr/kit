//go:build windows

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
)

func (d *Dir) switchTo(newDir string) (*string, error) {
	// Windows notes:
	// - os.Rename does NOT replace existing directories.
	// - Directory symlinks/junctions are unreliable without privileges.
	// Strategy:
	//   1) If target exists, rename it to a timestamped backup alongside base.
	//   2) Rename newDir -> target.
	//   3) Return backup path so we delete it on the *next* run (avoids data loss if step 2 fails).
	var backup *string

	// If target exists, rename it aside
	if fi, err := os.Lstat(d.target); err == nil && fi.IsDir() {
		bak := filepath.Join(d.base, fmt.Sprintf("backup-%d-%s", time.Now().UTC().UnixNano(), d.targetDir))
		// Be defensive: remove any stale leftover
		_ = os.RemoveAll(bak)

		if err := os.Rename(d.target, bak); err != nil {
			return nil, err
		}
		d.log.Debugf("Renamed existing %s to backup %s", d.target, bak)
		backup = &bak
	}

	// Move the freshly written versioned dir into place as the new target
	if err := os.Rename(newDir, d.target); err != nil {
		// Try to restore the backup if we created one
		if backup != nil {
			_ = os.Rename(*backup, d.target)
		}
		return nil, err
	}

	d.log.Debugf("Replaced directory at %s (Windows best-effort atomicity)", d.target)

	// On Windows we delete the backup on the *next* run (so we don't risk losing data if a crash happens now).
	return backup, nil
}

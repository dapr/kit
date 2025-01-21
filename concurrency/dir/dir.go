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

type Dir struct {
	log logger.Logger

	base      string
	target    string
	targetDir string

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

	if err := os.MkdirAll(d.base, os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(newDir, os.ModePerm); err != nil {
		return err
	}

	for file, b := range files {
		path := filepath.Join(newDir, file)
		if err := os.WriteFile(path, b, os.ModePerm); err != nil {
			return err
		}
		d.log.Infof("Written file %s", file)
	}

	if err := os.Symlink(newDir, d.target+".new"); err != nil {
		return err
	}

	d.log.Infof("Syslink %s to %s.new", newDir, d.target)

	if err := os.Rename(d.target+".new", d.target); err != nil {
		return err
	}

	d.log.Infof("Atomic write to %s", d.target)

	if d.prev != nil {
		if err := os.RemoveAll(*d.prev); err != nil {
			return err
		}
	}

	d.prev = &newDir

	return nil
}

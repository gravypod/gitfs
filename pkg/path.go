// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkg

import (
	"errors"
	"path/filepath"
	"strings"
)

var (
	ErrEscapesChroot = errors.New("attempted to resolve path that escapes chroot")
)

const SeparatorString = string(filepath.Separator)

type GitPath struct {
	AbsolutePath string
}

func (p GitPath) Parent() GitPath {
	return GitPath{AbsolutePath: filepath.Dir(p.AbsolutePath)}
}

func (p GitPath) Resolve(request string) (GitPath, error) {
	absolutePath := filepath.Join(p.AbsolutePath, request)

	if !strings.HasPrefix(absolutePath, p.AbsolutePath) {
		return GitPath{}, ErrEscapesChroot
	}

	return GitPath{
		AbsolutePath: absolutePath,
	}, nil
}

func (p GitPath) IsRoot() bool {
	return p.AbsolutePath == SeparatorString
}

func (p GitPath) RootRelativePath() string {
	return filepath.Join(".", p.AbsolutePath)
}

// LazyRootRelativePath returns a relative path from the root of the repository
// that looks something like this: "foo/bar/baz.txt" or "." if we are pointing
// to the root directory. This is done because Billy expects these kinds of paths
// as input/output.
func (p GitPath) LazyRootRelativePath() string {
	path := strings.TrimPrefix(p.AbsolutePath, "/")
	if path == "" {
		return "."
	}
	return path
}

func RootGitPath() GitPath {
	return GitPath{
		AbsolutePath: SeparatorString,
	}
}

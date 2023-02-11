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
	Path         []string
	cachedString *string // String version of the Path.
}

func (p *GitPath) Parent() GitPath {
	if p.IsRoot() {
		return GitPath{}
	}
	return GitPath{Path: p.Path[:len(p.Path)-1]}
}

func (p *GitPath) Resolve(request string) (GitPath, error) {
	requestParts := strings.Split(request, SeparatorString)
	scratch := make([]string, len(p.Path)+len(requestParts))

	idx := 0

	for _, part := range p.Path {
		scratch[idx] = part
		idx += 1
	}

	for _, path := range requestParts {
		switch path {
		case "..":
			if idx == 0 {
				return GitPath{}, ErrEscapesChroot
			}
			idx -= 1
		case ".":
			continue
		default:
			scratch[idx] = path
			idx += 1
		}
	}

	return GitPath{
		Path: scratch[:idx],
	}, nil
}

func (p *GitPath) IsRoot() bool {
	return len(p.Path) == 0
}

func (p *GitPath) RootRelativePath() string {
	if p.cachedString == nil {
		allocated := filepath.Join(".", strings.Join(p.Path, SeparatorString))
		p.cachedString = &allocated
	}
	return *(p.cachedString)
}

func RootGitPath() GitPath {
	return GitPath{
		Path: nil,
	}
}

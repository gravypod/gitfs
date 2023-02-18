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
	"fmt"
	"github.com/gravypod/gitfs/pkg/gitism"
)

var (
	ErrNoTreeLikeSpecified   = errors.New("cannot identify tree")
	ErrCannotListCommit      = errors.New("cannot list commit")
	ErrMultipleRefsSpecified = errors.New("only specify Commit, Branch, or Tag")
)

type GitReference struct {
	Commit, Branch, Tag *string
}

func (p GitReference) treeLike() (string, error) {
	possible := []*string{
		p.Branch,
		p.Commit,
		p.Tag,
	}
	var selected *string
	for _, treeLike := range possible {
		if treeLike == nil {
			continue
		}

		if selected != nil {
			return "", ErrMultipleRefsSpecified
		}

		selected = treeLike
	}
	if selected == nil {
		return "", ErrNoTreeLikeSpecified
	}
	return *selected, nil
}

type GitPath struct {
	Reference GitReference
	TreePath  string
}

type Git interface {
	ListTree(path GitPath, handler func(entry gitism.TreeEntry) error) error
	ListBranches(handler func(branch string) error) error
	ListTags(handler func(branch string) error) error
	ListCommits(ref GitReference, handler func(branch string) error) error
	ReadBlob(hash string) ([]byte, error)
}

type cliGit struct {
	cli gitism.Command
}

func NewCliGit(gitDirectory string) (Git, error) {
	cli, err := gitism.NewCommand(gitDirectory)
	if err != nil {
		return nil, err
	}
	return cliGit{cli: cli}, nil
}

func (g cliGit) ListBranches(handler func(branch string) error) error {
	return g.cli.ListBranches(handler)
}

func (g cliGit) ListTags(handler func(branch string) error) error {
	return g.ListTags(handler)
}

func (g cliGit) ListCommits(ref GitReference, handler func(branch string) error) error {
	if ref.Commit != nil {
		return ErrCannotListCommit
	}
	treeLike, err := ref.treeLike()
	if err != nil {
		return err
	}
	return g.cli.ListCommits(treeLike, handler)
}

func (g cliGit) ListTree(path GitPath, handler func(entry gitism.TreeEntry) error) error {
	treeLike, err := path.Reference.treeLike()
	if err != nil {
		return fmt.Errorf("please provide a Commit, Tag, or Branch: %v", err)
	}
	return g.cli.LsTree(treeLike, path.TreePath, handler)
}

func (g cliGit) ReadBlob(hash string) ([]byte, error) {
	return g.cli.CatFile("blob", hash)
}

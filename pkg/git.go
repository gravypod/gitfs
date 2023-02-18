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
	"bufio"
	"errors"
	"fmt"
	"github.com/gravypod/gitfs/pkg/gitism"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrNoTreeLikeSpecified   = errors.New("cannot identify tree")
	ErrCannotListCommit      = errors.New("cannot list commit")
	ErrMultipleRefsSpecified = errors.New("only specify Commit, Branch, or Tag")
)

type GitUnixPerms uint16

func (perms GitUnixPerms) String() string {
	return strconv.FormatUint(uint64(perms), 8)
}

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

type ListTreeEntry struct {
	Mode   gitism.FileMode
	Object string
	Hash   string
	Size   string
	Path   string
}

func newListTreeEntry(line string) (ListTreeEntry, error) {
	modeTextEnd := strings.IndexByte(line, ' ')
	if modeTextEnd == -1 {
		return ListTreeEntry{}, fmt.Errorf("oct not found in: %s", line)
	}
	modeText := line[:modeTextEnd]
	line = line[modeTextEnd+1:]
	objectTypeTextEnd := strings.IndexByte(line, ' ')
	objectTypeText := line[:objectTypeTextEnd]
	if objectTypeTextEnd == -1 {
		return ListTreeEntry{}, fmt.Errorf("type not found in: %s", line)
	}
	line = line[objectTypeTextEnd+1:]
	hashTextEnd := strings.IndexByte(line, ' ')
	if hashTextEnd == -1 {
		return ListTreeEntry{}, fmt.Errorf("hash not found in: %s", line)
	}
	hashText := line[:hashTextEnd]
	line = line[len(hashText)+1:]
	sizeTextEnd := strings.LastIndexByte(line, '\t')
	if sizeTextEnd == -1 {
		return ListTreeEntry{}, fmt.Errorf("size not found in: %s", line)
	}
	sizeText := line[:sizeTextEnd]
	pathText := line[len(sizeText)+1:]

	modeNum, err := strconv.ParseUint(modeText, 8, 16)
	if err != nil {
		return ListTreeEntry{}, err
	}

	return ListTreeEntry{
		Mode:   gitism.NewFileMode(uint16(modeNum)),
		Object: strings.TrimSpace(objectTypeText),
		Hash:   strings.TrimSpace(hashText),
		Size:   strings.TrimSpace(sizeText),
		Path:   strings.TrimSpace(pathText),
	}, nil
}

type Git interface {
	ListTree(path GitPath, handler func(entry ListTreeEntry) error) error
	ListBranches(handler func(branch string) error) error
	ListTags(handler func(branch string) error) error
	ListCommits(ref GitReference, handler func(branch string) error) error
	ReadBlob(hash string) ([]byte, error)
}

type cliGit struct {
	gitDirectory  string
	gitBinaryPath string
}

func NewCliGit(gitDirectory string) (Git, error) {
	gitBinaryPath, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	gitDirectory, err = filepath.Abs(gitDirectory)
	if err != nil {
		return nil, err
	}
	return cliGit{gitDirectory: gitDirectory, gitBinaryPath: gitBinaryPath}, nil
}

func (g cliGit) execute(args ...string) *exec.Cmd {
	modifiedArgs := append([]string{
		"--git-dir", g.gitDirectory,
	}, args...)
	cmd := exec.Command("git", modifiedArgs...)
	log.Printf("Execute() returning %s\n", cmd.String())
	return cmd
}

func (g cliGit) ListBranches(handler func(branch string) error) error {
	cmd := g.execute("branch", "--all")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("coult not pipe branch list: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to list branches: %v", err)
	}
	defer cmd.Wait()

	reader := bufio.NewScanner(stdout)
	for reader.Scan() {
		line := reader.Text()

		// The "selected" branch is printed like this:
		//  " * main"
		// Before we go forward we need to remove the `*` character.
		if index := strings.IndexRune(line, '*'); index != -1 {
			line = line[index+1:]
		}

		line = strings.TrimSpace(line)

		err = handler(line)
		if err != nil {
			return fmt.Errorf("failed to process branch '%s': %v", line, err)
		}
	}

	return nil
}

func (g cliGit) ListTags(handler func(branch string) error) error {
	cmd := g.execute("tag", "--all")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("coult not pipe tag list: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to list tags: %v", err)
	}
	defer cmd.Wait()

	reader := bufio.NewScanner(stdout)
	for reader.Scan() {
		line := reader.Text()
		line = strings.TrimSpace(line)

		err = handler(line)
		if err != nil {
			return fmt.Errorf("failed to process tag '%s': %v", line, err)
		}
	}

	return nil
}

func (g cliGit) ListCommits(ref GitReference, handler func(branch string) error) error {
	if ref.Commit != nil {
		return ErrCannotListCommit
	}
	treeLike, err := ref.treeLike()
	if err != nil {
		return err
	}
	cmd := g.execute("log", "--pretty=format:'%h'", "--abbrev=-1", treeLike)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("coult not pipe commit list: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed `git log`: %v", err)
	}
	defer cmd.Wait()

	reader := bufio.NewScanner(stdout)
	for reader.Scan() {
		line := reader.Text()
		line = strings.TrimSpace(line)

		err = handler(line)
		if err != nil {
			return fmt.Errorf("failed to process commit '%s': %v", line, err)
		}
	}

	return nil
}

func (g cliGit) ListTree(path GitPath, handler func(entry ListTreeEntry) error) error {
	treeLike, err := path.Reference.treeLike()
	if err != nil {
		return fmt.Errorf("please provide a Commit, Tag, or Branch: %v", err)
	}
	// TODO(gravypod): Support listing multiple revisions.
	cmd := g.execute(
		"ls-tree",
		"--long",      // Include blob size
		treeLike,      // revision to list from. Can be a remote ref, branch, tag, etc. Anything tree-like.
		path.TreePath, // File path to list
	)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not read ls-tree output for path '%s': %v", path.TreePath, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to list path '%s': %v", path.TreePath, err)
	}
	defer cmd.Wait()

	reader := bufio.NewScanner(stdout)
	for reader.Scan() {
		line := reader.Text()

		// TODO(gravypod): Support --long to include file sizes
		entry, err := newListTreeEntry(line)
		if err != nil {
			return fmt.Errorf("failed to parse ls-tree line: %v", err)
		}

		err = handler(entry)
		if err != nil {
			return fmt.Errorf("handler rejected file info: %v", err)
		}
	}

	return nil
}

func (g cliGit) ReadBlob(hash string) ([]byte, error) {
	cmd := g.execute(
		"cat-file",
		"blob",
		hash, // File path to list
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return []byte{}, err
	}

	if err := cmd.Start(); err != nil {
		return []byte{}, err
	}
	defer cmd.Wait()

	contents, err := io.ReadAll(stdout)
	if err != nil {
		return []byte{}, err
	}
	return contents, nil
}

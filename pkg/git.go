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
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ListTreeEntry struct {
	Mode   string
	Object string
	Hash   string
	Size   string
	Path   string
}

func NewListTreeEntry(line string) (ListTreeEntry, error) {
	modeTextEnd := strings.IndexByte(line, ' ')
	if modeTextEnd == -1 {
		return ListTreeEntry{}, fmt.Errorf("mode not found in: %s", line)
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

	return ListTreeEntry{
		Mode:   strings.TrimSpace(modeText),
		Object: strings.TrimSpace(objectTypeText),
		Hash:   strings.TrimSpace(hashText),
		Size:   strings.TrimSpace(sizeText),
		Path:   strings.TrimSpace(pathText),
	}, nil
}

type Git interface {
	ListTree(treeLike, rootRelativePath string, handler func(entry ListTreeEntry) error) error
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

func (g cliGit) InitBare() error {
	// TODO(gravypod): Move away from `master` as the main branch. Allow users to
	// to configure this on their own.
	cmd := g.execute("init", "--bare")
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func (g cliGit) AddAll(workspace string) error {
	cmd := g.execute("--work-tree", workspace, "add", "--all")
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func (g cliGit) Commit(message string, worktree string) error {
	cmd := g.execute("--work-tree", worktree, "commit", "-m", message)
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func (g cliGit) MakeMirroredRepository(message string, worktree string) error {
	err := g.InitBare()
	if err != nil {
		return err
	}

	worktree, err = filepath.Abs(worktree)
	if err != nil {
		return fmt.Errorf("couldn't find worktree: %v", err)
	}
	err = g.AddAll(worktree)
	if err != nil {
		return fmt.Errorf("couldn't create structure: %v", err)
	}
	return g.Commit(message, worktree)
}

func (g cliGit) ListTree(treeLike, rootRelativePath string, handler func(entry ListTreeEntry) error) error {
	// TODO(gravypod): Support listing multiple revisions.
	cmd := g.execute(
		"ls-tree",
		"--long",         // Include blob size
		treeLike,         // revision to list from. Can be a remote ref, branch, tag, etc. Anything tree-like.
		rootRelativePath, // File path to list
	)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not read ls-tree output for path '%s': %v", rootRelativePath, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to list path '%s': %v", rootRelativePath, err)
	}
	defer cmd.Wait()

	reader := bufio.NewScanner(stdout)
	for reader.Scan() {
		line := reader.Text()

		// TODO(gravypod): Support --long to include file sizes
		entry, err := NewListTreeEntry(line)
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

	contents, err := ioutil.ReadAll(stdout)
	if err != nil {
		return []byte{}, err
	}
	return contents, nil
}

// Below we have some functions that take the git file mode and turn them into fs.FileMode objects and perform other
// checks against them. More details are available here: https://unix.stackexchange.com/a/450488

func ParseGitFileMode(gitMode uint16) fs.FileMode {
	// Unixy file permissions are stored in the last 9 bits.
	var (
		gitPermsMask     uint16 = 0000777
		gitDirectoryMask uint16 = 0040000
		gitSymlinkMask   uint16 = 0120000
		gitLinkMask      uint16 = 0160000
	)

	fileMode := fs.FileMode(gitMode & gitPermsMask)

	if gitMode&gitSymlinkMask == gitSymlinkMask || gitMode&gitLinkMask == gitLinkMask {
		fileMode |= fs.ModeSymlink
	} else if gitMode&gitDirectoryMask == gitDirectoryMask {
		// Git does not store permissions for directories so we need
		// to add these back in. 444 means user, group, and other can
		// read which essentially makes this a read-only directory.
		fileMode = fs.ModeDir | fs.FileMode(0444)
	}

	return fileMode
}

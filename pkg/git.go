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
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type GitUnixPerms uint16

const (
	GitRegularFile GitFileType = iota
	GitDirectory
	GitSymlink
)

func (perms GitUnixPerms) String() string {
	return strconv.FormatUint(uint64(perms), 8)
}

type GitFileType uint8

func (fileType GitFileType) String() string {
	if fileType == GitSymlink {
		return "GitSymlink"
	}
	if fileType == GitDirectory {
		return "GitDirectory"
	}
	return "GitRegularFile"
}

type GitFileMode struct {
	Type            GitFileType
	UnixPermissions GitUnixPerms
}

// newGitFileMode takes a git file mode oct and turns it into fs.FileMode objects. It performs other fixes to the file
// mode to hack around edge cases in git. More details are available here: https://unix.stackexchange.com/a/450488
func newGitFileMode(gitMode uint16) GitFileMode {
	// Unixy file permissions are stored in the last 9 bits.
	var (
		gitPermsMask     uint16 = 0000777
		gitDirectoryMask uint16 = 0040000
		gitSymlinkMask   uint16 = 0120000
		gitLinkMask      uint16 = 0160000
	)

	mode := GitFileMode{
		UnixPermissions: GitUnixPerms(gitMode & gitPermsMask),
	}

	if gitMode&gitSymlinkMask == gitSymlinkMask || gitMode&gitLinkMask == gitLinkMask {
		mode.Type = GitSymlink
	} else if gitMode&gitDirectoryMask == gitDirectoryMask {
		// Git does not store permissions for directories so we need
		// to add these back in. 444 means user, group, and other can
		// read which essentially makes this a read-only directory.
		mode.Type = GitDirectory
		mode.UnixPermissions = 0444
	} else {
		mode.Type = GitRegularFile
	}

	return mode
}

type ListTreeEntry struct {
	Mode   GitFileMode
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
		Mode:   newGitFileMode(uint16(modeNum)),
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

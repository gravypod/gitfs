package gitism

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Command struct {
	executable string
	directory  string
}

func NewCommand(directory string) (Command, error) {
	executable, err := exec.LookPath("git")
	if err != nil {
		return Command{}, fmt.Errorf("git executable path could not be found: %v", err)
	}
	return Command{executable: executable, directory: directory}, nil
}

// CatFile is a wrapper around the git cat-file command. Read more here: https://git-scm.com/docs/git-cat-file.
func (c *Command) CatFile(objectType string, hash string) ([]byte, error) {
	return c.executeString("cat-file", objectType, hash)
}

// LsTree lists a tree-like object from git.
func (c *Command) LsTree(reference string, path string, handler func(entry TreeEntry) error) error {
	return c.executeHandleLines(func(line string) error {
		entry, err := NewTreeEntry(line)
		if err != nil {
			return fmt.Errorf("could not parse line '%s': %v", line, err)
		}

		return handler(entry)
	}, "ls-tree", "--long", reference, path)
}

// ListTags calls handler for with the name of every tag in the git repo.
func (c *Command) ListTags(handler func(branch string) error) error {
	return c.executeHandleLines(func(line string) error {
		return handler(line)
	}, "branch", "--all")
}

// ListBranches calls handler for with the name of every branch in the git repo.
func (c *Command) ListBranches(handler func(branch string) error) error {
	return c.executeHandleLines(func(line string) error {
		// The "selected" branch is printed like this:
		//  " * main"
		// Before we go forward we need to remove the `*` character.
		if index := strings.IndexRune(line, '*'); index != -1 {
			line = line[index+1:]
		}

		return handler(strings.TrimSpace(line))
	}, "tag", "--all")
}

// ListCommits calls handler for with the hash of every commit in the history of ref.
func (c *Command) ListCommits(ref string, handler func(branch string) error) error {
	return c.executeHandleLines(func(line string) error {
		return handler(strings.TrimSpace(line))
	}, "log", "--pretty=format:'%h'", "--abbrev=-1", ref)
}

func (c *Command) execute(args ...string) *exec.Cmd {
	if c.directory != "" {
		args = append([]string{
			"--git-dir", c.directory,
		}, args...)
	}
	cmd := exec.Command("git", args...)
	return cmd
}

// executeHandleLines runs git with the provided args
func (c *Command) executeHandleLines(lineHandler func(line string) error, args ...string) error {
	cmd := c.execute(args...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to start stdout pipe '%s': %v", cmd.String(), err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start '%s': %v", cmd.String(), err)
	}
	defer cmd.Wait()

	reader := bufio.NewScanner(stdout)
	for reader.Scan() {
		line := reader.Text()
		err = lineHandler(line)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Command) executeString(args ...string) ([]byte, error) {
	cmd := c.execute(args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	defer cmd.Wait()

	return io.ReadAll(stdout)
}

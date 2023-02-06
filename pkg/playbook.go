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

// Test-only library for creating git repositories from a script. These scripts live in testdata/playbooks/ and are
// executed within temp directories which are removed after tests are finished.

package pkg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// runPlaybook creates a new git repo in "tmp" by executing "playbook" as a
// shell script.
func runPlaybook(playbook, tmp string) (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("could not find testdata directory")
	}

	baseDirectory := filepath.Dir(filename)

	script := filepath.Join(baseDirectory, "testdata", "playbooks", playbook+".sh")

	cmd := exec.Command(script)

	// Set CWD to where we want to store tmp files.
	cmd.Dir = tmp
	// Pass all environment variables to subprocess
	cmd.Env = os.Environ()

	// Pipe stderr to our own.
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return "", err
	}

	err = cmd.Wait()
	if err != nil {
		return "", err
	}

	return filepath.Join(tmp, ".git"), nil
}

func newGitCliFromPlaybook(t *testing.T, playbook string) Git {
	tmp := t.TempDir()

	bareRepoPath, err := runPlaybook(playbook, tmp)
	if err != nil {
		t.Fatalf("playbook '%s' failed: %v", playbook, err)
	}
	friendlyGit, err := NewCliGit(bareRepoPath)
	if err != nil {
		t.Fatal(err)
	}

	return friendlyGit
}

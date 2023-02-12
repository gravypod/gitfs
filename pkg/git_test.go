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
	"github.com/google/go-cmp/cmp"
	"sort"
	"testing"
)

var BranchMaster = "master"

func TestListing(t *testing.T) {
	git := newGitCliFromPlaybook(t, "base")

	want := []ListTreeEntry{
		{
			Mode: GitFileMode{
				Type:            GitRegularFile,
				UnixPermissions: GitUnixPerms(0755),
			},
			Object: "blob",
			Hash:   "2266c0a976d1b3c4df0b6d02217d1bbe11110693",
			Size:   "633",
			Path:   "executable.sh",
		},
		{
			Mode: GitFileMode{
				Type:            GitRegularFile,
				UnixPermissions: GitUnixPerms(0644),
			},
			Object: "blob",
			Hash:   "557db03de997c86a4a028e1ebd3a1ceb225be238",
			Size:   "12",
			Path:   "real.txt",
		},
		{
			Mode: GitFileMode{
				Type:            GitSymlink,
				UnixPermissions: GitUnixPerms(0),
			},
			Object: "blob",
			Hash:   "c9c61fe1fb4b3bbadb18744348069f1cb5aa7416",
			Size:   "8",
			Path:   "symlink.txt",
		},
		{
			Mode: GitFileMode{
				Type:            GitDirectory,
				UnixPermissions: GitUnixPerms(0444),
			},
			Object: "tree",
			Hash:   "4e59bddb9f480a1b6d0041c534b5c53a5921dd52",
			Size:   "-",
			Path:   "test",
		},
	}

	var got []ListTreeEntry

	gitPath := GitPath{
		Reference: GitReference{Branch: &BranchMaster},
		TreePath:  ".",
	}
	err := git.ListTree(gitPath, func(entry ListTreeEntry) error {
		got = append(got, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to list main branch: %v", err)
	}

	trans := cmp.Transformer("Sort", func(in []ListTreeEntry) []ListTreeEntry {
		out := append([]ListTreeEntry(nil), in...) // Copy input to avoid mutating it
		sort.Slice(out, func(i, j int) bool {
			return out[i].Path < out[j].Path
		})
		return out
	})

	if diff := cmp.Diff(want, got, trans); diff != "" {
		t.Fatal(diff)
	}
}

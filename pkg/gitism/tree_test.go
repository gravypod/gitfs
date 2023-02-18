package gitism

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestTree(t *testing.T) {
	line := "100644 blob c64211fac0a777ffada0af11bd64ca20e6289d7c    3500    README.md"
	tree, err := NewTreeEntry(line)
	if err != nil {
		t.Fatalf("could not parse valid tree: %v", err)
	}

	want := TreeEntry{
		Mode: FileMode{
			Type:  RegularFile,
			Perms: PermissionMask(0644),
		},
		Object: BlobObject,
		Hash:   "c64211fac0a777ffada0af11bd64ca20e6289d7c",
		Size:   "3500",
		Path:   "README.md",
	}
	if diff := cmp.Diff(want, tree); diff != "" {
		t.Fatal(diff)
	}
}

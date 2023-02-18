package gitism

// ChangeType describes the type of modification done to the file in the git index. A full enumerations of types can be
// found on git's docs: https://git-scm.com/docs/git-diff-tree#:~:text=Possible%20status%20letters%20are
type ChangeType uint8

const (
	// ChangeUnknown represents X the change type from git-diff-tree.
	ChangeUnknown ChangeType = iota
	// ChangeAddition represents A the change type from git-diff-tree.
	ChangeAddition
	// ChangeCopy represents C the change type from git-diff-tree.
	ChangeCopy
	// ChangeDeletion represents D the change type from git-diff-tree.
	ChangeDeletion
	// ChangeModification represents M the change type from git-diff-tree.
	ChangeModification
	// ChangeRename represents R the change type from git-diff-tree.
	ChangeRename
	// ChangeFileType represents T the change type from git-diff-tree.
	ChangeFileType
	// ChangeUnmerged represents U the change type from git-diff-tree.
	ChangeUnmerged
)

// ChangeHashMissing is used by git to represent when a file cannot have a hash defined. The logic for when this is used
// is defined in https://git-scm.com/docs/git-diff-tree#_raw_output_format.
const ChangeHashMissing = "0000000000000000000000000000000000000000"

// Change describes a modification to a single file. Refer to git's documentation about what is possible to include:
// https://git-scm.com/docs/git-diff-tree#_raw_output_format
type Change struct {
	Type               ChangeType
	PreviousHash, Hash string // The previous hash of the file and the new hash of the file.
	PreviousMode, Mode FileMode
	Path               string
}

type Commit struct {
	Hash    string
	Changes []Change
}

package gitism

type FileType uint8

const (
	RegularFile FileType = iota
	Directory
	Symlink
)

// FileMode is a struct representing the "type" of a file in the git repo. This is a tuple of FileType and PermissionMask.
type FileMode struct {
	Type  FileType
	Perms PermissionMask
}

// NewFileMode takes a git file mode oct and turns it into fs.FileMode objects. It performs other fixes to the file
// mode to hack around edge cases in git. More details are available here: https://unix.stackexchange.com/a/450488
func NewFileMode(gitMode uint16) FileMode {
	// Unixy file permissions are stored in the last 9 bits.
	const (
		gitPermsMask     uint16 = 0000777
		gitDirectoryMask uint16 = 0040000
		gitSymlinkMask   uint16 = 0120000
		gitLinkMask      uint16 = 0160000
	)

	mode := FileMode{
		Perms: PermissionMask(gitMode & gitPermsMask),
	}

	if gitMode&gitSymlinkMask == gitSymlinkMask || gitMode&gitLinkMask == gitLinkMask {
		mode.Type = Symlink
	} else if gitMode&gitDirectoryMask == gitDirectoryMask {
		// Git does not store permissions for directories so we need
		// to add these back in. 444 means user, group, and other can
		// read which essentially makes this a read-only directory.
		mode.Type = Directory
		mode.Perms = 0444
	} else {
		mode.Type = RegularFile
	}

	return mode
}

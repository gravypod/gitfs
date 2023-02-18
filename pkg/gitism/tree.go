package gitism

import (
	"strconv"
	"strings"
	"unicode"
)

type TreeEntry struct {
	Mode   FileMode
	Object ObjectType
	Hash   string
	Size   string
	Path   string
}

func NewTreeEntry(lsTreeLine string) (TreeEntry, error) {
	// We will parse a line in this format:
	// "100644 blob c64211fac0a777ffada0af11bd64ca20e6289d7c    3500    README.md"
	//  012345|7890|2345678901234567890123456789012345678901|
	//  6,     4,   40

	modeText := lsTreeLine[0:6]
	typeText := lsTreeLine[7:11]
	hashText := lsTreeLine[12:52]

	mode, err := strconv.ParseUint(modeText, 8, 16)
	if err != nil {
		return TreeEntry{}, err
	}

	remainder := strings.TrimSpace(lsTreeLine[52:])

	// <size> and <path> are seperated by a tab character
	nextWhiteSpace := strings.IndexFunc(remainder, unicode.IsSpace)
	size := remainder[:nextWhiteSpace]
	path := strings.TrimSpace(remainder[nextWhiteSpace+1:])

	entry := TreeEntry{
		Mode:   NewFileMode(uint16(mode)),
		Object: NewObjectType(typeText),
		Hash:   hashText,
		Size:   size,
		Path:   path,
	}
	return entry, nil
}

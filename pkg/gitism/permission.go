package gitism

import "strconv"

// PermissionMask represents the underlying representation git uses for defining permissions. This format is described
// in this stackoverflow post: https://unix.stackexchange.com/a/450488
type PermissionMask uint16

func (mask PermissionMask) String() string {
	return strconv.FormatUint(uint64(mask), 8)
}

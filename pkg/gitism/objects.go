package gitism

type ObjectType uint8

const (
	UnknownObjectType ObjectType = iota
	BlobObject
	TreeObject
)

func NewObjectType(name string) ObjectType {
	switch name {
	case "blob":
		return BlobObject
	case "tree":
		return TreeObject
	default:
		return UnknownObjectType
	}
}

func (t ObjectType) String() string {
	switch t {
	case BlobObject:
		return "blob"
	case TreeObject:
		return "tree"
	default:
		return "unknown-object"
	}
}

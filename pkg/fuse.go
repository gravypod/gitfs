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
	"container/list"
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var latest time.Time = time.Unix(1<<63-62135596801, 999999999)

type billyInode struct {
	Id       fuseops.InodeID
	ParentId fuseops.InodeID
	info     os.FileInfo
	Children []fuseops.InodeID
}

type billyFuse struct {
	fuseutil.NotImplementedFileSystem

	inodes  map[fuseops.InodeID]*billyInode
	handles map[fuseops.HandleID]billy.File
	fs      billy.Filesystem
}

func (f *billyFuse) getInode(id fuseops.InodeID) (*billyInode, error) {
	if id == 0 {
		// Zero is not a valid node id
		return nil, fuse.EINVAL
	}

	inode, ok := f.inodes[id]
	if !ok {
		return nil, fuse.ENOENT
	}
	return inode, nil
}

func NewBillyFuse(fs billy.Filesystem) (fuseutil.FileSystem, error) {
	billyFuse := new(billyFuse)
	billyFuse.inodes = map[fuseops.InodeID]*billyInode{}
	billyFuse.handles = map[fuseops.HandleID]billy.File{}
	billyFuse.fs = fs

	type queuedPath struct {
		parentInodeId fuseops.InodeID
		path          string
	}

	nextInode := fuseops.RootInodeID
	createInode := func(info os.FileInfo) *billyInode {
		node := new(billyInode)

		node.Id = fuseops.InodeID(nextInode)
		nextInode += 1

		node.info = info
		node.Children = []fuseops.InodeID{}
		billyFuse.inodes[node.Id] = node
		return node
	}

	queue := list.New()
	queue.PushBack(queuedPath{
		parentInodeId: 0,
		path:          ".",
	})
	for queue.Len() > 0 {
		front := queue.Front()
		next := (front.Value).(queuedPath)
		currentDirectory := next.path
		queue.Remove(front)

		fileInfo, err := fs.Stat(currentDirectory)
		if err != nil {
			return nil, fmt.Errorf("failed to stat directory %s: %v", currentDirectory, err)
		}
		directoryInode := createInode(fileInfo)

		if next.parentInodeId != 0 {
			parentInode, ok := billyFuse.inodes[next.parentInodeId]
			if ok {
				parentInode.Children = append(parentInode.Children, directoryInode.Id)
			}
			directoryInode.ParentId = next.parentInodeId
		}

		files, err := fs.ReadDir(currentDirectory)
		if err != nil {
			return nil, fmt.Errorf("failed to read dir %s: %v", currentDirectory, err)
		}

		for _, file := range files {
			if file.IsDir() {
				queue.PushBack(queuedPath{
					parentInodeId: directoryInode.Id,
					path:          filepath.Join(currentDirectory, file.Name()),
				})
				continue
			}

			fileInode := createInode(file)
			fileInode.ParentId = directoryInode.Id
			directoryInode.Children = append(directoryInode.Children, fileInode.Id)
		}
	}

	return billyFuse, nil
}

func NewBillyFuseServer(fs billy.Filesystem) (fuse.Server, error) {
	fuseFileSystem, err := NewBillyFuse(fs)
	if err != nil {
		return nil, err
	}
	return fuseutil.NewFileSystemServer(fuseFileSystem), nil
}

func (f *billyFuse) findChildInode(parent fuseops.InodeID, name string) (fuseops.InodeID, error) {
	log.Println("fuse findChildInode()")
	inode, err := f.getInode(parent)
	if err != nil {
		return 0, fuse.EEXIST
	}
	if !inode.info.IsDir() {
		return 0, fuse.ENOTDIR
	}
	for _, childId := range inode.Children {
		inode, err = f.getInode(childId)
		if err != nil {
			continue
		}
		if inode.info.Name() == name {
			return childId, nil
		}
	}
	return 0, fuse.ENOENT
}

func infoToAttributes(info os.FileInfo) fuseops.InodeAttributes {
	log.Println("fuse infoToAttributes()")
	mode := info.Mode()
	if mode.IsDir() {
		// make directories readable
		mode = os.ModeDir | os.FileMode(0444)
	}
	modificationTime := info.ModTime()
	attributes := fuseops.InodeAttributes{
		Size:   uint64(info.Size()),
		Nlink:  1,
		Mode:   mode,
		Atime:  modificationTime,
		Mtime:  modificationTime,
		Ctime:  modificationTime,
		Crtime: modificationTime,
		Uid:    0,
		Gid:    0,
	}
	log.Printf("%s attributes -> %v. Mode: %s", info.Name(), attributes, mode.String())
	return attributes
}

func (f *billyFuse) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	log.Println("fuse LookUpInode()")
	// Find the child within the parent.
	childId, err := f.findChildInode(op.Parent, op.Name)
	if err != nil {
		return err
	}

	inode, err := f.getInode(childId)
	if err != nil {
		return fuse.ENOENT
	}

	// Copy over information.
	op.Entry.Child = childId
	op.Entry.Attributes = infoToAttributes(inode.info)
	op.Entry.AttributesExpiration = latest
	op.Entry.EntryExpiration = latest

	return nil
}

func (f *billyFuse) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	log.Println("fuse GetInodeAttributes()")
	inode, err := f.getInode(op.Inode)
	if err != nil {
		return fuse.ENOENT
	}
	op.Attributes = infoToAttributes(inode.info)
	op.AttributesExpiration = latest
	return nil
}

func (f *billyFuse) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	log.Println("fuse ReadDir()")
	inode, err := f.getInode(op.Inode)
	if err != nil {
		return fuse.ENOENT
	}

	if !inode.info.IsDir() {
		return fuse.ENOTDIR
	}

	var entries []fuseutil.Dirent
	offset := 0
	for _, child := range inode.Children {
		childInode, err := f.getInode(child)
		if err != nil {
			return fuse.EIO
		}
		offset += 1

		entType := fuseutil.DT_Unknown
		mode := childInode.info.Mode()
		if mode&os.ModeDir != 0 {
			entType = fuseutil.DT_Directory
		} else if mode&os.ModeSymlink != 0 {
			entType = fuseutil.DT_Link
		} else {
			entType = fuseutil.DT_File
		}

		entries = append(entries, fuseutil.Dirent{
			Offset: fuseops.DirOffset(offset),
			Inode:  child,
			Name:   childInode.info.Name(),
			Type:   entType,
		})
	}

	// Grab the range of interest.
	if op.Offset > fuseops.DirOffset(len(entries)) {
		return fuse.EIO
	}

	entries = entries[op.Offset:]

	// Resume at the specified offset into the array.
	for _, e := range entries {
		n := fuseutil.WriteDirent(op.Dst[op.BytesRead:], e)
		if n == 0 {
			break
		}

		op.BytesRead += n
	}

	return nil
}

func (f *billyFuse) getBillyPath(inodeId fuseops.InodeID) (string, error) {
	log.Println("fuse getBillyPath()")
	inode, err := f.getInode(inodeId)
	if err != nil {
		return "", fuse.EIO
	}

	path := ""
	for inode.Id != fuseops.RootInodeID {
		path = f.fs.Join(inode.info.Name(), path)

		inode, err = f.getInode(inode.ParentId)
		if err != nil {
			return "", err
		}
	}
	return f.fs.Join(".", path), nil
}

func (f *billyFuse) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	log.Println("fuse ReadFile()")
	path, err := f.getBillyPath(op.Inode)
	if err != nil {
		return err
	}

	handle, err := f.fs.Open(path)
	if err != nil {
		return fuse.EIO
	}

	bytesRead, err := handle.ReadAt(op.Dst, op.Offset)
	op.BytesRead = bytesRead

	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (f *billyFuse) StatFS(ctx context.Context, op *fuseops.StatFSOp) error {
	log.Println("fuse StatFS()")
	_ = ctx
	_ = op
	return nil
}

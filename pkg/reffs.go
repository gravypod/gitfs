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
	"bytes"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/gravypod/gitfs/pkg/gitism"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type GitObjectType int

const (
	GitUnknown GitObjectType = iota
	GitBlob
	GitTree
)

type gitFileInfo struct {
	mode os.FileMode
	Type GitObjectType
	// TODO(gravypod): should this be parsed into an int or is this a waste of cycles?
	Hash string

	// TODO(gravypod): Should we only store the basename and make the "owner" of this path
	//                 handle the parent dirs? This could save memory
	path string

	size uint32
}

func (i gitFileInfo) Name() string {
	return filepath.Base(i.path)
}

func (i gitFileInfo) Size() int64 {
	return int64(i.size)
}

func (i gitFileInfo) Mode() fs.FileMode {
	return i.mode
}

func (i gitFileInfo) ModTime() time.Time {
	return time.Unix(0, 0)
}

func (i gitFileInfo) IsDir() bool {
	return i.Mode().IsDir()
}

func (i gitFileInfo) Sys() interface{} {
	return nil
}

type gitFile struct {
	name     string
	fs       ReferenceFileSystem
	info     gitFileInfo
	contents []byte
	reader   *bytes.Reader
}

func (f gitFile) Name() string {
	return filepath.Base(f.name)
}

func (f gitFile) Write(p []byte) (n int, err error) {
	_ = p
	return 0, billy.ErrNotSupported
}

func (f gitFile) Read(p []byte) (n int, err error) {
	return f.reader.Read(p)
}

func (f gitFile) ReadAt(p []byte, off int64) (n int, err error) {
	return f.reader.ReadAt(p, off)
}

func (f gitFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}

func (f gitFile) Close() error {
	return nil
}

func (f gitFile) Lock() error {
	return billy.ErrNotSupported
}

func (f gitFile) Unlock() error {
	return billy.ErrNotSupported
}

func (f gitFile) Truncate(size int64) error {
	_ = size
	return billy.ErrNotSupported
}

type ReferenceFileSystem struct {
	git       Git
	reference GitReference
	// Either an empty string or a path to a directory with the repository.
	root FilePath
}

func NewReferenceFileSystem(git Git, reference GitReference) billy.Filesystem {
	return ReferenceFileSystem{
		git:       git,
		reference: reference,
		root:      RootGitPath(),
	}
}

func (s ReferenceFileSystem) openFile(filename string, fileInfo gitFileInfo) (billy.File, error) {
	contents, err := s.git.ReadBlob(fileInfo.Hash)
	if err != nil {
		return nil, err
	}

	file := gitFile{
		name:     filename,
		fs:       s,
		info:     fileInfo,
		contents: contents,
	}
	file.reader = bytes.NewReader(file.contents)

	return file, nil
}

func (s ReferenceFileSystem) lsTree(path FilePath, children bool, handler func(file gitFileInfo) error) error {
	relativePath := path.String()
	// We want to list the contents of this tree (aka list the contents of a directory) so we need to
	// append a trailing path otherwise ls-tree will just print the tree's metadata.
	if children {
		relativePath += SeparatorString
	}

	branch := "master"
	gitPath := GitPath{
		Reference: GitReference{
			Branch: &branch,
		},
		TreePath: relativePath,
	}

	return s.git.ListTree(gitPath, func(entry ListTreeEntry) error {
		file := gitFileInfo{
			Hash: entry.Hash,
			path: entry.Path,
			size: 0,
		}

		// Type
		var typeMap = map[string]GitObjectType{
			"blob": GitBlob,
			"tree": GitTree,
		}
		if objectType, ok := typeMap[entry.Object]; ok {
			file.Type = objectType
		} else {
			objectType = GitUnknown
		}

		// Mode
		file.mode = fs.FileMode(entry.Mode.Perms)
		if entry.Mode.Type == gitism.Symlink {
			file.mode |= fs.ModeSymlink
		} else if entry.Mode.Type == gitism.Directory {
			file.mode |= fs.ModeDir
		}

		// Size
		if entry.Size != "-" {
			parsedSize, err := strconv.ParseUint(entry.Size, 10, 32)
			if err != nil {
				return err
			}
			file.size = uint32(parsedSize)
		}

		return handler(file)
	})
}

func (s ReferenceFileSystem) lsFile(path FilePath) (gitFileInfo, error) {
	seen := false
	var returnedPath gitFileInfo
	err := s.lsTree(path, false, func(file gitFileInfo) error {
		if seen {
			return fs.ErrInvalid
		}
		returnedPath = file
		seen = true
		return nil
	})
	if err != nil {
		return gitFileInfo{}, nil
	}
	if !seen {
		return gitFileInfo{}, fs.ErrNotExist
	}
	return returnedPath, nil
}

// billy.Basic type implementation

func (s ReferenceFileSystem) Create(filename string) (billy.File, error) {
	_ = filename
	return nil, billy.ErrReadOnly
}

func (s ReferenceFileSystem) Open(filename string) (billy.File, error) {
	log.Printf("Open(%s)\n", filename)
	path, err := s.root.Resolve(filename)
	if err != nil {
		return nil, fs.ErrInvalid
	}
	fileInfo, err := s.lsFile(path)
	if err != nil {
		return nil, err
	}
	return s.openFile(filename, fileInfo)
}

func (s ReferenceFileSystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	log.Printf("OpenFile(%s, %d, %s)\n", filename, flag, perm.String())

	path, err := s.root.Resolve(filename)
	if err != nil {
		return nil, fs.ErrInvalid
	}

	if flag != os.O_RDONLY {
		return nil, billy.ErrReadOnly
	}

	fileInfo, err := s.lsFile(path)
	if err != nil {
		return nil, err
	}

	if perm != fileInfo.mode {
		return nil, billy.ErrReadOnly
	}
	return s.openFile(filename, fileInfo)
}

func (s ReferenceFileSystem) Stat(filename string) (os.FileInfo, error) {
	log.Printf("Stat(%s)\n", filename)

	path, err := s.root.Resolve(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path %s: %v", filename, err)
	}

	// Root must be a directory so we like and say it is. Git doesn't really have a root they expose through ls-tree
	// so we can't make this as easy as I'd like it to be. Technically the "hash" of this would be the commit that we
	// are pointing to at head but I didn't feel like executing another git command here.
	if path.IsRoot() {
		return gitFileInfo{
			mode: 0555 | os.ModeDir,
			Type: GitTree,
			Hash: "",
			path: filename,
			size: 0,
		}, nil
	}

	return s.lsFile(path)
}

func (s ReferenceFileSystem) Rename(oldpath, newpath string) error {
	_ = oldpath
	_ = newpath
	return billy.ErrReadOnly
}

func (s ReferenceFileSystem) Remove(filename string) error {
	_ = filename
	return billy.ErrReadOnly
}

func (s ReferenceFileSystem) Join(elem ...string) string {
	return filepath.Clean(filepath.Join(elem...))
}

// billy.TempFile type implementation

func (s ReferenceFileSystem) TempFile(dir, prefix string) (billy.File, error) {
	_ = dir
	_ = prefix
	return nil, billy.ErrReadOnly
}

// billy.Dir type implementation

func (s ReferenceFileSystem) ReadDir(path string) ([]os.FileInfo, error) {
	log.Printf("ReadDir(%s)\n", path)
	gitPath, err := s.root.Resolve(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path %s: %v", path, err)
	}

	if !gitPath.IsRoot() {
		fileInfo, err := s.lsFile(gitPath)
		if err != nil {
			return nil, err
		}

		if !fileInfo.IsDir() {
			return nil, fs.ErrInvalid
		}
	}

	var files []os.FileInfo
	err = s.lsTree(gitPath, true, func(file gitFileInfo) error {
		files = append(files, file)
		return nil
	})
	return files, err
}

func (s ReferenceFileSystem) MkdirAll(filename string, perm os.FileMode) error {
	_ = filename
	_ = perm
	return billy.ErrReadOnly
}

// billy.Chroot type implementation

func (s ReferenceFileSystem) Root() string {
	log.Printf("Root()\n")
	return s.root.String()
}

func (s ReferenceFileSystem) Chroot(path string) (billy.Filesystem, error) {
	log.Printf("Chroot(%s)\n", path)
	gitPath, err := s.root.Resolve(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path %s: %v", path, err)
	}

	// TODO(gravypod): Handle these following cases...
	//  1. path does not exist
	//  2. path leads to a symlink
	//  3. path is not a directory
	return ReferenceFileSystem{
		root: gitPath,
		git:  s.git,
	}, nil
}

// billy.Symlink type implementation

func (s ReferenceFileSystem) Lstat(filename string) (os.FileInfo, error) {
	return s.Stat(filename)
}

func (s ReferenceFileSystem) Symlink(target, link string) error {
	_ = target
	_ = link
	return billy.ErrReadOnly
}

func (s ReferenceFileSystem) Readlink(link string) (string, error) {
	log.Printf("ReadLink(%s)\n", link)
	gitPath, err := s.root.Resolve(link)
	if err != nil {
		return "", fmt.Errorf("failed to parse path %s: %v", link, err)
	}
	fileInfo, err := s.lsFile(gitPath)
	if err != nil {
		return "", err
	}
	contents, err := s.git.ReadBlob(fileInfo.Hash)
	if err != nil {
		return "", err
	}
	parent := gitPath.Parent()
	realGitPath, err := parent.Resolve(string(contents))
	if err != nil {
		return "", err
	}
	return realGitPath.String(), nil
}

// billy.Change type implementation

func (s ReferenceFileSystem) Chmod(name string, mode os.FileMode) error {
	_ = name
	_ = mode
	return billy.ErrReadOnly
}

func (s ReferenceFileSystem) Lchown(name string, uid, gid int) error {
	_ = name
	_ = uid
	_ = gid
	return billy.ErrReadOnly
}

func (s ReferenceFileSystem) Chown(name string, uid, gid int) error {
	_ = name
	_ = uid
	_ = gid
	return billy.ErrReadOnly
}

func (s ReferenceFileSystem) Chtimes(name string, atime time.Time, mtime time.Time) error {
	_ = name
	_ = atime
	_ = mtime
	return billy.ErrReadOnly
}

// billy.Capable

func (s ReferenceFileSystem) Capabilities() billy.Capability {
	log.Println("Checking capabilities of gitfs")
	return billy.ReadCapability | billy.SeekCapability
}

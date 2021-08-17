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
	"github.com/go-git/go-billy/v5"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func fileMap(paths []os.FileInfo) map[string]os.FileInfo {
	pathsMap := map[string]os.FileInfo{}
	for _, path := range paths {
		pathsMap[path.Name()] = path
	}
	return pathsMap
}

func makeRepository(t *testing.T) Git {
	testDataPath := filepath.Join("testdata", "repo")

	tmp := t.TempDir()

	friendlyGit, err := NewCliGit(filepath.Join(tmp, ".git"))
	if err != nil {
		t.Fatal(err)
	}

	rawGit := friendlyGit.(cliGit)

	err = rawGit.MakeMirroredRepository("hello world", testDataPath)
	if err != nil {
		t.Fatalf("couln't create fake repository: %v", err)
	}

	return rawGit
}

func TestRepositoryFileSystemReadOnly(t *testing.T) {
	git := makeRepository(t)
	fs := NewGitFileSystem(git)
	t.Run("reported capabilities", func(t *testing.T) {
		capabilities := billy.Capabilities(fs)
		writableCapabilities := []billy.Capability{
			billy.WriteCapability,
			billy.ReadAndWriteCapability,
			billy.TruncateCapability,
			billy.LockCapability,
		}
		for _, writable := range writableCapabilities {
			if capabilities&writable != 0 {
				t.Fatal("billy.Capabilities() reports writable bits are set.")
			}
		}
	})

	t.Run("fail writes", func(t *testing.T) {
		_, err := fs.OpenFile("something/that/doesnt/exist.txt", os.O_RDWR|os.O_CREATE, os.FileMode(0700))
		if err == nil {
			t.Fatal("OpenFile() opened a file that didn't exist with O_CREATE! This is a modification.")
		}

		file, err := fs.Open("real.txt")
		if err != nil {
			t.Fatalf("Open(real.txt) failed: %v", err)
		}
		_, err = file.Write([]byte{1})
		if err == nil {
			t.Fatal("file.Write() reported it successfully wrote to a file.")
		}

		err = file.Truncate(0)
		if err == nil {
			t.Fatal("file.Truncate(0) reported it truncated a file.")
		}

		err = file.Lock()
		if err == nil {
			t.Fatal("file.Lock() reported it locked a file.")
		}

		err = file.Unlock()
		if err == nil {
			t.Fatal("file.Unlock() reported it unlocked a file.")
		}
	})

	t.Run("reading", func(t *testing.T) {
		openFile, err := fs.OpenFile("real.txt", os.O_RDONLY, os.FileMode(0644))
		if err != nil {
			t.Fatalf("OpenFile(real.txt) failed: %v", err)
		}

		file, err := fs.Open("real.txt")
		if err != nil {
			t.Fatalf("Open(real.txt) failed: %v", err)
		}

		files := []billy.File{
			openFile, file,
		}
		for _, file := range files {
			data, err := ioutil.ReadAll(file)
			if err != nil {
				t.Fatalf("ioutils.ReadAll(file) failed: %v", err)
			}
			if string(data) != "Hello World\n" {
				t.Fatal("file.Read() on real.txt produced incorrect data")
			}

			data = []byte{0}

			pos, err := file.Seek(1, io.SeekStart)
			if err != nil {
				t.Fatal("file.Seek(1, io.SeekStart) failed to seek.")
			}
			if pos != 1 {
				t.Fatalf("file.Seek(1, io.SeekStart) incorrectly ended up at: %d", pos)
			}

			pos, err = file.Seek(1, io.SeekCurrent)
			if err != nil {
				t.Fatal("file.Seek(1, io.SeekCurrent) failed to seek.")
			}
			if pos != 2 {
				t.Fatalf("file.Seek(1, io.SeekCurrent) incorrectly ended up at: %d", pos)
			}
		}
	})

	t.Run("stat", func(t *testing.T) {
		filePaths := []string{
			"real.txt",
			"symlink.txt",
			"test",
			"test/nested.txt",
			"executable.sh",
		}

		stats := map[string]os.FileInfo{}
		for _, filePath := range filePaths {
			lstat, err := fs.Lstat(filePath)
			if err != nil {
				t.Fatalf("Lstat(%s) failed: %v", filePath, err)
			}

			stat, err := fs.Stat(filePath)
			if err != nil {
				t.Fatalf("Lstat(%s) failed: %v", filePath, err)
			}
			stats[filePath] = stat

			if lstat != stat {
				t.Fatalf("Lstat(%s) != Stat(%s)", filePath, filePath)
			}
		}

		if stats["symlink.txt"].Mode()&os.ModeSymlink == 0 {
			t.Fatal("symlink.txt was not a symlink")
		}

		if !stats["test"].IsDir() {
			t.Fatal("test/ was not reported to be a directory")
		}

		executable := stats["executable.sh"]
		if executable.Mode()&0111 == 0 {
			t.Fatal("executable.sh was not marked as executable!")
		}
	})

	t.Run("symlinks", func(t *testing.T) {
		destination, err := fs.Readlink("symlink.txt")
		if err != nil {
			t.Fatalf("Readlink(symlink.txt) failed: %v", err)
		}
		if destination != "real.txt" {
			t.Fatalf("symlink.txt->real.txt != %s", destination)
		}
	})

	t.Run("listing directories", func(t *testing.T) {
		_, err := fs.ReadDir("nonexistant_directory")
		if err == nil {
			t.Fatalf("listing a directory that didn't exist succeeded")
		}

		tests := map[string][]string{
			".":    {"test", "executable.sh", "real.txt", "symlink.txt"},
			"test": {"escaping.txt", "nested.txt"},
		}

		for readDirPath, expectedPaths := range tests {
			paths, err := fs.ReadDir(readDirPath)
			if err != nil {
				t.Fatalf("failed to list %s directory: %v", readDirPath, err)
			}
			if len(paths) != len(expectedPaths) {
				t.Fatalf("listing did not include all files: %v", paths)
			}
			pathsMap := fileMap(paths)

			for _, expectedPath := range expectedPaths {
				_, ok := pathsMap[expectedPath]
				if !ok {
					t.Fatalf("directory %s missing file %s", readDirPath, expectedPath)
				}
			}
		}
	})

	t.Run("chroot", func(t *testing.T) {
		newRoot, err := fs.Chroot("test")
		if err != nil {
			t.Fatalf("failed to chroot into nested: %v", err)
		}

		if newRoot.Root() != "test" {
			t.Fatalf("returned incorrect root path: %s", newRoot.Root())
		}

		paths, err := newRoot.ReadDir(".")
		if err != nil {
			t.Fatalf("failed to list from chroot: %v", err)
		}
		if len(paths) != 2 {
			t.Fatalf("listed wrong number of files in nested folder")
		}

		pathsMap := fileMap(paths)
		if _, ok := pathsMap["nested.txt"]; !ok {
			t.Fatalf("nested.txt was not found")
		}

		file, err := newRoot.Open("escaping.txt")
		if err != nil {
			t.Fatalf("couldn't open broken symlink: %v", err)
		}
		contents, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatalf("couldn't read broken symlink: %v", err)
		}
		text := string(contents)
		if text == "Hello World\n" {
			t.Fatalf("Was able to escape chroot with symlink.")
		}
		if text != "../real.txt" {
			t.Fatalf("expected symlink to ../real.txt but found %s", text)
		}
	})

	t.Run("mutators", func(t *testing.T) {
		_, err := fs.Create("something.txt")
		if err != billy.ErrReadOnly {
			t.Fatalf("Was allowed to create something.txt")
		}

		err = fs.Rename("something.txt", "else.txt")
		if err != billy.ErrReadOnly {
			t.Fatalf("Was allowed to rename something.txt")
		}

		err = fs.Remove("real.txt")
		if err != billy.ErrReadOnly {
			t.Fatalf("Was allowed to remove real.txt")
		}

		_, err = fs.TempFile("test", "abcd")
		if err != billy.ErrReadOnly {
			t.Fatalf("Was allowed to remove create a temp file")
		}

		err = fs.MkdirAll("nonexisting_dir", os.FileMode(0777))
		if err != billy.ErrReadOnly {
			t.Fatalf("Was allowed to create a directory")
		}

		err = fs.Symlink("fake.txt", "real.txt")
		if err != billy.ErrReadOnly {
			t.Fatalf("Was allowed to symlink to real.txt")
		}
	})
}

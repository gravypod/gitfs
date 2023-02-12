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

package main

import (
	"flag"
	gitfs "github.com/gravypod/gitfs/pkg"
	"github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
	"log"
	"net"
)

var repositoryDirectory = flag.String("git-dir", "", "Path to bare git repo to serve.")

func main() {
	flag.Parse()

	if len(*repositoryDirectory) == 0 {
		panic("No repository provided. Please specify '-git-dir'")
	}

	listener, err := net.Listen("tcp", "0.0.0.0:46051")
	if err != nil {
		log.Panicf("could not bind tcp port: %v", err)
	}
	defer listener.Close()
	log.Printf("NFS server started at %s\n", listener.Addr())

	git, err := gitfs.NewCliGit(*repositoryDirectory)
	if err != nil {
		log.Fatalf("Failed to create git client for directory '%s': %v", *repositoryDirectory,
			err)
	}

	branch := "master"
	fs := gitfs.NewReferenceFileSystem(git, gitfs.GitReference{Branch: &branch})

	authHandler := nfshelper.NewNullAuthHandler(fs)
	cachedFs := nfshelper.NewCachingHandler(authHandler, 1024)
	err = nfs.Serve(listener, cachedFs)
	if err != nil {
		log.Panicln(err)
	}
}

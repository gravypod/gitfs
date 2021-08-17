# gitfs

WARNING: This project is not an official Google project. See disclaimer below.

A virtual file system that allows you to mount a git repository from a local
bare git repository or remotely as an NFS share. This makes your repository
show up as if you have performed a `git checkout` on a normal git repository.

## Why?

This can be useful if you want to have many programs simultaneously reading
from your git repository and would like changes to your git repository to
automagically appear in all instances.

For example if you wanted to use [Kythe](kythe) to analyize a [Bazel](bazel)
repository over time (on every commit) you could clone the bare repos on a
single hosts, mount the repository on a bunch of worker nodes using this tool's
NFS share, and then execute compile [extractions](kythe-extract) on every
single worker node. There are some roadblocks preventing this from being a
possibility as it stands but this would allow your workers to be entirely
stateless as they wouldn't need disks to even store the code they are compiling
and you could mount the Bazel's build output into a tmpfs that is discarded
after every build.

Another benifit is that this file system entirely rejects writes so you know
that code built from this mount is exactly representative of what was inside
your bare git repository. If you were using [Bazel](bazel) and two machines
built off of the same NFS mount they should both produce identical binaries
which would make it easier for users to build and validate sha outputs of
binaries to attempt to verify their builds.

You can also use this to expose a slow, but usable, config folder to many
production workers. This code implements a chroot-like mechanism that could
be tweaked to make it so a `configs/` folder could be exposed allowing you
to do feature flag flips and other config changes with your standard
code review process. Since all of you jobs would be reading from an NFS
share these changes should be present in all of your jobs at the same
time assuming you disable nfs caching on the mount and you only run a
single gitfs server.

## TODO

Some things that I wish this code supported:

1. Improve fuse support. Right now fuse is only minimally supported and there
   are many subtle bugs.
2. Expose a file tree that allows users of the file system to explore multiple
   git refs (branches, tags, commits, etc).
3. Expose very detailed prometheus metrics. Ideally we should be able to track
   everything down to the specific operations being done.
4. Directly read the git index rather than shelling out to git commands. This
   would allow us to have better deadlining support for go Contexts (deadlines,
   etc) and would probably significantly improve the performance of the VFS.
   Rather than reading files into entirely into memory we could just map all
   read calls to the underlying object files.
5. Write support? Allow users you create branches using `mkdir` and generate
   and ammend commits as people write files?

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for details.

## License

Apache 2.0; see [`LICENSE`](LICENSE) for details.

## Disclaimer

This project is not an official Google project. It is not supported by
Google and Google specifically disclaims all warranties as to its quality,
merchantability, or fitness for a particular purpose.

[kythe]: https://kythe.io
[kythe-extractions]: https://kythe.io/examples/#extracting-other-bazel-based-repositories
[bazel]: https://bazel.build


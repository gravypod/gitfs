#!/usr/bin/env sh
#
# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

git init

## real.txt ##
cat <<EOF >real.txt
Hello World
EOF
git add real.txt
git commit -m "Add a normal file"

## symlink.txt ##
ln -s real.txt symlink.txt
git add symlink.txt
git commit -m "Add a symlink file"

## executable.sh (+x) ##
cat <<EOF >executable.sh
#!/bin/bash -eu
#
# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/usr/bin/env bash
echo "Hello World"
EOF
chmod +x executable.sh

git add executable.sh
git commit -m "Added an executable file."

## symlink.txt ##
ln -s real.txt symlink.txt
git add symlink.txt
git commit -m "Add symlink"

## test/nested.txt, test/escaping.txt ##
mkdir test/

# Normal file.
cat <<EOF >test/nested.txt
Nested file
EOF

# Symlink to previous dir.
ln --symbolic --relative real.txt test/escaping.txt

git add test/
git commit -m "Add a directory."

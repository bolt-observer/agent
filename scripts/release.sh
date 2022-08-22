#!/bin/sh
set -euo pipefail

# Simple for now
# TODO: add deterministic build and stuff

SCRIPTPATH=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
cd $SCRIPTPATH/..

cd release
rm -rf *
cd ..

make all
cd release
tag=$(git describe)

find . -type f -executable -exec zip {}.zip {} \;
echo > "manifest-$tag.txt"
shasum -a 256 *.zip >> "manifest-$tag.txt"

# for me
gpg --detach-sign --local-user F4B8B3B59C1E5AA39A1B9636E897355718E1DBF4 --armor "manifest-$tag.txt"

cd $SCRIPTPATH

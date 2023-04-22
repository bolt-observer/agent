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

find . -type f -exec zip {}.zip {} \;
echo > "manifest-$tag.txt"
shasum -a 256 *.zip >> "manifest-$tag.txt"

# Determine user
git config user.email | grep -Eiq '(gregor|fiksn)' && USER=fiksn
git config user.email | grep -Eiq '(tomaz|dcrystalj)' && USER=dcrystalj

KEY=$(gpg --import-options show-only --import < ../scripts/keys/$USER.asc 2>/dev/null | awk '/^pub/ { getline; {$1=$1}; print}')
echo "User $USER signing with key $KEY"

gpg --detach-sign --local-user $KEY --armor "manifest-$tag.txt"

cd $SCRIPTPATH

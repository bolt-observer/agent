#!/bin/sh

{ # Prevent execution if this script was only partially downloaded
oops() {
    echo "$0:" "$@" >&2
    exit 1
}

umask 0022

sudo=${sudo:-"sudo"}
agent="bolt-agent"

tmpDir="$(mktemp -d -t boltobserver-unpack.XXXXXXXXXX || \
          oops "Can't create temporary directory")"
cleanup() {
    rm -rf "$tmpDir"
}
trap cleanup EXIT INT QUIT TERM

require_util() {
    command -v "$1" > /dev/null 2>&1 ||
        oops "you do not have '$1' installed, which I need to $2"
}

require_util unzip "unpack the package"
require_util gpg "check integrity"
require_util openssl "check integrity"

if command -v curl > /dev/null 2>&1; then
    fetch() { curl -s --fail -L "$1" -o "$2"; }
elif command -v wget > /dev/null 2>&1; then
    fetch() { wget -q "$1" -O "$2"; }
else
    oops "you don't have wget or curl installed, which I need to download zip"
fi

latest=$(fetch "https://api.github.com/repos/bolt-observer/agent/releases/latest" "-" | grep 'browser_' | cut -d":" -f 2- | tr -d '"' | rev | cut -d"/" -f 2 | rev | head -1)

case "$(uname -s).$(uname -m)" in
   Linux.x86_64)
        name=${agent}-${latest}-linux.zip
        ;;
   Linux.aarch64)
        name=${agent}-${latest}-rasp.zip
        ;;
   Darwin.arm64|Darwin.aarch64)
        name=${agent}-${latest}-darwin.zip
        ;;
    *) oops "sorry, your platform is not yet supported";;
esac

url="https://github.com/bolt-observer/agent/releases/download/${latest}/${name}"
echo "downloading agent from '$url' to '$tmpDir'..."
fetch "$url" "$tmpDir/agent.zip" || oops "failed to download '$url'"
url="https://github.com/bolt-observer/agent/releases/download/${latest}/manifest-${latest}.txt"
echo "downloading manifest from '$url' to '$tmpDir'..."
fetch "$url" "$tmpDir/manifest.txt" || oops "failed to download '$url'"
url="https://github.com/bolt-observer/agent/releases/download/${latest}/manifest-${latest}.txt.asc"
echo "downloading manifest.asc from '$url' to '$tmpDir'..."
fetch "$url" "$tmpDir/manifest.txt.asc" || oops "failed to download '$url'"
url="https://raw.githubusercontent.com/bolt-observer/agent/main/scripts/keys/fiksn.asc"
echo "downloading key from '$url' to '$tmpDir'..."
fetch "$url" "$tmpDir/key" || oops "failed to download '$url'"
cat "$tmpDir/key" | gpg --import

url="https://raw.githubusercontent.com/bolt-observer/agent/main/${agent}.service"
echo "downloading key from '$url' to '$tmpDir'..."
fetch "$url" "$tmpDir/service" || oops "failed to download '$url'"
if [ -f "/etc/system/system" ]; then
  if [ ! -f "/etc/systemd/system/${agent}" ]; then
    cp ${agent}.service /etc/systemd/system/
    systemctl daemon-reload
  fi
fi

cd $tmpDir 2>/dev/null

# Checksum
gpg --verify manifest.txt.asc manifest.txt || { echo "GPG signature incorrect"; exit 1; }
sum=$(openssl sha256 agent.zip | cut -d "=" -f 2 | tr -d " \n")
grep -q $sum manifest.txt || { echo "Checksum invalid"; exit 1; }

unzip agent.zip
${sudo} mkdir -p /usr/local/bin
${sudo} cp -f *-agent-* /usr/local/bin

if [ -f "/etc/system/system" ]; then
  ${sudo} systemctl restart ${agent}.service
fi

cd - 2>/dev/null

echo "Agent ${latest} installed to /usr/local/bin"

} # end of wrapping

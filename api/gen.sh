#! /usr/bin/env nix-shell
#! nix-shell -i bash -p bash protoc-gen-go grpc-tools

### Generate go code from protobuf
set -euo pipefail

rm -f *.pb.go
protoc --plugin $(which protoc-gen-go) --go_out=.. --go_opt=Mapi.proto=api/ api.proto

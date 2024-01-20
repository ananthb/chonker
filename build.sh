#!/bin/bash

set -euo pipefail

app="$(basename "$(pwd)")"

go generate -x ./...

mkdir -p bin
pushd bin

gobuild() {
  mkdir -p "$1/$2"
  pushd "$1/$2"
  env GOOS="$1" GOARCH="$2" go build ../../../cmd/...
  if [[ $1 == "windows" ]]; then
    zip ../../"${app}_${1}_${2}".zip ./*.exe
  else
    tar -c --zstd --numeric-owner \
      -f ../../"${app}_${1}_${2}".tar.gz .
  fi
  popd
}

gobuild linux amd64
gobuild linux arm64
gobuild darwin amd64
gobuild darwin arm64
gobuild windows amd64

sha1sum ./*.tar.gz ./*.zip >sha1sums.txt

popd

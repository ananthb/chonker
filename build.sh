#!/bin/bash

set -euo pipefail

app="$(basename "$PWD")"
version="${1:-dev}"

env VERSION="$version" go generate -x ./...

rm -rf bin
mkdir -p bin
pushd bin

gobuild() {
  mkdir -p "$1/$2"
  pushd "$1/$2"
  env GOOS="$1" GOARCH="$2" go build ../../../cmd/...
  case "$1" in
    "windows")
      zip ../../"${app}_${1}_${2}_${version}".zip ./*.exe
      ;;
    "linux")
      tar -c --zstd --numeric-owner \
        -f ../../"${app}_${1}_${2}_${version}".tar.zst .
      ;;
    *)
      tar -c --numeric-owner \
        -f ../../"${app}_${1}_${2}_${version}".tar.gz .
      ;;
  esac
  popd
}

export CGO_ENABLED=0

gobuild linux amd64
gobuild linux arm64
gobuild darwin amd64
gobuild darwin arm64
gobuild windows amd64

sha1sum ./*.tar.gz ./*.zip >sha1sums.txt

popd

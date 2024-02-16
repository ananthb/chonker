#!/bin/bash
# 
# test.sh - Test chonk with a list of URLs
#
# Usage: test.sh [--help|-h] [--list|-l] [--files|-f [all|<file>...]]

set -euo pipefail

chonk="./bin/chonk_test"

mkdir -p bin
go build -cover -o "$chonk" ./cmd/chonk

r2_bucket_url="https://chonker-test.kedi.dev"
download_urls=()
for file in "1_MiB.bin" "10_MiB.bin" "100MiB.bin"; do
	download_urls+=("$r2_bucket_url/$file")
done


printf 'Testing chonk with %s URLs\n\n' "${#download_urls[@]}"

rm -rf coverage
mkdir -p coverage
export GOCOVERDIR="coverage"
xargs -n 1 "$chonk" -c 16MiB -o /dev/null < <(printf '%s\n' "${download_urls[@]}")

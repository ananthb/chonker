#!/bin/bash
# 
# test.sh - Test chonk with a list of URLs
#
# Usage: test.sh [--help|-h] [--list|-l] [--files|-f [all|<file>...]]

set -euo pipefail

rm -rf tests

GOCOVERDIR="tests/coverage"
export GOCOVERDIR

mkdir -p "$GOCOVERDIR"

printf 'Unit test coverage\n'
go test -race ./...

export CGO_ENABLED=0
go test -coverprofile tests/unit-cover-profile.txt -cover ./...
go tool cover -html=tests/unit-cover-profile.txt -o tests/unit-cover-profile.html

chonk="$(mktemp -d)/chonk"
go build -cover -o "$chonk" ./cmd/chonk

r2_bucket_url="https://chonker-test.kedi.dev"
download_urls=()
for file in "1_MiB.bin" "10_MiB.bin" "100_MiB.bin"; do
	download_urls+=("$r2_bucket_url/$file")
done

xargs -n 1 "$chonk" -q \
	-m /dev/null \
	-c 16MiB \
	-o /dev/null \
	< <(printf '%s\n' "${download_urls[@]}")

# Test a 404
"$chonk" -q -w 4 -o /dev/null "https://example.com/404" || true

# Test a URL without range support
# TODO: replace this with a URL that doesn't support range requests
"$chonk" -continue -q -w 4 -o /dev/null "https://example.com/404" || true

# Cancel a download
"$chonk" -q -w 16 -c 1KiB -o /dev/null "https://chonker-test.kedi.dev/100_MiB.bin" &
pid=$!
sleep 1
kill -SIGINT "$pid"

printf 'Integration test coverage\n'
go tool covdata percent -i "$GOCOVERDIR"
go tool covdata textfmt -i "$GOCOVERDIR" -o tests/integration-cover-profile.txt
go tool cover -html=tests/integration-cover-profile.txt -o tests/integration-cover-profile.html

if command -v xdg-open &> /dev/null; then
	xdg-open tests/*-profile.html
fi
if command -v open &> /dev/null; then
	open tests/*-profile.html
fi

#!/bin/bash
# 
# test.sh - Test chonk with a list of URLs
#
# Usage: test.sh [--help|-h] [--list|-l] [--files|-f [all|<file>...]]

set -euo pipefail


chonk=${CHONK:-}
if [[ -z $chonk ]]; then
	chonk="$(command -v chonk || true)"
fi
if [[ -z $chonk ]]; then
	printf 'chonk not found\nSet CHONK environment variable to the path of chonk binary\n' >&2
	exit 1
fi

r2_bucket_url="https://chonker-test.kedi.dev"
download_urls=()
for file in "1_MiB.bin" "10_MiB.bin" "100MiB.bin"; do
	download_urls+=("$r2_bucket_url/$file")
done

printf 'Testing chonk with %s URLs\n\n' "${#download_urls[@]}"

xargs -n 1 "$chonk" -c 16MiB -o /dev/null < <(printf '%s\n' "${download_urls[@]}")

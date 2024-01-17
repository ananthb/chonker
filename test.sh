#!/bin/bash
# 
# test.sh - Test chonk with a list of URLs
#
# Usage: test.sh [--help|-h] [--list|-l] [--files|-f [all|<file>...]]

set -euo pipefail

index_url="https://zwyr3fszdjzjeu6panxr6m3kqe0hukab.lambda-url.ap-south-1.on.aws"

chonk=${CHONK:-}
if [[ -z $chonk ]]; then
	chonk="$(command -v chonk || true)"
fi
if [[ -z $chonk ]]; then
	printf 'chonk not found\nSet CHONK environment variable to the path of chonk binary\n' >&2
	exit 1
fi

# Download file list to tmp folder


case "${1:-}" in
--help | -h)
	printf 'Usage: %s [--help|-h] [--list|-l] [--files|-f [all|<file>...]]\n' "$0"
	printf '  -h, --help\t\tShow this help message\n'
	exit 0
	;;

--list | -l)
	readarray -t test_file_names < <(curl -s "$index_url" | jq -r '.options[].name')
	printf '%s\n' "${test_file_names[@]}"
	exit 0
	;;

--files | -f)
	shift
	if [[ $# -eq 0 ]]; then
		printf 'No files specified\n' >&2
		exit 1
	fi

	test_files_index=$(mktemp)
	curl -s "$index_url" -o "$test_files_index"
	trap 'rm -f "$test_files_index"' EXIT
	readarray -t test_file_names < <(jq -r '.options[].name' "$test_files_index")
	if [[ $1 == 'all' ]]; then
		shift
	else
		# Check if all files are valid
		for file in "$@"; do
			if ! printf '%s\n' "${test_file_names[@]}" | grep -q "^$file$"; then
				printf 'Invalid file: %s\nRun %s --list for a list of files' "$file" "$0" >&2
				exit 1
			fi
		done
		test_file_names=("$@")
	fi
	;;

*)
	if [[ $# -gt 0 ]]; then
		printf 'Invalid option: %s\n' "$1" >&2
		exit 1
	fi
	;;
esac

download_urls=()
for file in "${test_file_names[@]}"; do
	readarray -t -O "${#download_urls[@]}" download_urls < \
		<(jq -r '.options[] | select(.name == "'"$file"'") | .links[]' "$test_files_index")
done

printf 'Testing chonk with %s URLs\n\n' "${#download_urls[@]}"

xargs -n 1 "$chonk" -c 16MiB -o /dev/null < <(printf '%s\n' "${download_urls[@]}")

#!/bin/bash
# 
# test.sh - Test chonk with a list of URLs
#
# Usage: test.sh [--help|-h] [--list|-l] [--files|-f [all|<file>...]]

set -euo pipefail

# Download file list to tmp folder

test_files_index=$(mktemp)
curl -s https://zwyr3fszdjzjeu6panxr6m3kqe0hukab.lambda-url.ap-south-1.on.aws -o "$test_files_index"
trap 'rm -f "$test_files_index"' EXIT

readarray -t test_file_names < <(jq -r '.options[].name' "$test_files_index")

case "${1:-}" in
--help | -h)
	printf 'Usage: %s [--help|-h] [--list|-l] [--files|-f [all|<file>...]]\n' "$0"
	printf '  -h, --help\t\tShow this help message\n'
	exit 0
	;;

--list | -l)
	printf '%s\n' "${test_file_names[@]}"
	exit 0
	;;

--files | -f)
	shift
	if [[ $1 == 'all' ]]; then
		shift
	else
		# Check if all files are valid
		for file in "$@"; do
			if ! printf '%s\n' "${test_file_names[@]}" | grep -q "^$file$"; then
				printf 'Invalid file: %s\n' "$file"
				exit 1
			fi
		done
		test_file_names=("$@")
	fi
	;;

*)
	if [[ $# -gt 0 ]]; then
		printf 'Invalid option: %s\n' "$1"
		exit 1
	fi
	;;
esac

download_urls=()
for file in "${test_file_names[@]}"; do
	readarray -t -O "${#download_urls[@]}" download_urls < \
		<(curl -s "https://zwyr3fszdjzjeu6panxr6m3kqe0hukab.lambda-url.ap-south-1.on.aws" |
			jq -r '.options[] | select(.name == "'"$file"'") | .links[]')
done

printf 'Testing chonk with %s URLs\n\n' "${#download_urls[@]}"

xargs -n 1 ./bin/chonk -out /dev/null < <(printf '%s\n' "${download_urls[@]}")

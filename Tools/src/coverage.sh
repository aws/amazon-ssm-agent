#!/usr/bin/env bash
# Generate test coverage statistics for Go packages.
#

echo "Try update 'cover'"
GOPATH=`pwd`/Tools go get -u golang.org/x/tools/cmd/cover

set -e

work_dir=.cover
profile="$work_dir/cover.out"
mode=count
code_coverage_roots=("$@")

mkdir -p "$work_dir"

generate_code_coverage(){
	for package in "$@"; do
        f="$work_dir/$(echo $package | tr / -).cover"
        go test -covermode="$mode" -coverprofile="$f" "$package"
    done
    grep -h -v "^mode:" "$work_dir"/*.cover >>"$profile"
}

echo "mode: $mode" >"$profile"
for root in ${code_coverage_roots[@]}; do
    generate_code_coverage $(go list $root)
done

go tool cover -html="$profile" -o ./bin/coverage.html
go tool cover -func="$profile"

rm -rf "$work_dir"
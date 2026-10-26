#!/usr/bin/env bats

load helpers

@test "version prints expected output" {
    repo_root=$(dirname "$(dirname "$SKILLCTL")")
    version=$(sed -n 's/^var Version = "\(.*\)"$/\1/p' "$repo_root/internal/version/version.go")
    run_skillctl version
    [ "$status" -eq 0 ]
    [[ "$output" =~ ^skillctl\ ${version}\ \([a-f0-9]+(\+dirty)?\)$ ]]
}

@test "version takes no arguments" {
    run "$SKILLCTL" version foo
    [ "$status" -ne 0 ]
}

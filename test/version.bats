#!/usr/bin/env bats

load helpers

@test "version prints expected output" {
    repo_root=$(dirname "$(dirname "$SKILLCTL")")
    version=$(cat "$repo_root/VERSION")
    run_skillctl version
    [ "$status" -eq 0 ]
    [[ "$output" =~ ^skillctl\ ${version}\ \([a-f0-9]+\)$ ]]
}

@test "version takes no arguments" {
    run "$SKILLCTL" version foo
    [ "$status" -ne 0 ]
}

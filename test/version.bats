#!/usr/bin/env bats

load helpers

@test "version reports the build and rejects arguments" {
    local version
    version=$(sed -n 's/^var Version = "\(.*\)"$/\1/p' "$REPO_ROOT/internal/version/version.go")
    run_skillctl version
    [[ "$output" =~ ^skillctl\ ${version}\ \([a-f0-9]+(\+dirty)?\)$ ]]

    run "$SKILLCTL" version foo
    [ "$status" -ne 0 ]
    [[ "$output" == *'unknown command "foo" for "skillctl version"'* ]]
}

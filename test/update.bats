#!/usr/bin/env bats

load helpers

@test "update without init fails" {
    run "$SKILLCTL" update
    [ "$status" -ne 0 ]
}

@test "update with empty manifest succeeds" {
    run_skillctl init
    run_skillctl update
    [[ "$output" == *"No skills in manifest"* ]]
}

@test "update takes no arguments" {
    run "$SKILLCTL" update foo
    [ "$status" -ne 0 ]
}

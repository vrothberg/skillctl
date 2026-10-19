#!/usr/bin/env bats

# Tests for list with populated manifests are in pull.bats.

load helpers

@test "list without init fails" {
    run "$SKILLCTL" list
    [ "$status" -ne 0 ]
    [[ "$output" =~ "skillctl init" ]]
}

@test "list on empty manifest shows no skills" {
    run_skillctl init
    [ "$status" -eq 0 ]

    run_skillctl list
    [ "$status" -eq 0 ]
    [[ "$output" =~ "No skills tracked" ]]
}

@test "list alias ls works" {
    run_skillctl init
    [ "$status" -eq 0 ]

    run_skillctl ls
    [ "$status" -eq 0 ]
    [[ "$output" =~ "No skills tracked" ]]
}

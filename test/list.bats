#!/usr/bin/env bats

# Populated lists are exercised by the pull and remove workflows.
load helpers

@test "list requires init and lists an empty project through both aliases" {
    run "$SKILLCTL" list
    [ "$status" -ne 0 ]
    [[ "$output" == *"skillctl init"* ]]
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]

    run_skillctl list
    [[ "$output" == *"No skills tracked"* ]]
    local listed="$output"
    run_skillctl ls
    [[ "$output" == "$listed" ]]
}

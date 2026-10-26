#!/usr/bin/env bats

load helpers

@test "init creates manifest and skills directory" {
    run_skillctl init
    [ "$status" -eq 0 ]
    [ -f skillctl.yaml ]
    [ -d .agents/skills ]
    [[ "$output" =~ "Initialized skillctl project" ]]
}

@test "init manifest contains correct apiVersion" {
    run_skillctl init
    [ "$status" -eq 0 ]
    run grep "apiVersion: v1" skillctl.yaml
    [ "$status" -eq 0 ]
}

@test "init manifest contains empty skills list" {
    run_skillctl init
    [ "$status" -eq 0 ]
    run grep "skills: \[\]" skillctl.yaml
    [ "$status" -eq 0 ]
}

@test "init fails if manifest already exists" {
    run_skillctl init
    [ "$status" -eq 0 ]

    run "$SKILLCTL" init
    [ "$status" -ne 0 ]
    [[ "$output" =~ "already exists" ]]
}

@test "init is idempotent for skills directory" {
    mkdir -p .agents/skills
    run_skillctl init
    [ "$status" -eq 0 ]
    [ -d .agents/skills ]
}

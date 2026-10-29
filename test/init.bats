#!/usr/bin/env bats

load helpers

@test "init creates an empty project and refuses to overwrite its manifest" {
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    [ -d .agents/skills ]
    grep -q '^apiVersion: v1$' skillctl.yaml
    grep -q '^skills: \[\]$' skillctl.yaml
    local before
    before=$(cat skillctl.yaml)

    run "$SKILLCTL" init
    [ "$status" -ne 0 ]
    [[ "$output" == *"already exists"* ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
}

@test "init preserves a preexisting skills directory" {
    mkdir -p .agents/skills
    echo existing > .agents/skills/marker
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    [ -d .agents/skills ]
    [[ "$(cat .agents/skills/marker)" == "existing" ]]
}

#!/usr/bin/env bats

load helpers

function setup() {
    setup_example_registry
}

@test "remove without init fails" {
    run "$SKILLCTL" remove foo
    [ "$status" -ne 0 ]
}

@test "remove requires an argument" {
    run "$SKILLCTL" remove
    [ "$status" -ne 0 ]
}

@test "remove nonexistent skill fails" {
    run_skillctl init
    run "$SKILLCTL" remove nonexistent
    [ "$status" -ne 0 ]
    [[ "$output" =~ "not found" ]]
}

@test "remove --force succeeds for nonexistent skill" {
    run_skillctl init
    run_skillctl remove --force nonexistent
    [ "$status" -eq 0 ]
}

@test "remove deletes extracted files and manifest entry" {
    run_skillctl init
    run_skillctl pull --trust none ${EXAMPLE_IMAGE}:v1.0.0
    [ "$status" -eq 0 ]
    [ -f .agents/skills/skillctl-example-skill/SKILL.md ]

    run_skillctl remove skillctl-example-skill
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Removed skillctl-example-skill" ]]

    # Extracted files gone
    [ ! -d .agents/skills/skillctl-example-skill ]

    # Manifest entry gone
    ! grep -q "skillctl-example-skill" skillctl.yaml

    # List confirms removal
    run_skillctl list
    [[ "$output" =~ "No skills tracked" ]]
}

@test "rm alias works" {
    run_skillctl init
    run_skillctl pull --trust none ${EXAMPLE_IMAGE}:v1.0.0
    [ "$status" -eq 0 ]

    run_skillctl rm skillctl-example-skill
    [ "$status" -eq 0 ]
    [ ! -d .agents/skills/skillctl-example-skill ]
}

@test "remove one skill leaves others intact" {
    run_skillctl init
    run_skillctl pull --trust none --name skill-v1 ${EXAMPLE_IMAGE}:v1.0.0
    run_skillctl pull --trust none --name skill-v2 ${EXAMPLE_IMAGE}:v1.0.1
    [ "$status" -eq 0 ]

    run_skillctl remove skill-v1
    [ "$status" -eq 0 ]

    [ ! -d .agents/skills/skill-v1 ]
    [ -f .agents/skills/skill-v2/counter.txt ]

    run_skillctl list
    [[ "$output" != *"skill-v1"* ]]
    [[ "$output" =~ "skill-v2" ]]
}

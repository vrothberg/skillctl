#!/usr/bin/env bats

load helpers

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
    run_skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]
    [ -f skills/skillctl-example-skill/skill.md ]

    run_skillctl remove skillctl-example-skill
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Removed skillctl-example-skill" ]]

    # Extracted files gone
    [ ! -d skills/skillctl-example-skill ]

    # Manifest entry gone
    ! grep -q "skillctl-example-skill" skillctl.yaml

    # List confirms removal
    run_skillctl list
    [[ "$output" =~ "No skills tracked" ]]
}

@test "rm alias works" {
    run_skillctl init
    run_skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    run_skillctl rm skillctl-example-skill
    [ "$status" -eq 0 ]
    [ ! -d skills/skillctl-example-skill ]
}

@test "remove one skill leaves others intact" {
    run_skillctl init
    run_skillctl pull --name skill-v1 quay.io/vrothberg/skillctl-example-skill:v1.0.0
    run_skillctl pull --name skill-v2 quay.io/vrothberg/skillctl-example-skill:v1.0.1
    [ "$status" -eq 0 ]

    run_skillctl remove skill-v1
    [ "$status" -eq 0 ]

    [ ! -d skills/skill-v1 ]
    [ -f skills/skill-v2/counter.txt ]

    run_skillctl list
    [[ "$output" != *"skill-v1"* ]]
    [[ "$output" =~ "skill-v2" ]]
}

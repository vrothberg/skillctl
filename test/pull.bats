#!/usr/bin/env bats

load helpers

@test "pull without init fails without extracting" {
    run "$SKILLCTL" pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -ne 0 ]
    # Must not leave orphaned files on disk
    [ ! -d skills ]
}

@test "pull requires an argument" {
    run "$SKILLCTL" pull
    [ "$status" -ne 0 ]
}

@test "pull rejects invalid image reference" {
    run_skillctl init
    run "$SKILLCTL" pull ":::invalid:::"
    [ "$status" -ne 0 ]
}

@test "pull accepts --name flag" {
    run_skillctl init
    run "$SKILLCTL" pull --name custom-name ":::invalid:::"
    # It should fail on the image ref, not the flag
    [ "$status" -ne 0 ]
    [[ "$output" != *"unknown flag"* ]]
}

@test "pull downloads and extracts a skill" {
    run_skillctl init
    run_skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # Verify extracted files
    [ -f skills/skillctl-example-skill/skill.md ]
    [ -f skills/skillctl-example-skill/counter.txt ]
    [[ "$(cat skills/skillctl-example-skill/counter.txt)" == "1" ]]

    # Verify manifest was updated
    grep -q "skillctl-example-skill" skillctl.yaml
    grep -q "quay.io/vrothberg/skillctl-example-skill:v1.0.0" skillctl.yaml

    # Verify list shows the pulled skill
    run_skillctl list
    [ "$status" -eq 0 ]
    [[ "$output" =~ "skillctl-example-skill" ]]
    [[ "$output" =~ "quay.io/vrothberg/skillctl-example-skill:v1.0.0" ]]
}

@test "pull skips extraction when digest unchanged" {
    run_skillctl init
    run_skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # Second pull of the same image should skip extraction
    run_skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]
    [[ "$output" =~ "is up to date" ]]
}

@test "pull --force re-extracts even when digest unchanged" {
    run_skillctl init
    run_skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # Force pull should re-extract
    run_skillctl pull --force quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled" ]]
    [[ "$output" != *"is up to date"* ]]
    [ -f skills/skillctl-example-skill/counter.txt ]
}

@test "pull two versions with --name" {
    run_skillctl init
    run_skillctl pull --name skill-v1 quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]
    run_skillctl pull --name skill-v2 quay.io/vrothberg/skillctl-example-skill:v1.0.1
    [ "$status" -eq 0 ]

    # Both extracted
    [ -f skills/skill-v1/counter.txt ]
    [ -f skills/skill-v2/counter.txt ]

    # Different content
    [[ "$(cat skills/skill-v1/counter.txt)" == "1" ]]
    [[ "$(cat skills/skill-v2/counter.txt)" == "2" ]]

    # Verify list shows both skills
    run_skillctl list
    [ "$status" -eq 0 ]
    [[ "$output" =~ "skill-v1" ]]
    [[ "$output" =~ "skill-v2" ]]
}

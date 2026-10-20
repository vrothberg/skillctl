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

@test "update skips skills with unchanged digest" {
    run_skillctl init
    run_skillctl pull --name skill-v1 quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    run_skillctl update
    [ "$status" -eq 0 ]
    [[ "$output" =~ "is up to date" ]]
    [[ "$output" =~ "Updated 0 of 1 skill(s)" ]]
}

@test "update --force re-extracts even when digests match" {
    run_skillctl init
    run_skillctl pull --name skill-v1 quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    run_skillctl update --force
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Updated 1 of 1 skill(s)" ]]
    [[ "$(cat skills/skill-v1/counter.txt)" == "1" ]]
}

@test "update re-extracts skills with changed digest" {
    run_skillctl init
    run_skillctl pull --name skill-v1 quay.io/vrothberg/skillctl-example-skill:v1.0.0
    run_skillctl pull --name skill-v2 quay.io/vrothberg/skillctl-example-skill:v1.0.1
    [ "$status" -eq 0 ]

    # Tamper with a digest to simulate Renovate bumping a version
    sed -i'' -e 's/digest: sha256:.*/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' skillctl.yaml

    run_skillctl update
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Updated 2 of 2 skill(s)" ]]

    # Verify files are still correct
    [[ "$(cat skills/skill-v1/counter.txt)" == "1" ]]
    [[ "$(cat skills/skill-v2/counter.txt)" == "2" ]]
}

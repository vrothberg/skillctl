#!/usr/bin/env bats

load helpers

function setup() {
    setup_example_registry
}

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

@test "update fails when a skill has no trust declaration" {
    run_skillctl init
    run_skillctl pull --trust none ${EXAMPLE_IMAGE}:v1.0.0
    [ "$status" -eq 0 ]

    # Simulate a legacy manifest without a trust declaration
    sed -i'' -e '/trust:/d' skillctl.yaml

    run "$SKILLCTL" update
    [ "$status" -ne 0 ]
    [[ "$output" =~ "no trust declaration" ]]

    # Nothing must have been pulled or extracted
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "1" ]]
}

@test "update skips skills with unchanged digest" {
    run_skillctl init
    run_skillctl pull --trust none --name skill-v1 ${EXAMPLE_IMAGE}:v1.0.0
    [ "$status" -eq 0 ]

    run_skillctl update
    [ "$status" -eq 0 ]
    [[ "$output" =~ "is up to date" ]]
    [[ "$output" =~ "Updated 0 of 1 skill(s)" ]]
}

@test "update --force re-extracts even when digests match" {
    run_skillctl init
    run_skillctl pull --trust none --name skill-v1 ${EXAMPLE_IMAGE}:v1.0.0
    [ "$status" -eq 0 ]

    run_skillctl update --force
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Updated 1 of 1 skill(s)" ]]
    [[ "$(cat .agents/skills/skill-v1/counter.txt)" == "1" ]]
}

@test "update re-extracts skills with changed digest" {
    run_skillctl init
    run_skillctl pull --trust none --name skill-v1 ${EXAMPLE_IMAGE}:v1.0.0
    run_skillctl pull --trust none --name skill-v2 ${EXAMPLE_IMAGE}:v1.0.1
    [ "$status" -eq 0 ]

    # Tamper with a digest to simulate Renovate bumping a version
    sed -i'' -e 's/digest: sha256:.*/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' skillctl.yaml

    run_skillctl update
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Updated 2 of 2 skill(s)" ]]

    # Verify files are still correct
    [[ "$(cat .agents/skills/skill-v1/counter.txt)" == "1" ]]
    [[ "$(cat .agents/skills/skill-v2/counter.txt)" == "2" ]]
}

@test "update removes stale files from prior version" {
    run_skillctl init
    run_skillctl pull --trust none ${EXAMPLE_IMAGE}:v1.0.0

    # Simulate a file that existed in v1 but not in v2
    echo "stale" > .agents/skills/skillctl-example-skill/old-helper.txt
    [ -f .agents/skills/skillctl-example-skill/old-helper.txt ]

    # Simulate Renovate bumping the tag
    sed -i'' -e 's|skillctl-example-skill:v1.0.0|skillctl-example-skill:v1.0.1|' skillctl.yaml

    run_skillctl update
    [[ "$output" =~ "Updated 1 of 1 skill(s)" ]]
    [ ! -f .agents/skills/skillctl-example-skill/old-helper.txt ]
    [ -f .agents/skills/skillctl-example-skill/SKILL.md ]
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "2" ]]
}

@test "update handles image tag change (Renovate workflow)" {
    run_skillctl init
    run_skillctl pull --trust none ${EXAMPLE_IMAGE}:v1.0.0
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "1" ]]

    # Simulate Renovate bumping the tag in the manifest
    sed -i'' -e 's|skillctl-example-skill:v1.0.0|skillctl-example-skill:v1.0.1|' skillctl.yaml

    run_skillctl update
    [[ "$output" =~ "Updated 1 of 1 skill(s)" ]]

    # Verify files reflect the new version
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "2" ]]

    # Verify manifest has the new image reference
    grep -q "v1.0.1" skillctl.yaml
}

#!/usr/bin/env bats

load helpers

@test "pull without init fails without extracting" {
    run "$SKILLCTL" pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -ne 0 ]
    # Must not leave orphaned files on disk
    [ ! -d .agents/skills ]
}

@test "pull requires an argument" {
    run "$SKILLCTL" pull
    [ "$status" -ne 0 ]
}

@test "pull rejects invalid image reference" {
    run_skillctl init
    run "$SKILLCTL" pull --trust none ":::invalid:::"
    [ "$status" -ne 0 ]
}

@test "pull accepts --name flag" {
    run_skillctl init
    run "$SKILLCTL" pull --trust none --name custom-name ":::invalid:::"
    # It should fail on the image ref, not the flag
    [ "$status" -ne 0 ]
    [[ "$output" != *"unknown flag"* ]]
}

@test "pull rejects --name with path traversal" {
    run_skillctl init
    run "$SKILLCTL" pull --name "../../etc/evil" quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -ne 0 ]
    [[ "$output" =~ "path separators" ]]
    [ ! -d "../../etc/evil" ]
}

@test "pull without --trust fails with guidance" {
    run_skillctl init
    run "$SKILLCTL" pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -ne 0 ]
    [[ "$output" =~ "no trust declared" ]]
    [[ "$output" =~ "--trust key=KEY.pub" ]]
    [[ "$output" =~ "--trust none" ]]
    [ ! -d .agents/skills/skillctl-example-skill ]
}

@test "pull rejects invalid --trust value" {
    run_skillctl init
    run "$SKILLCTL" pull --trust foo=bar quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -ne 0 ]
    [[ "$output" =~ "invalid trust" ]]
    [[ "$output" =~ "key=KEY.pub" ]]
    [ ! -d .agents/skills/skillctl-example-skill ]
}

@test "pull reuses trust from the manifest when upgrading" {
    run_skillctl init
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # Pulling a new version of a tracked skill without --trust reuses the
    # trust declaration recorded in the manifest.
    run_skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.1
    [[ "$output" =~ "Pulled" ]]
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "2" ]]
    grep -q "trust: none" skillctl.yaml
}

@test "pull --store overrides the store location" {
    run_skillctl init

    run_skillctl pull --trust none --store "$TEST_WORKDIR/custom-store" \
        quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # The generated configuration lives in the flag-provided store...
    [ -f "$TEST_WORKDIR/custom-store/policy.json" ]
    [ -f "$TEST_WORKDIR/custom-store/registries.d/skillctl.yaml" ]

    # ... not in the one from SKILLCTL_STORE.
    [ ! -f "$SKILLCTL_STORE/policy.json" ]
}

@test "pull downloads and extracts a skill" {
    run_skillctl init
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # Verify extracted files
    [ -f .agents/skills/skillctl-example-skill/skill.md ]
    [ -f .agents/skills/skillctl-example-skill/counter.txt ]
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "1" ]]

    # Verify manifest was updated
    grep -q "skillctl-example-skill" skillctl.yaml
    grep -q "quay.io/vrothberg/skillctl-example-skill:v1.0.0" skillctl.yaml
    grep -q "trust: none" skillctl.yaml

    # Verify list shows the pulled skill
    run_skillctl list
    [ "$status" -eq 0 ]
    [[ "$output" =~ "skillctl-example-skill" ]]
    [[ "$output" =~ "quay.io/vrothberg/skillctl-example-skill:v1.0.0" ]]
}

@test "pull skips extraction when digest unchanged" {
    run_skillctl init
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # Second pull of the same image should skip extraction
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]
    [[ "$output" =~ "is up to date" ]]
}

@test "pull --force re-extracts even when digest unchanged" {
    run_skillctl init
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]

    # Force pull should re-extract
    run_skillctl pull --force quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled" ]]
    [[ "$output" != *"is up to date"* ]]
    [ -f .agents/skills/skillctl-example-skill/counter.txt ]
}

@test "pull upgrades a skill in-place to a new version" {
    run_skillctl init
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "1" ]]

    # Pull a newer version of the same skill (same derived name)
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.1
    [[ "$output" =~ "Pulled" ]]
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "2" ]]

    # Manifest should have exactly one entry pointing to the new version
    grep -q "v1.0.1" skillctl.yaml
    run_skillctl list
    local count
    count=$(echo "$output" | grep -c "skillctl-example-skill")
    [ "$count" -eq 1 ]
}

@test "pull removes stale files from prior version" {
    run_skillctl init
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0

    # Simulate a file that existed in v1 but not in v2
    echo "stale" > .agents/skills/skillctl-example-skill/old-helper.txt
    [ -f .agents/skills/skillctl-example-skill/old-helper.txt ]

    # Pull a new version; the stale file must be gone
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.1
    [[ "$output" =~ "Pulled" ]]
    [ ! -f .agents/skills/skillctl-example-skill/old-helper.txt ]
    [ -f .agents/skills/skillctl-example-skill/skill.md ]
    [[ "$(cat .agents/skills/skillctl-example-skill/counter.txt)" == "2" ]]
}

@test "pull two versions with --name" {
    run_skillctl init
    run_skillctl pull --trust none --name skill-v1 quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [ "$status" -eq 0 ]
    run_skillctl pull --trust none --name skill-v2 quay.io/vrothberg/skillctl-example-skill:v1.0.1
    [ "$status" -eq 0 ]

    # Both extracted
    [ -f .agents/skills/skill-v1/counter.txt ]
    [ -f .agents/skills/skill-v2/counter.txt ]

    # Different content
    [[ "$(cat .agents/skills/skill-v1/counter.txt)" == "1" ]]
    [[ "$(cat .agents/skills/skill-v2/counter.txt)" == "2" ]]

    # Verify list shows both skills
    run_skillctl list
    [ "$status" -eq 0 ]
    [[ "$output" =~ "skill-v1" ]]
    [[ "$output" =~ "skill-v2" ]]
}

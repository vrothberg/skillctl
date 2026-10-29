#!/usr/bin/env bats

load helpers

@test "remove requires a project and name and makes force explicit for missing skills" {
    run "$SKILLCTL" remove foo
    [ "$status" -ne 0 ]
    [[ "$output" == *"skillctl init"* ]]
    run "$SKILLCTL" remove
    [ "$status" -ne 0 ]
    [[ "$output" == *"accepts 1 arg(s), received 0"* ]]
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    local before
    before=$(cat skillctl.yaml)
    run "$SKILLCTL" remove nonexistent
    [ "$status" -ne 0 ]
    [[ "$output" == *'skill "nonexistent" not found in manifest'* ]]
    run_skillctl remove --force nonexistent
    [[ "$output" == "" ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
}

@test "named skill versions list independently and remove and rm delete only their own files" {
    start_example_registry
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust none --name skill-v1 "$EXAMPLE_IMAGE:v1.0.0"
    [[ "$output" == *"Pulled skill-v1"* ]]
    run_skillctl pull --trust none --name skill-v2 "$EXAMPLE_IMAGE:v1.0.1"
    [[ "$output" == *"Pulled skill-v2"* ]]
    [[ "$(cat .agents/skills/skill-v1/counter.txt)" == "1" ]]
    [[ "$(cat .agents/skills/skill-v2/counter.txt)" == "2" ]]
    run_skillctl list
    [[ "$output" == *"skill-v1"*"$EXAMPLE_IMAGE:v1.0.0"* ]]
    [[ "$output" == *"skill-v2"*"$EXAMPLE_IMAGE:v1.0.1"* ]]

    run_skillctl remove skill-v1
    [[ "$output" == "Removed skill-v1" ]]
    [ ! -d .agents/skills/skill-v1 ]
    [ -f .agents/skills/skill-v2/SKILL.md ]
    [[ "$(cat .agents/skills/skill-v2/counter.txt)" == "2" ]]
    ! grep -q 'name: skill-v1' skillctl.yaml
    grep -q 'name: skill-v2' skillctl.yaml
    run_skillctl list
    [[ "$output" != *"skill-v1"* ]]
    [[ "$output" == *"skill-v2"* ]]

    run_skillctl rm skill-v2
    [[ "$output" == "Removed skill-v2" ]]
    [ ! -d .agents/skills/skill-v2 ]
    [ -d .agents/skills ]
    ! grep -q 'name:' skillctl.yaml
    run_skillctl list
    [[ "$output" == *"No skills tracked"* ]]
}

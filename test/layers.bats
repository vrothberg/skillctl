#!/usr/bin/env bats

load helpers

@test "signed tar skill preserves its directory tree and executable files" {
    start_registry
    local image="$REGISTRY_HOST/skillctl-example-skill:v1.0.0"
    push_artifact "$image" --sign
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust key="$SIGSTORE_PUB" --tls-verify=false "$image"
    [[ "$output" == *"Pulled skillctl-example-skill"* ]]
    local installed=.agents/skills/skillctl-example-skill
    run diff -r "$EXAMPLE_SKILL_DIR" "$installed"
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
    [ -x "$installed/scripts/read-counter.sh" ]
    [[ "$(stat -c %a "$installed/scripts/read-counter.sh")" == "$(stat -c %a "$EXAMPLE_SKILL_DIR/scripts/read-counter.sh")" ]]
    run "$installed/scripts/read-counter.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == "1" ]]

    mkdir -p "$installed/references/stale"
    echo "stale" > "$installed/references/stale/old.txt"
    run_skillctl update --force --tls-verify=false
    [[ "$output" == *"Updated 1 of 1 skill(s)"* ]]
    [ ! -e "$installed/references/stale" ]
    run diff -r "$EXAMPLE_SKILL_DIR" "$installed"
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
    run "$installed/scripts/read-counter.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == "1" ]]
}

@test "later tar layers override files and update replaces stale nested files" {
    start_registry
    local image="$REGISTRY_HOST/skillctl-example-skill"
    push_artifact "$image:v1.0.0"
    local overlay="$TEST_WORKDIR/overlay"
    mkdir -p "$overlay/references"
    echo "2" > "$overlay/counter.txt"
    echo "updated reference" > "$overlay/references/format.md"
    push_artifact "$image:v1.0.1" --layer "$overlay"
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust none --tls-verify=false "$image:v1.0.0"
    [[ "$output" == *"Pulled skillctl-example-skill"* ]]
    local installed=.agents/skills/skillctl-example-skill
    echo "stale" > "$installed/references/old.txt"
    sed -i 's/:v1.0.0/:v1.0.1/' skillctl.yaml
    run_skillctl update --tls-verify=false
    [[ "$output" == *"Updated 1 of 1 skill(s)"* ]]
    [[ "$(cat "$installed/references/format.md")" == "updated reference" ]]
    [ ! -e "$installed/references/old.txt" ]
    [ -f "$installed/references/examples/output.txt" ]
    run "$installed/scripts/read-counter.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == "2" ]]
}

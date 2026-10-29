#!/usr/bin/env bats

load helpers

@test "pull validates project arguments names and trust before accessing a registry" {
    local image=quay.io/example/skill:v1
    run "$SKILLCTL" pull "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"skillctl init"* ]]
    [ ! -d .agents/skills ]
    run "$SKILLCTL" pull
    [ "$status" -ne 0 ]
    [[ "$output" == *"accepts 1 arg(s), received 0"* ]]

    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run "$SKILLCTL" pull --trust none ':::invalid:::'
    [ "$status" -ne 0 ]
    [[ "$output" == *"parsing image reference"* ]]
    run "$SKILLCTL" pull --trust none --name custom-name ':::invalid:::'
    [ "$status" -ne 0 ]
    [[ "$output" == *"parsing image reference"* ]]
    run "$SKILLCTL" pull --name ../../etc/evil "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"path separators"* ]]
    [ ! -d ../../etc/evil ]
    run "$SKILLCTL" pull "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"no trust declared"* ]]
    [[ "$output" == *"--trust key=KEY.pub"* ]]
    [[ "$output" == *"--trust none"* ]]
    run "$SKILLCTL" pull --trust foo=bar "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"invalid trust"*"key=KEY.pub"* ]]
    [[ "$(ls -A .agents/skills)" == "" ]]
    [ ! -e "$SKILLCTL_STORE" ]
}

@test "pull installs lists skips unchanged files and force restores using an explicit store" {
    start_example_registry
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    local image="$EXAMPLE_IMAGE:v1.0.0"
    local custom_store="$TEST_WORKDIR/custom-store"
    local installed=.agents/skills/skillctl-example-skill
    run_skillctl pull --trust none --store "$custom_store" "$image"
    [[ "$output" == *"Pulled skillctl-example-skill"* ]]
    [[ "$output" == *"Extracted to $installed"* ]]
    [ -f "$custom_store/index.json" ]
    [ ! -e "$SKILLCTL_STORE/index.json" ]
    grep -Fq "image: $image" skillctl.yaml
    grep -q 'trust: none' skillctl.yaml
    run diff -r "$EXAMPLE_SKILL_DIR" "$installed"
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
    run_skillctl list
    [[ "$output" == *"skillctl-example-skill"*"$image"* ]]
    local before
    before=$(cat skillctl.yaml)

    echo 'local edit' > "$installed/counter.txt"
    run_skillctl pull --trust none --store "$custom_store" "$image"
    [[ "$output" == *"is up to date"* ]]
    [[ "$(cat "$installed/counter.txt")" == "local edit" ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]

    run_skillctl pull --force --store "$custom_store" "$image"
    [[ "$output" == *"Pulled skillctl-example-skill"* ]]
    [[ "$output" != *"is up to date"* ]]
    run diff -r "$EXAMPLE_SKILL_DIR" "$installed"
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
}

@test "pull upgrades in place reuses trust and removes stale files" {
    start_example_registry
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust none "$EXAMPLE_IMAGE:v1.0.0"
    [[ "$output" == *"Pulled skillctl-example-skill"* ]]
    local installed=.agents/skills/skillctl-example-skill
    [[ "$(cat "$installed/counter.txt")" == "1" ]]
    local digest
    digest=$(awk '/digest:/ {print $2}' skillctl.yaml)
    echo stale > "$installed/old-helper.txt"

    run_skillctl pull "$EXAMPLE_IMAGE:v1.0.1"
    [[ "$output" == *"Pulled skillctl-example-skill"* ]]
    [[ "$(cat "$installed/counter.txt")" == "2" ]]
    [ ! -e "$installed/old-helper.txt" ]
    [ -f "$installed/SKILL.md" ]
    grep -Fq "image: $EXAMPLE_IMAGE:v1.0.1" skillctl.yaml
    grep -q 'trust: none' skillctl.yaml
    [[ "$(awk '/digest:/ {print $2}' skillctl.yaml)" != "$digest" ]]
    [[ "$(grep -c 'name:' skillctl.yaml)" == "1" ]]
    run_skillctl list
    [[ "$output" == *"$EXAMPLE_IMAGE:v1.0.1"* ]]
    [[ "$(printf '%s\n' "$output" | grep -c skillctl-example-skill)" == "1" ]]
}

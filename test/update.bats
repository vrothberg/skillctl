#!/usr/bin/env bats

load helpers

@test "update validates its project arguments and manifest trust before pulling" {
    run "$SKILLCTL" update
    [ "$status" -ne 0 ]
    [[ "$output" == *"skillctl init"* ]]
    run "$SKILLCTL" update foo
    [ "$status" -ne 0 ]
    [[ "$output" == *'unknown command "foo" for "skillctl update"'* ]]
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl update
    [[ "$output" == "No skills in manifest" ]]

    # No registry is needed to reject a legacy entry without trust.
    cat > skillctl.yaml <<EOF
apiVersion: v1
skills:
    - name: legacy
      image: quay.io/example/skill:v1
      extractPath: .agents/skills/legacy
EOF
    mkdir .agents/skills/legacy
    echo 'local contents' > .agents/skills/legacy/counter.txt
    local before
    before=$(cat skillctl.yaml)
    run "$SKILLCTL" update
    [ "$status" -ne 0 ]
    [[ "$output" == *"no trust declaration"* ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
    [[ "$(cat .agents/skills/legacy/counter.txt)" == "local contents" ]]
    [ ! -e "$SKILLCTL_STORE" ]
}

@test "update skips unchanged skills and restores both forced and changed digests" {
    start_example_registry
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust none --name skill-v1 "$EXAMPLE_IMAGE:v1.0.0"
    [[ "$output" == *"Pulled skill-v1"* ]]
    run_skillctl pull --trust none --name skill-v2 "$EXAMPLE_IMAGE:v1.0.1"
    [[ "$output" == *"Pulled skill-v2"* ]]
    local before
    before=$(cat skillctl.yaml)
    echo local > .agents/skills/skill-v1/counter.txt
    echo local > .agents/skills/skill-v2/counter.txt

    run_skillctl update
    [[ "$output" == *"skill-v1 is up to date"* ]]
    [[ "$output" == *"skill-v2 is up to date"* ]]
    [[ "$output" == *"Updated 0 of 2 skill(s)"* ]]
    [[ "$(cat .agents/skills/skill-v1/counter.txt)" == "local" ]]
    [[ "$(cat .agents/skills/skill-v2/counter.txt)" == "local" ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]

    run_skillctl update --force
    [[ "$output" == *"Updated 2 of 2 skill(s)"* ]]
    [[ "$(cat .agents/skills/skill-v1/counter.txt)" == "1" ]]
    [[ "$(cat .agents/skills/skill-v2/counter.txt)" == "2" ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]

    sed -i 's/digest: sha256:.*/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' skillctl.yaml
    echo local > .agents/skills/skill-v1/counter.txt
    echo local > .agents/skills/skill-v2/counter.txt
    run_skillctl update
    [[ "$output" == *"Updated 2 of 2 skill(s)"* ]]
    [[ "$(cat .agents/skills/skill-v1/counter.txt)" == "1" ]]
    [[ "$(cat .agents/skills/skill-v2/counter.txt)" == "2" ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
}

@test "update follows Renovate tag changes and removes stale files" {
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
    sed -i 's/:v1.0.0/:v1.0.1/' skillctl.yaml

    run_skillctl update
    [[ "$output" == *"Updated 1 of 1 skill(s)"* ]]
    [[ "$(cat "$installed/counter.txt")" == "2" ]]
    [ -f "$installed/SKILL.md" ]
    [ ! -e "$installed/old-helper.txt" ]
    grep -Fq "image: $EXAMPLE_IMAGE:v1.0.1" skillctl.yaml
    grep -q 'trust: none' skillctl.yaml
    [[ "$(awk '/digest:/ {print $2}' skillctl.yaml)" != "$digest" ]]
}

#!/usr/bin/env bats

load helpers

@test "init rejects a skills directory outside the project" {
    mkdir outside
    ln -s "$TEST_WORKDIR/outside" .agents
    run "$SKILLCTL" init
    [ "$status" -ne 0 ]
    [[ "$output" == *"escapes"* ]]
    [ ! -d outside/skills ]
    [ ! -f skillctl.yaml ]
}

@test "pull update and remove reject a symlinked skills parent" {
    run_skillctl init
    run_skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
    [[ "$output" == *"Pulled"* ]]
    mv .agents/skills "$TEST_WORKDIR/outside"
    ln -s "$TEST_WORKDIR/outside" .agents/skills
    local before
    before=$(cat skillctl.yaml)

    for command in pull update remove; do
        case "$command" in
            pull) run "$SKILLCTL" pull --force quay.io/vrothberg/skillctl-example-skill:v1.0.0 ;;
            update) run "$SKILLCTL" update --force ;;
            remove) run "$SKILLCTL" remove skillctl-example-skill ;;
        esac
        [ "$status" -ne 0 ]
        [[ "$output" == *"escapes"* ]]
        [[ "$(cat outside/skillctl-example-skill/counter.txt)" == "1" ]]
        [[ "$(cat skillctl.yaml)" == "$before" ]]
    done
}

#!/usr/bin/env bats

load helpers

@test "action commits shell syntax in messages literally" {
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    git init -q -b test
    git config user.name "test"
    git config user.email "test@example.com"
    git add skillctl.yaml
    git commit -qm "Initial manifest"
    git init -q --bare "$TEST_WORKDIR/remote.git"
    git remote add origin "$TEST_WORKDIR/remote.git"
    git push -qu origin test
    cp "$EXAMPLE_SKILL_DIR/skill.md" .agents/skills/skill.md
    local message='Update $(touch marker) `touch marker2` "quoted"'
    run env SKILLCTL_COMMIT_MESSAGE="$message" bash "$REPO_ROOT/hack/action/commit.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"$message"* ]]
    [[ "$(git log -1 --format=%s)" == "$message" ]]
    [[ "$(git --git-dir="$TEST_WORKDIR/remote.git" log test -1 --format=%s)" == "$message" ]]
    [ ! -e marker ]
    [ ! -e marker2 ]
}

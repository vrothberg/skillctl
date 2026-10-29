#!/usr/bin/env bats
# Live simple-signing compatibility check. Requires network access.

load helpers

REDHAT_KEY_URL="https://security.access.redhat.com/data/63405576.txt"
REDHAT_IMAGE="registry.access.redhat.com/hi/static:latest"

function setup() {
    setup_workdir
    if ! curl -sf --max-time 5 "https://registry.access.redhat.com/v2/" >/dev/null 2>&1; then
        skip "registry.access.redhat.com is unreachable"
    fi
}

@test "Red Hat signatures reject a wrong key and verify before unsafe image extraction" {
    openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out wrong.key 2>/dev/null
    openssl pkey -in wrong.key -pubout -out wrong.pub 2>/dev/null
    curl -sfL "$REDHAT_KEY_URL" -o redhat.pub
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]

    run "$SKILLCTL" pull --trust key=wrong.pub "$REDHAT_IMAGE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"cryptographic signature verification failed"* ]]
    [ ! -d .agents/skills/static ]
    ! grep -q 'name: static' skillctl.yaml

    run "$SKILLCTL" pull --trust key=redhat.pub "$REDHAT_IMAGE"
    [ "$status" -ne 0 ]
    # Reaching extraction proves signature verification succeeded. This
    # container image has absolute symlinks unsuitable for a skill directory.
    [[ "$output" == *"extracting tar layer"* ]]
    [[ "$output" == *"symlink"*"escapes the skill directory"* ]]
    [ ! -d .agents/skills/static ]
    ! grep -q 'name: static' skillctl.yaml
    [ ! -f "$SKILLCTL_STORE/policy.json" ]
}

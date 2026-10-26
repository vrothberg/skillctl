#!/usr/bin/env bats
#
# End-to-end signature verification against registry.access.redhat.com.
# Requires network access; skipped when the registry is unreachable.

load helpers

REDHAT_KEY_URL="https://security.access.redhat.com/data/63405576.txt"
REDHAT_IMAGE="registry.access.redhat.com/hi/static:latest"

function setup() {
    TEST_WORKDIR=$(mktemp -d)
    setup_store
    pushd "$TEST_WORKDIR" > /dev/null

    if ! curl -sf --max-time 5 "https://registry.access.redhat.com/v2/" >/dev/null 2>&1; then
        skip "registry.access.redhat.com is unreachable"
    fi
}

@test "pull Red Hat image with pinned key succeeds" {
    curl -sL "$REDHAT_KEY_URL" -o "$TEST_WORKDIR/redhat.pub"

    run_skillctl init

    run_skillctl pull --trust key="$TEST_WORKDIR/redhat.pub" "$REDHAT_IMAGE"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled static" ]]
    [ -d .agents/skills/static ]
    grep -q "trust:" skillctl.yaml

    # The store must contain a fail-closed policy requiring a signature for
    # exactly this repository, signed by the pinned key.
    [ -f "$SKILLCTL_STORE/policy.json" ]
    grep -q '"type": "reject"' "$SKILLCTL_STORE/policy.json"
    grep -q "\"registry.access.redhat.com/hi/static\"" "$SKILLCTL_STORE/policy.json"
    grep -q '"type": "sigstoreSigned"' "$SKILLCTL_STORE/policy.json"
    grep -q "\"keyPath\": \"$TEST_WORKDIR/redhat.pub\"" "$SKILLCTL_STORE/policy.json"

    # ... and a registries.d enabling sigstore attachments for it.
    [ -f "$SKILLCTL_STORE/registries.d/skillctl.yaml" ]
    grep -q "registry.access.redhat.com/hi/static:" "$SKILLCTL_STORE/registries.d/skillctl.yaml"
    grep -q "use-sigstore-attachments: true" "$SKILLCTL_STORE/registries.d/skillctl.yaml"
}

@test "pull Red Hat image with wrong key fails" {
    local wrong_key_dir="$TEST_WORKDIR/wrong-keys"
    mkdir -p "$wrong_key_dir"
    openssl ecparam -name prime256v1 -genkey -noout 2>/dev/null \
        | openssl pkcs8 -topk8 -nocrypt -out "$wrong_key_dir/wrong.pem" 2>/dev/null
    openssl ec -in "$wrong_key_dir/wrong.pem" -pubout -out "$wrong_key_dir/wrong.pub" 2>/dev/null

    run_skillctl init

    run "$SKILLCTL" pull --trust key="$wrong_key_dir/wrong.pub" "$REDHAT_IMAGE"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "cryptographic signature verification failed" ]]
}

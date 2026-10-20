#!/usr/bin/env bats
#
# End-to-end signature verification against registry.access.redhat.com.
# Requires network access; skipped when the registry is unreachable.

load helpers

REDHAT_KEY_URL="https://security.access.redhat.com/data/63405576.txt"
REDHAT_IMAGE="registry.access.redhat.com/hi/static:latest"

function setup() {
    TEST_WORKDIR=$(mktemp -d)
    pushd "$TEST_WORKDIR" > /dev/null

    if ! curl -sf --max-time 5 "https://registry.access.redhat.com/v2/" >/dev/null 2>&1; then
        skip "registry.access.redhat.com is unreachable"
    fi
}

@test "pull Red Hat image with sigstore verification succeeds" {
    curl -sL "$REDHAT_KEY_URL" -o "$TEST_WORKDIR/redhat.pub"

    write_sigstore_policy "$TEST_WORKDIR/policy.json" "registry.access.redhat.com" "$TEST_WORKDIR/redhat.pub"

    local registries_d="$TEST_WORKDIR/registries.d"
    write_registries_d "$registries_d" "registry.access.redhat.com"

    run_skillctl init

    run_skillctl pull \
        --signature-policy "$TEST_WORKDIR/policy.json" \
        --registries-dir "$registries_d" \
        "$REDHAT_IMAGE"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled static" ]]
    [ -d skills/static ]
}

@test "pull Red Hat image with wrong key fails" {
    local wrong_key_dir="$TEST_WORKDIR/wrong-keys"
    mkdir -p "$wrong_key_dir"
    openssl ecparam -name prime256v1 -genkey -noout 2>/dev/null \
        | openssl pkcs8 -topk8 -nocrypt -out "$wrong_key_dir/wrong.pem" 2>/dev/null
    openssl ec -in "$wrong_key_dir/wrong.pem" -pubout -out "$wrong_key_dir/wrong.pub" 2>/dev/null

    write_sigstore_policy "$TEST_WORKDIR/policy.json" "registry.access.redhat.com" "$wrong_key_dir/wrong.pub"

    local registries_d="$TEST_WORKDIR/registries.d"
    write_registries_d "$registries_d" "registry.access.redhat.com"

    run_skillctl init

    run "$SKILLCTL" pull \
        --signature-policy "$TEST_WORKDIR/policy.json" \
        --registries-dir "$registries_d" \
        "$REDHAT_IMAGE"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "cryptographic signature verification failed" ]]
}

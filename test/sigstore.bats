#!/usr/bin/env bats
#
# Sigstore signature verification tests, inspired by podman's e2e signing
# tests (github.com/containers/podman, test/e2e).

load helpers

@test "pull unsigned artifact fails with sigstore policy" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1"

    local registries_d="$TEST_WORKDIR/registries.d"
    write_sigstore_policy "$TEST_WORKDIR/policy.json" "$REGISTRY_HOST" "$SIGSTORE_PUB"
    write_registries_d "$registries_d" "$REGISTRY_HOST"

    run_skillctl init

    run "$SKILLCTL" pull \
        --signature-policy "$TEST_WORKDIR/policy.json" \
        --registries-dir "$registries_d" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "A signature was required, but no signature exists" ]]
}

@test "pull signed artifact succeeds with sigstore policy" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1" --sign

    local registries_d="$TEST_WORKDIR/registries.d"
    write_sigstore_policy "$TEST_WORKDIR/policy.json" "$REGISTRY_HOST" "$SIGSTORE_PUB"
    write_registries_d "$registries_d" "$REGISTRY_HOST"

    run_skillctl init

    run_skillctl pull \
        --signature-policy "$TEST_WORKDIR/policy.json" \
        --registries-dir "$registries_d" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled" ]]
    [ -f skills/skill/skill.md ]
}

@test "pull artifact signed with wrong key fails" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1" --sign

    # Generate a different key pair
    local wrong_key_dir="$TEST_WORKDIR/wrong-keys"
    mkdir -p "$wrong_key_dir"
    openssl ecparam -name prime256v1 -genkey -noout 2>/dev/null \
        | openssl pkcs8 -topk8 -nocrypt -out "$wrong_key_dir/wrong.pem" 2>/dev/null
    openssl ec -in "$wrong_key_dir/wrong.pem" -pubout -out "$wrong_key_dir/wrong.pub" 2>/dev/null

    local registries_d="$TEST_WORKDIR/registries.d"
    write_sigstore_policy "$TEST_WORKDIR/policy.json" "$REGISTRY_HOST" "$wrong_key_dir/wrong.pub"
    write_registries_d "$registries_d" "$REGISTRY_HOST"

    run_skillctl init

    run "$SKILLCTL" pull \
        --signature-policy "$TEST_WORKDIR/policy.json" \
        --registries-dir "$registries_d" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "cryptographic signature verification failed" ]]
}

@test "pull with permissive policy succeeds for unsigned artifact" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1"

    echo '{"default":[{"type":"insecureAcceptAnything"}]}' > "$TEST_WORKDIR/policy.json"

    run_skillctl init

    run_skillctl pull \
        --signature-policy "$TEST_WORKDIR/policy.json" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled" ]]
    [ -f skills/skill/skill.md ]
}

@test "update respects signature policy" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1"

    echo '{"default":[{"type":"insecureAcceptAnything"}]}' > "$TEST_WORKDIR/permissive.json"

    local registries_d="$TEST_WORKDIR/registries.d"
    write_sigstore_policy "$TEST_WORKDIR/strict.json" "$REGISTRY_HOST" "$SIGSTORE_PUB"
    write_registries_d "$registries_d" "$REGISTRY_HOST"

    run_skillctl init

    # Pull with permissive policy succeeds
    run_skillctl pull \
        --signature-policy "$TEST_WORKDIR/permissive.json" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -eq 0 ]

    # Tamper digest to force re-pull
    sed -i'' -e 's/digest: sha256:.*/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' skillctl.yaml

    # Update with strict policy fails (unsigned artifact)
    run "$SKILLCTL" update \
        --signature-policy "$TEST_WORKDIR/strict.json" \
        --registries-dir "$registries_d" \
        --tls-verify=false
    [ "$status" -ne 0 ]
    [[ "$output" =~ "A signature was required, but no signature exists" ]]
}

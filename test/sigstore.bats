#!/usr/bin/env bats
#
# Sigstore signature verification tests, inspired by podman's e2e signing
# tests (github.com/containers/podman, test/e2e). Trust is declared via
# --trust and recorded in the manifest; skillctl synthesizes the signature
# policy and registries.d configuration from it.

load helpers

@test "pull records a new image reference without replacing unchanged files" {
    start_registry
    push_artifact "$REGISTRY_HOST/skill:v1"
    push_artifact "$REGISTRY_HOST/skill:v2"
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust none --tls-verify=false "$REGISTRY_HOST/skill:v1"
    [[ "$output" == *"Pulled skill"* ]]
    local digest
    digest=$(awk '/digest:/ {print $2}' skillctl.yaml)
    echo "local edit" > .agents/skills/skill/counter.txt

    run_skillctl pull --tls-verify=false "$REGISTRY_HOST/skill:v2"
    [[ "$output" == *"Updated manifest for skill"* ]]
    [[ "$output" == *"skill is up to date"* ]]
    grep -Fq "image: $REGISTRY_HOST/skill:v2" skillctl.yaml
    grep -Fq "digest: $digest" skillctl.yaml
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]
}

@test "pull records changed trust without replacing unchanged files" {
    start_registry
    push_artifact "$REGISTRY_HOST/skill:v1" --sign
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust none --tls-verify=false "$REGISTRY_HOST/skill:v1"
    [[ "$output" == *"Pulled skill"* ]]
    echo "local edit" > .agents/skills/skill/counter.txt

    run_skillctl pull --trust key="$SIGSTORE_PUB" --tls-verify=false "$REGISTRY_HOST/skill:v1"
    [[ "$output" == *"Updated manifest for skill"* ]]
    [[ "$output" == *"skill is up to date"* ]]
    grep -Fq "key: $SIGSTORE_PUB" skillctl.yaml
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]

    run_skillctl pull --trust none --tls-verify=false "$REGISTRY_HOST/skill:v1"
    [[ "$output" == *"Updated manifest for skill"* ]]
    grep -Fq "trust: none" skillctl.yaml
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]
}

@test "pull keeps existing metadata when new trust verification fails" {
    start_registry
    push_artifact "$REGISTRY_HOST/skill:v1"
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust none --tls-verify=false "$REGISTRY_HOST/skill:v1"
    [[ "$output" == *"Pulled skill"* ]]
    local before
    before=$(cat skillctl.yaml)

    run "$SKILLCTL" pull --trust key="$SIGSTORE_PUB" --tls-verify=false "$REGISTRY_HOST/skill:v1"
    [ "$status" -ne 0 ]
    [[ "$output" == *"A signature was required, but no signature exists"* ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "1" ]]
}

@test "tagged digest references preserve input and verify by digest on pull and update" {
    start_registry
    push_artifact "$REGISTRY_HOST/skill:v1" --sign
    run_skillctl init
    run_skillctl pull --trust key="$SIGSTORE_PUB" --tls-verify=false "$REGISTRY_HOST/skill:v1"
    [[ "$output" =~ "Pulled skill" ]]
    local digest
    digest=$(awk '/digest:/ {print $2}' skillctl.yaml)
    local image="$REGISTRY_HOST/skill:nonexistent@$digest"

    # The nonexistent tag must be ignored; the digest selects the signed artifact.
    run_skillctl pull --force --tls-verify=false "$image"
    [[ "$output" =~ "Pulled skill" ]]
    [ -f .agents/skills/skill/skill.md ]
    grep -Fq "image: $image" skillctl.yaml

    rm .agents/skills/skill/skill.md
    run_skillctl update --force --tls-verify=false
    [[ "$output" =~ "Updated 1 of 1 skill(s)" ]]
    [ -f .agents/skills/skill/skill.md ]
    grep -Fq "image: $image" skillctl.yaml
}

@test "pull unsigned artifact with pinned key fails" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1"

    run_skillctl init

    run "$SKILLCTL" pull \
        --trust key="$SIGSTORE_PUB" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "A signature was required, but no signature exists" ]]
    [ ! -d .agents/skills/skill ]
}

@test "pull signed artifact with pinned key succeeds" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1" --sign

    run_skillctl init

    run_skillctl pull \
        --trust key="$SIGSTORE_PUB" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled" ]]
    [ -f .agents/skills/skill/skill.md ]

    # The trust declaration is recorded in the manifest
    grep -q "trust:" skillctl.yaml

    # Trust configuration must not be shared between invocations.
    [ ! -f "$SKILLCTL_STORE/policy.json" ]
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

    run_skillctl init

    run "$SKILLCTL" pull \
        --trust key="$wrong_key_dir/wrong.pub" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "cryptographic signature verification failed" ]]
    [ ! -d .agents/skills/skill ]
}

@test "pull unsigned artifact with --trust none succeeds" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1"

    run_skillctl init

    run_skillctl pull \
        --trust none \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Pulled" ]]
    [ -f .agents/skills/skill/skill.md ]

    [ ! -f "$SKILLCTL_STORE/policy.json" ]
}

@test "update verifies against the pinned key from the manifest" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1" --sign

    run_skillctl init

    run_skillctl pull \
        --trust key="$SIGSTORE_PUB" \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -eq 0 ]

    # Tamper digest to force a re-pull
    sed -i'' -e 's/digest: sha256:.*/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' skillctl.yaml

    run_skillctl update --tls-verify=false
    [[ "$output" =~ "Updated 1 of 1 skill(s)" ]]
    [ -f .agents/skills/skill/skill.md ]
}

@test "update fails when the manifest pins a key the artifact does not match" {
    start_registry

    push_artifact "$REGISTRY_HOST/skill:v1"

    run_skillctl init

    run_skillctl pull \
        --trust none \
        --tls-verify=false \
        "$REGISTRY_HOST/skill:v1"
    [ "$status" -eq 0 ]

    # Rewrite the manifest to pin a key for the unsigned artifact
    cat > skillctl.yaml <<EOF
apiVersion: v1
skills:
    - name: skill
      image: $REGISTRY_HOST/skill:v1
      digest: sha256:0000000000000000000000000000000000000000000000000000000000000000
      extractPath: .agents/skills/skill
      trust:
          key: $SIGSTORE_PUB
EOF

    run "$SKILLCTL" update --tls-verify=false
    [ "$status" -ne 0 ]
    [[ "$output" =~ "A signature was required, but no signature exists" ]]
}

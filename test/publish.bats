#!/usr/bin/env bats

load helpers

@test "publish requires both image and directory arguments" {
    run "$SKILLCTL" publish --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"accepts 2 arg(s), received 0"* ]]
    run "$SKILLCTL" publish quay.io/team/skill:v1 --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"accepts 2 arg(s), received 1"* ]]
}

@test "publish requires explicit signing and a tag without a digest" {
    run "$SKILLCTL" publish quay.io/team/skill:v1 "$EXAMPLE_SKILL_DIR"
    [ "$status" -ne 0 ]
    [[ "$output" == *"specify --sign key=PRIVATE_KEY or --sign none"* ]]
    run "$SKILLCTL" publish quay.io/team/skill "$EXAMPLE_SKILL_DIR" --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"requires an explicit tag"* ]]
    run "$SKILLCTL" publish "quay.io/team/skill:v1@sha256:$(printf '%064d' 0)" "$EXAMPLE_SKILL_DIR" --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"must not contain a digest"* ]]
}

@test "publish rejects invalid signing declarations" {
    run "$SKILLCTL" publish quay.io/team/skill:v1 . --sign key=a --sign key=b
    [ "$status" -ne 0 ]
    [[ "$output" == *'duplicate --sign option "key"'* ]]
    run "$SKILLCTL" publish quay.io/team/skill:v1 . --sign none --sign key=a
    [ "$status" -ne 0 ]
    [[ "$output" == *"none cannot be combined"* ]]
    run "$SKILLCTL" publish quay.io/team/skill:v1 . --sign passphrase-file=a
    [ "$status" -ne 0 ]
    [[ "$output" == *"requires --sign key=PRIVATE_KEY"* ]]
    run "$SKILLCTL" publish quay.io/team/skill:v1 . --sign unknown=a
    [ "$status" -ne 0 ]
    [[ "$output" == *'invalid --sign option "unknown"'* ]]
}

@test "publish unsigned directory without skill metadata preserves every file" {
    start_registry
    local source="$TEST_WORKDIR/source"
    mkdir -p "$source/.hidden/empty" "$TEST_WORKDIR/tmp"
    ln -s "$TEST_WORKDIR/tmp" "$TEST_WORKDIR/tmp-link"
    export TMPDIR="$TEST_WORKDIR/tmp-link"
    printf '#!/bin/sh\necho published\n' > "$source/run.sh"
    chmod 751 "$source/run.sh"
    echo "hidden value" > "$source/.hidden/value"
    echo '.hidden' > "$source/.gitignore"
    ln -s run.sh "$source/link"
    ln "$source/run.sh" "$source/hardlink"
    local image="$REGISTRY_HOST/arbitrary-directory:v1"
    run_skillctl publish "$image" "$source" --sign none --tls-verify=false
    [[ "$output" == *"Published $image"* ]]
    [[ "$output" == *"Digest: sha256:"* ]]
    [[ "$output" == *"Reference: $image@sha256:"* ]]
    local reference
    reference=$(printf '%s\n' "$output" | sed -n 's/^Reference: //p')
    [ ! -e skillctl.yaml ]
    [ ! -e .agents ]
    [ ! -e "$SKILLCTL_STORE" ]
    [[ "$(ls -A "$TMPDIR")" == "" ]]

    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull "$reference" --trust none --tls-verify=false
    [[ "$output" == *"Pulled arbitrary-directory"* ]]
    local installed=.agents/skills/arbitrary-directory
    run diff -r "$source" "$installed"
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
    [[ "$(stat -c %a "$installed/run.sh")" == "751" ]]
    [ "$installed/run.sh" -ef "$installed/hardlink" ]
    run "$installed/link"
    [ "$status" -eq 0 ]
    [[ "$output" == "published" ]]
}

@test "publish signs an artifact verified by skopeo and skillctl" {
    start_registry
    local image="$REGISTRY_HOST/skill:v1"
    # String-array flags must preserve commas and equals signs in paths.
    local passphrase="$TEST_WORKDIR/passphrase,with=punctuation"
    cp "$SIGSTORE_PASS" "$passphrase"
    run_skillctl publish "$image" "$EXAMPLE_SKILL_DIR" \
        --sign "key=$SIGSTORE_KEY" --sign "passphrase-file=$passphrase" --tls-verify=false
    [[ "$output" == *"Published $image"* ]]
    local reference
    reference=$(printf '%s\n' "$output" | sed -n 's/^Reference: //p')
    [[ "$reference" == "$image@sha256:"* ]]

    # Verify with an independent tool and policy, without skillctl's
    # generated verification configuration or ambient signature policy.
    local trusted_key="$TEST_WORKDIR/skopeo-trusted.pub"
    cp "$SIGSTORE_PUB" "$trusted_key"
    local policy="$TEST_WORKDIR/skopeo-policy.json"
    cat > "$policy" <<EOF
{
  "default": [{"type": "reject"}],
  "transports": {
    "docker": {
      "$REGISTRY_HOST/skill": [{"type": "sigstoreSigned", "keyPath": "$trusted_key"}]
    }
  }
}
EOF
    local registries_d="$TEST_WORKDIR/skopeo-registries.d"
    write_registries_d "$registries_d" "$REGISTRY_HOST"
    # OCI layouts cannot store signatures. Skopeo still verifies the source
    # signature before removing it from the destination copy.
    run skopeo copy --src-tls-verify=false --remove-signatures \
        --policy "$policy" --registries.d "$registries_d" \
        "docker://$image" "oci:$TEST_WORKDIR/skopeo-verified:v1"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Writing manifest to image destination"* ]]
    local digest="${reference##*@}"
    run sha256sum "$TEST_WORKDIR/skopeo-verified/blobs/sha256/${digest#sha256:}"
    [ "$status" -eq 0 ]
    [[ "$output" == "${digest#sha256:}  "* ]]

    # A wrong pinned key must fail, proving verification was enforced.
    openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 \
        -out "$TEST_WORKDIR/wrong.key" 2>/dev/null
    openssl pkey -in "$TEST_WORKDIR/wrong.key" -pubout -out "$trusted_key" 2>/dev/null
    run skopeo copy --src-tls-verify=false --remove-signatures \
        --policy "$policy" --registries.d "$registries_d" \
        "docker://$image" "oci:$TEST_WORKDIR/skopeo-rejected:v1"
    [ "$status" -ne 0 ]
    [[ "$output" == *"cryptographic signature verification failed"* ]]

    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull "$reference" --trust "key=$SIGSTORE_PUB" --tls-verify=false
    [[ "$output" == *"Pulled skill"* ]]
    run diff -r "$EXAMPLE_SKILL_DIR" .agents/skills/skill
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
}

@test "publish produces the same remote digest for equivalent trees" {
    start_registry
    mkdir first second
    echo a > first/a
    echo b > first/b
    echo b > second/b
    echo a > second/a
    touch -t 200001010000 first first/a first/b
    touch -t 202001010000 second second/a second/b
    chmod 755 first second
    chmod 640 first/* second/*
    local image="$REGISTRY_HOST/skill"
    run_skillctl publish "$image:v1" first --sign none --tls-verify=false
    [[ "$output" == *"Published $image:v1"* ]]
    local first_digest
    first_digest=$(printf '%s\n' "$output" | sed -n 's/^Digest: //p')
    [[ "$first_digest" == sha256:* ]]
    run_skillctl publish "$image:v2" second --tls-verify=false \
        --sign "key=$SIGSTORE_KEY" --sign "passphrase-file=$SIGSTORE_PASS"
    [[ "$output" == *"Published $image:v2"* ]]
    [[ "$output" == *"Digest: $first_digest"* ]]
    local raw="$TEST_WORKDIR/remote-manifest"
    curl -sk -H 'Accept: application/vnd.oci.image.manifest.v1+json' \
        "https://$REGISTRY_HOST/v2/skill/manifests/v2" > "$raw"
    [[ "sha256:$(sha256sum "$raw" | cut -d' ' -f1)" == "$first_digest" ]]
}

@test "publish signs with an unencrypted key without a passphrase file" {
    start_registry
    openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out key.pem 2>/dev/null
    openssl pkey -in key.pem -pubout -out key.pub 2>/dev/null
    local image="$REGISTRY_HOST/skill:v1"
    run_skillctl publish "$image" "$EXAMPLE_SKILL_DIR" --sign key=key.pem --tls-verify=false
    [[ "$output" == *"Published $image"* ]]
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull "$image" --trust key=key.pub --tls-verify=false
    [[ "$output" == *"Pulled skill"* ]]
    run diff -r "$EXAMPLE_SKILL_DIR" .agents/skills/skill
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
}

@test "example publisher uses skillctl publish" {
    start_registry
    run env SKILLCTL="$SKILLCTL" REGISTRY="$REGISTRY_HOST" \
        SSL_CERT_FILE="$TEST_WORKDIR/registry/certs/registry.crt" \
        bash "$REPO_ROOT/hack/build-example-skill.sh" v1
    [ "$status" -eq 0 ]
    [[ "$output" == *"Published $REGISTRY_HOST/skillctl-example-skill:v1"* ]]
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull "$REGISTRY_HOST/skillctl-example-skill:v1" --trust none --tls-verify=false
    [[ "$output" == *"Pulled skillctl-example-skill"* ]]
    run diff -r "$EXAMPLE_SKILL_DIR" .agents/skills/skillctl-example-skill
    [ "$status" -eq 0 ]
    [[ "$output" == "" ]]
}

@test "publish reports upload failures and removes private staging" {
    start_registry
    mkdir tmp
    export TMPDIR="$TEST_WORKDIR/tmp"
    run "$SKILLCTL" publish "$REGISTRY_HOST/skill:v1" "$EXAMPLE_SKILL_DIR" --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"publishing"*"may have been partially uploaded"* ]]
    [[ "$output" == *"x509"* ]]
    [[ "$output" != *"Published "* ]]
    [[ "$(ls -A "$TMPDIR")" == "" ]]
}

@test "publish replaces an existing tag without force" {
    start_registry
    mkdir source
    echo 1 > source/counter.txt
    local image="$REGISTRY_HOST/skill:v1"
    run_skillctl publish "$image" source --sign none --tls-verify=false
    [[ "$output" == *"Published $image"* ]]
    local first="$output"
    echo 2 > source/counter.txt
    run_skillctl publish "$image" source --sign none --tls-verify=false
    [[ "$output" == *"Published $image"* ]]
    [[ "$output" != "$first" ]]
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull "$image" --trust none --tls-verify=false
    [[ "$output" == *"Pulled skill"* ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "2" ]]
}

@test "publish rejects bad signing inputs before uploading without prompting" {
    start_registry
    local image="$REGISTRY_HOST/skill:v1"
    run "$SKILLCTL" publish "$image" "$EXAMPLE_SKILL_DIR" \
        --sign "key=$SIGSTORE_KEY" --tls-verify=false </dev/null
    [ "$status" -ne 0 ]
    [[ "$output" == *"loading signing key"*"--sign passphrase-file=FILE"* ]]
    echo 'not-the-passphrase' > wrong-passphrase
    run "$SKILLCTL" publish "$image" "$EXAMPLE_SKILL_DIR" \
        --sign "key=$SIGSTORE_KEY" --sign passphrase-file=wrong-passphrase --tls-verify=false </dev/null
    [ "$status" -ne 0 ]
    [[ "$output" == *"loading signing key"* ]]
    [[ "$output" != *"not-the-passphrase"* ]]
    run "$SKILLCTL" publish "$image" "$EXAMPLE_SKILL_DIR" \
        --sign key=missing --sign "passphrase-file=$SIGSTORE_PASS" --tls-verify=false
    [ "$status" -ne 0 ]
    [[ "$output" == *"reading private key"* ]]
    run curl -sk -o /dev/null -w '%{http_code}' "https://$REGISTRY_HOST/v2/skill/manifests/v1"
    [ "$status" -eq 0 ]
    [[ "$output" == "404" ]]
    [ ! -e "$SKILLCTL_STORE" ]
}

@test "publish rejects missing sources and unsafe entries and cleans staging" {
    mkdir tmp
    export TMPDIR="$TEST_WORKDIR/tmp"
    run "$SKILLCTL" publish quay.io/team/skill:v1 missing --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"opening source directory"* ]]
    mkdir source
    ln -s ../outside source/escape
    run "$SKILLCTL" publish quay.io/team/skill:v1 source --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"symlink"*"escapes the skill directory"* ]]
    rm source/escape
    mkfifo source/pipe
    run "$SKILLCTL" publish quay.io/team/skill:v1 source --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"unsupported source entry"* ]]
    run "$SKILLCTL" publish quay.io/team/skill:v1 "$TEST_WORKDIR" --sign none
    [ "$status" -ne 0 ]
    [[ "$output" == *"set TMPDIR outside the source directory"* ]]
    [[ "$(ls -A "$TMPDIR")" == "" ]]
    [ ! -e "$SKILLCTL_STORE" ]
}

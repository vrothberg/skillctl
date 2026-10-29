#!/usr/bin/env bats
# Trust is declared via --trust and recorded in the manifest; skillctl
# synthesizes signature policy and registries.d from it.

load helpers

@test "unsigned skills require explicit opt out and reject pinned keys on pull and update" {
    start_registry
    local image="$REGISTRY_HOST/skill:v1"
    push_artifact "$image"
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]

    run "$SKILLCTL" pull --trust "key=$SIGSTORE_PUB" --tls-verify=false "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"A signature was required, but no signature exists"* ]]
    [ ! -d .agents/skills/skill ]
    ! grep -q 'name: skill' skillctl.yaml

    run_skillctl pull --trust none --tls-verify=false "$image"
    [[ "$output" == *"Pulled skill"* ]]
    [ -f .agents/skills/skill/SKILL.md ]
    [ ! -f "$SKILLCTL_STORE/policy.json" ]
    local before
    before=$(cat skillctl.yaml)

    # A failed trust change must preserve both the manifest and files.
    run "$SKILLCTL" pull --trust "key=$SIGSTORE_PUB" --tls-verify=false "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"A signature was required, but no signature exists"* ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "1" ]]

    sed -i "s|trust: none|trust:\n          key: $SIGSTORE_PUB|" skillctl.yaml
    before=$(cat skillctl.yaml)
    echo local > .agents/skills/skill/counter.txt
    run "$SKILLCTL" update --tls-verify=false
    [ "$status" -ne 0 ]
    [[ "$output" == *"A signature was required, but no signature exists"* ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local" ]]
}

@test "signed skills change trust without extraction verify updates and reject wrong keys" {
    start_registry
    local image="$REGISTRY_HOST/skill:v1"
    push_artifact "$image" --sign
    openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out wrong.key 2>/dev/null
    openssl pkey -in wrong.key -pubout -out wrong.pub 2>/dev/null
    local wrong_pub="$TEST_WORKDIR/wrong.pub"
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]

    run "$SKILLCTL" pull --trust "key=$wrong_pub" --tls-verify=false "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"cryptographic signature verification failed"* ]]
    [ ! -d .agents/skills/skill ]

    run_skillctl pull --trust none --tls-verify=false "$image"
    [[ "$output" == *"Pulled skill"* ]]
    echo 'local edit' > .agents/skills/skill/counter.txt
    run_skillctl pull --trust "key=$SIGSTORE_PUB" --tls-verify=false "$image"
    [[ "$output" == *"Updated manifest for skill"* ]]
    [[ "$output" == *"skill is up to date"* ]]
    grep -Fq "key: $SIGSTORE_PUB" skillctl.yaml
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]
    local before
    before=$(cat skillctl.yaml)
    run "$SKILLCTL" pull --trust "key=$wrong_pub" --tls-verify=false "$image"
    [ "$status" -ne 0 ]
    [[ "$output" == *"cryptographic signature verification failed"* ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]

    # Changing back to none also records metadata without replacing files.
    run_skillctl pull --trust none --tls-verify=false "$image"
    [[ "$output" == *"Updated manifest for skill"* ]]
    grep -q 'trust: none' skillctl.yaml
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]
    run_skillctl pull --trust "key=$SIGSTORE_PUB" --tls-verify=false "$image"
    [[ "$output" == *"Updated manifest for skill"* ]]
    grep -Fq "key: $SIGSTORE_PUB" skillctl.yaml

    # An ordinary update with a changed digest must verify using the key
    # from the manifest and restore the artifact contents.
    sed -i 's/digest: sha256:.*/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' skillctl.yaml
    run_skillctl update --tls-verify=false
    [[ "$output" == *"Updated 1 of 1 skill(s)"* ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "1" ]]
    [ -f .agents/skills/skill/SKILL.md ]
    [ ! -f "$SKILLCTL_STORE/policy.json" ]

    # Verification is required even if the recorded digest is unchanged.
    sed -i "s|key: $SIGSTORE_PUB|key: $wrong_pub|" skillctl.yaml
    before=$(cat skillctl.yaml)
    echo local > .agents/skills/skill/counter.txt
    run "$SKILLCTL" update --tls-verify=false
    [ "$status" -ne 0 ]
    [[ "$output" == *"cryptographic signature verification failed"* ]]
    [[ "$(cat skillctl.yaml)" == "$before" ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local" ]]
}

@test "tag and digest aliases preserve metadata local edits and port qualified skill names" {
    start_registry
    local repository="$REGISTRY_HOST/skill"
    push_artifact "$repository:v1" --sign
    push_artifact "$repository:v2" --sign
    run_skillctl init
    [[ "$output" == *"Initialized skillctl project"* ]]
    run_skillctl pull --trust "key=$SIGSTORE_PUB" --tls-verify=false "$repository:v1"
    [[ "$output" == *"Pulled skill"* ]]
    local digest
    digest=$(awk '/digest:/ {print $2}' skillctl.yaml)
    echo 'local edit' > .agents/skills/skill/counter.txt

    run_skillctl pull --tls-verify=false "$repository:v2"
    [[ "$output" == *"Updated manifest for skill"* ]]
    [[ "$output" == *"skill is up to date"* ]]
    grep -Fq "image: $repository:v2" skillctl.yaml
    grep -Fq "digest: $digest" skillctl.yaml
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]

    run_skillctl pull --tls-verify=false "$repository@$digest"
    [[ "$output" == *"Updated manifest for skill"* ]]
    [[ "$output" == *"skill is up to date"* ]]
    grep -Fq "image: $repository@$digest" skillctl.yaml
    [[ "$(grep -c 'name:' skillctl.yaml)" == "1" ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "local edit" ]]
    [ ! -d .agents/skills/localhost ]

    # The nonexistent tag must be ignored; the digest selects the content.
    local image="$repository:nonexistent@$digest"
    run_skillctl pull --force --tls-verify=false "$image"
    [[ "$output" == *"Pulled skill"* ]]
    [[ "$(cat .agents/skills/skill/counter.txt)" == "1" ]]
    [ -f .agents/skills/skill/SKILL.md ]
    grep -Fq "image: $image" skillctl.yaml
    rm .agents/skills/skill/SKILL.md
    run_skillctl update --force --tls-verify=false
    [[ "$output" == *"Updated 1 of 1 skill(s)"* ]]
    [ -f .agents/skills/skill/SKILL.md ]
    grep -Fq "image: $image" skillctl.yaml
}

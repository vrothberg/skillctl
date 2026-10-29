# Registry lifecycle, sigstore policy, and registries.d helpers are based on
# podman's test infrastructure (github.com/containers/podman, test/e2e and
# test/system).

SKILLCTL=${SKILLCTL:-$(pwd)/bin/skillctl}
TEST_WORKDIR=""
REGISTRY_PID=""
REGISTRY_PORT=""
REGISTRY_HOST=""

TESTDATA_DIR="${BATS_TEST_DIRNAME}/testdata"
SIGSTORE_KEY="${TESTDATA_DIR}/sigstore.key"
SIGSTORE_PUB="${TESTDATA_DIR}/sigstore.pub"
SIGSTORE_PASS="${TESTDATA_DIR}/sigstore.pass"

REPO_ROOT="$(cd "$(dirname "$SKILLCTL")/.." && pwd)"
EXAMPLE_SKILL_DIR="${REPO_ROOT}/hack/example-skill"

# Use an isolated store per test so the suite never touches the developer's
# real store. Trust configuration is isolated separately per operation.
function setup_store() {
    export SKILLCTL_STORE="${TEST_WORKDIR}/store"
}

function setup_workdir() {
    TEST_WORKDIR=$(mktemp -d)
    setup_store
    pushd "$TEST_WORKDIR" > /dev/null
}

function setup() {
    setup_workdir
}

# Build both versions from the checked-in example tree.
function setup_example_registry() {
    setup_workdir
    start_registry
    export SSL_CERT_FILE="$TEST_WORKDIR/registry/certs/registry.crt"
    EXAMPLE_IMAGE="$REGISTRY_HOST/skillctl-example-skill"
    local version
    for version in 0 1; do
        local source="$TEST_WORKDIR/example-v1.0.$version"
        cp -a "$EXAMPLE_SKILL_DIR" "$source"
        echo "$((version + 1))" > "$source/counter.txt"
        push_artifact "$EXAMPLE_IMAGE:v1.0.$version" --files "$source" >/dev/null
    done
}

function teardown() {
    stop_registry
    popd > /dev/null
    rm -rf "$TEST_WORKDIR"
}

function run_skillctl() {
    run "$SKILLCTL" "$@"
    if [ "$status" -ne 0 ]; then
        echo "run_skillctl failed:"
        echo "  CLI: $SKILLCTL $*"
        echo "  EXIT: $status"
        echo "  OUT: $output"
        return 1
    fi
}

# --- Registry helpers ---

function start_registry() {
    local registry_dir="$TEST_WORKDIR/registry"
    local cert_dir="$registry_dir/certs"
    local storage_dir="$registry_dir/storage"
    mkdir -p "$storage_dir" "$cert_dir"

    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
        -nodes -days 1 -subj "/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" \
        -keyout "$cert_dir/registry.key" \
        -out "$cert_dir/registry.crt" \
        2>/dev/null

    REGISTRY_PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()')
    REGISTRY_HOST="localhost:${REGISTRY_PORT}"

    cat > "$registry_dir/config.yml" <<EOF
version: 0.1
storage:
  filesystem:
    rootdirectory: $storage_dir
http:
  addr: 0.0.0.0:${REGISTRY_PORT}
  tls:
    certificate: $cert_dir/registry.crt
    key: $cert_dir/registry.key
EOF

    registry serve "$registry_dir/config.yml" &>"$registry_dir/registry.log" &
    REGISTRY_PID=$!

    local retries=0
    while ! curl -sk "https://${REGISTRY_HOST}/v2/" >/dev/null 2>&1; do
        retries=$((retries + 1))
        if [ "$retries" -gt 50 ]; then
            echo "Registry failed to start on port $REGISTRY_PORT"
            cat "$registry_dir/registry.log"
            return 1
        fi
        sleep 0.1
    done
}

function stop_registry() {
    if [ -n "${REGISTRY_PID:-}" ]; then
        kill "$REGISTRY_PID" 2>/dev/null || true
        wait "$REGISTRY_PID" 2>/dev/null || true
        REGISTRY_PID=""
    fi
}

function build_oci_layout() {
    local dir="$1"
    shift

    mkdir -p "$dir/blobs/sha256"
    echo '{"imageLayoutVersion":"1.0.0"}' > "$dir/oci-layout"

    # Config: empty JSON descriptor
    local config='{}'
    local config_hash
    config_hash=$(printf '%s' "$config" | sha256sum | cut -d' ' -f1)
    local config_size=${#config}
    printf '%s' "$config" > "$dir/blobs/sha256/$config_hash"

    # Each layer is a deterministic compressed tar of a directory tree.
    local layers=""
    local index=0
    local files_dir
    for files_dir in "$@"; do
        local layer="$dir/layer-$index.tar.gz"
        tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner \
            -czf "$layer" -C "$files_dir" .
        local hash
        hash=$(sha256sum "$layer" | cut -d' ' -f1)
        local size
        size=$(wc -c < "$layer" | tr -d ' ')
        cp "$layer" "$dir/blobs/sha256/$hash"

        [ -n "$layers" ] && layers="$layers,"
        layers="${layers}{\"mediaType\":\"application/vnd.oci.image.layer.v1.tar+gzip\",\"digest\":\"sha256:${hash}\",\"size\":${size}}"
        index=$((index + 1))
    done

    # Manifest
    local manifest
    manifest=$(printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.empty.v1+json","digest":"sha256:%s","size":%d},"layers":[%s]}' \
        "$config_hash" "$config_size" "$layers")
    local manifest_hash
    manifest_hash=$(printf '%s' "$manifest" | sha256sum | cut -d' ' -f1)
    local manifest_size=${#manifest}
    printf '%s' "$manifest" > "$dir/blobs/sha256/$manifest_hash"

    # Index
    printf '{"schemaVersion":2,"manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:%s","size":%d}]}' \
        "$manifest_hash" "$manifest_size" > "$dir/index.json"
}

function push_artifact() {
    local ref="$1"
    shift

    local sign_args=()
    local files_dir="$EXAMPLE_SKILL_DIR"
    local extra_layers=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --files)
                files_dir="$2"
                shift 2
                ;;
            --layer)
                extra_layers+=("$2")
                shift 2
                ;;
            --sign)
                sign_args=(
                    --sign-by-sigstore-private-key "$SIGSTORE_KEY"
                    --sign-passphrase-file "$SIGSTORE_PASS"
                )
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    local oci_dir="$TEST_WORKDIR/oci-push"
    rm -rf "$oci_dir"
    build_oci_layout "$oci_dir" "$files_dir" "${extra_layers[@]}"

    local registries_d="$TEST_WORKDIR/push-registries.d"
    write_registries_d "$registries_d" "$REGISTRY_HOST"

    local permissive_policy="$TEST_WORKDIR/permissive-policy.json"
    echo '{"default":[{"type":"insecureAcceptAnything"}]}' > "$permissive_policy"

    skopeo copy \
        --dest-tls-verify=false \
        --policy "$permissive_policy" \
        --registries.d "$registries_d" \
        "${sign_args[@]}" \
        "oci:${oci_dir}" \
        "docker://${ref}"
}

function write_registries_d() {
    local dir="$1"
    local registry_host="$2"
    mkdir -p "$dir"
    cat > "$dir/sigstore.yaml" <<EOF
docker:
    ${registry_host}:
        use-sigstore-attachments: true
EOF
}

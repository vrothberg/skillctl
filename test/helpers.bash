SKILLCTL=${SKILLCTL:-$(pwd)/bin/skillctl}
TEST_WORKDIR=""

function setup() {
    TEST_WORKDIR=$(mktemp -d)
    pushd "$TEST_WORKDIR" > /dev/null
}

function teardown() {
    popd > /dev/null
    rm -rf "$TEST_WORKDIR"
}

function run_skillctl() {
    run "$SKILLCTL" "$@"
    if [ "$status" -ne 0 ]; then
        echo "-------------"
        echo "CLI: $SKILLCTL $*"
        echo "OUT: $output"
        echo "-------------"
    fi
}

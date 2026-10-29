#!/usr/bin/env bats

load helpers

@test "concurrent pulls retain both entries with shared and separate stores" {
    start_example_registry
    local mode
    for mode in shared separate; do
        local project="$TEST_WORKDIR/$mode-project"
        local first_store="$TEST_WORKDIR/$mode-first-store"
        local second_store="$first_store"
        if [ "$mode" == separate ]; then
            second_store="$TEST_WORKDIR/second-store"
        fi
        mkdir -p "$project" "$first_store" "$TEST_WORKDIR/$mode-private"
        export TMPDIR="$TEST_WORKDIR/$mode-private"
        cd "$project"
        run_skillctl init
        [[ "$output" == *"Initialized skillctl project"* ]]

        # Hold the store lock so the first pull has read the manifest but
        # cannot fetch or save it. Its private policy marks that point.
        printf '%s\n' "$$" > "$first_store/skillctl.lock"
        "$SKILLCTL" pull --store "$first_store" --trust none --name first \
            "$EXAMPLE_IMAGE:v1.0.0" > first.log 2>&1 &
        local first_pid=$!
        TEST_BACKGROUND_PIDS+=("$first_pid")
        local attempt count
        for attempt in {1..100}; do
            count=$(find "$TMPDIR" -name policy.json | wc -l)
            [ "$count" -ge 1 ] && break
            sleep 0.05
        done
        [ "$count" -ge 1 ]

        "$SKILLCTL" pull --store "$second_store" --trust none --name second \
            "$EXAMPLE_IMAGE:v1.0.1" > second.log 2>&1 &
        local second_pid=$!
        TEST_BACKGROUND_PIDS+=("$second_pid")
        local ready=false
        for attempt in {1..100}; do
            # With project locking, the first writer holds the project
            # lock while the second waits. Without it, both load stale
            # manifests (or the second finishes when stores differ).
            if [ -f skillctl.yaml.lock ] && [[ "$(cat skillctl.yaml.lock)" == "$first_pid" ]]; then
                ready=true
                break
            fi
            count=$(find "$TMPDIR" -name policy.json | wc -l)
            if [ "$count" -ge 2 ] || grep -q 'Pulled second' second.log; then
                ready=true
                break
            fi
            sleep 0.05
        done
        [ "$ready" == true ]
        rm "$first_store/skillctl.lock"
        if ! wait "$first_pid" || ! wait "$second_pid"; then
            cat first.log second.log
            return 1
        fi
        TEST_BACKGROUND_PIDS=()
        grep -q 'Pulled first' first.log
        grep -q 'Pulled second' second.log
        [[ "$(cat .agents/skills/first/counter.txt)" == "1" ]]
        [[ "$(cat .agents/skills/second/counter.txt)" == "2" ]]
        [[ "$(grep -c 'name:' skillctl.yaml)" == "2" ]]
        [ ! -e skillctl.yaml.lock ]
        run_skillctl list
        [[ "$output" == *"first"*"$EXAMPLE_IMAGE:v1.0.0"* ]]
        [[ "$output" == *"second"*"$EXAMPLE_IMAGE:v1.0.1"* ]]
        cd "$TEST_WORKDIR"
    done
}

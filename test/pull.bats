#!/usr/bin/env bats

load helpers

@test "pull without init fails" {
    run "$SKILLCTL" pull quay.io/fake/skill:latest
    [ "$status" -ne 0 ]
}

@test "pull requires an argument" {
    run "$SKILLCTL" pull
    [ "$status" -ne 0 ]
}

@test "pull rejects invalid image reference" {
    run_skillctl init
    run "$SKILLCTL" pull ":::invalid:::"
    [ "$status" -ne 0 ]
}

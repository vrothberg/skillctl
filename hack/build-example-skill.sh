#!/bin/bash
#
# Build and push the example skill as an OCI artifact.
#
# Usage:
#   ./hack/build-example-skill.sh [TAG]
#
# Defaults to tag "v1.0.0" if not specified. Increment the counter
# in hack/example-skill/counter.txt and bump the tag to simulate
# a skill update that Dependabot/Renovate can detect.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="${SCRIPT_DIR}/example-skill"
REGISTRY="${REGISTRY:-quay.io/vrothberg}"
IMAGE="${REGISTRY}/skillctl-example-skill"
TAG="${1:-v1.0.0}"
REF="${IMAGE}:${TAG}"

echo "Building example skill: ${REF}"

# Remove existing artifact if present, then add fresh.
podman artifact rm "${REF}" 2>/dev/null || true
podman artifact add "${REF}" \
    "${SKILL_DIR}/skill.md" \
    "${SKILL_DIR}/counter.txt"

echo "Pushing ${REF} ..."
podman artifact push "${REF}"

echo "Done: ${REF}"

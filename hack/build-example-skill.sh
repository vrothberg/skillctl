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
BUILD_DIR=$(mktemp -d /tmp/skillctl-example.XXXXXX)
trap 'rm -rf "$BUILD_DIR"' EXIT

echo "Building example skill: ${REF}"

tar -czf "${BUILD_DIR}/skill.tar.gz" -C "$SKILL_DIR" .

# Store the compressed directory tree verbatim as one OCI layer.
podman artifact rm "${REF}" 2>/dev/null || true
podman artifact add --file-type application/vnd.oci.image.layer.v1.tar+gzip \
    "${REF}" "${BUILD_DIR}/skill.tar.gz"

echo "Pushing ${REF} ..."
podman artifact push "${REF}"

echo "Done: ${REF}"

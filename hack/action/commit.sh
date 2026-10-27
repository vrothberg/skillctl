#!/usr/bin/env bash
set -euo pipefail

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"
git add .agents/skills/ skillctl.yaml
if git diff --cached --quiet; then
    echo "No skill changes to commit"
else
    git commit -m "$SKILLCTL_COMMIT_MESSAGE"
    git push
fi

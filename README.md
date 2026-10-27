<p align="center">
  <img src="logo.svg" alt="skillctl logo" width="400">
</p>
Distribute shared AI agent skills through OCI registries and update them through reviewed pull requests.

skillctl helps teams reuse skills across projects with explicit signature verification and version tracking.
It installs skills into `.agents/skills/` and records their image references, resolved digests, and trust declarations in `skillctl.yaml`.

It fits teams that already use container registries and want to reuse their publishing and review infrastructure for agent skills.
Renovate can propose version bumps, and CI can verify and extract the updated skills so reviewers see the content changes alongside the manifest changes.

skillctl is built on top of podman's [libartifact](https://pkg.go.dev/go.podman.io/common/pkg/libartifact) package.

## Installation

Requires Go >= 1.26.

```console
$ go install -tags "exclude_graphdriver_btrfs containers_image_openpgp" \
    github.com/vrothberg/skillctl/cmd/skillctl@latest
```

Or build from source:

```console
$ git clone https://github.com/vrothberg/skillctl.git
$ cd skillctl
$ make build
$ ./bin/skillctl version
```

## Quick start

```console
$ skillctl init
Initialized skillctl project: skillctl.yaml and .agents/skills/

$ skillctl pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
Pulling quay.io/vrothberg/skillctl-example-skill:v1.0.0 ...
Pulled skillctl-example-skill (sha256:c696d5bb34aa)
Extracted to .agents/skills/skillctl-example-skill

$ skillctl list
NAME                    IMAGE                                            DIGEST               PATH                                  TRUST
skillctl-example-skill  quay.io/vrothberg/skillctl-example-skill:v1.0.0  sha256:c696d5bb34aa  .agents/skills/skillctl-example-skill  none

$ skillctl remove skillctl-example-skill
Removed skillctl-example-skill
```

Every skill must declare how it is trusted: `--trust key=KEY.pub` pins a public key and requires a valid sigstore signature, `--trust none` explicitly opts out of verification. The declaration is recorded in the manifest and reused for later pulls and updates.

Skills are extracted to `.agents/skills/`, the cross-client discovery convention of the [Agent Skills](https://agentskills.io) ecosystem.
Agents that scan `.agents/skills/`, such as OpenAI Codex and opencode, pick up skillctl-managed skills automatically.

## Commands

| Command | Description |
|---------|-------------|
| `skillctl init` | Create `skillctl.yaml` and `.agents/skills/` in the current directory |
| `skillctl pull --trust ... IMAGE` | Pull a skill from an OCI registry, extract it, and update the manifest |
| `skillctl update` | Pull and extract all skills listed in the manifest |
| `skillctl remove NAME` | Remove a skill from the manifest and delete its extracted files |
| `skillctl list` | List skills tracked in the manifest |
| `skillctl version` | Print the version |

## Manifest format

`skillctl.yaml` tracks pinned skills with their OCI image references, digests, and trust declarations:

```yaml
apiVersion: v1
skills:
    - name: translator
      image: quay.io/redhat/skills/translator:1.0
      digest: sha256:c696d5bb34aa30f6fc23e1f0a0aafd91c642d7d0341e42c2ac009cddc743619f
      extractPath: .agents/skills/translator
      trust:
          key: keys/translator.pub

    - name: skillctl-example-skill
      image: quay.io/vrothberg/skillctl-example-skill:v1.0.0
      digest: sha256:a02ed76caf2de9d1ea33328d9ff0810a87ed37f6721baf9276d4d4023e346447
      extractPath: .agents/skills/skillctl-example-skill
      trust: none
```

This file serves as a lockfile. Renovate and Dependabot can detect OCI image references, open PRs to bump versions, and CI can run `skillctl update` to pull and extract all skills.

The `trust` entry declares how a skill is verified, and it travels with the version bump: when Renovate updates the image reference and digest, the new version is verified against the same trust as the version it replaces.

See [skillctl-consumer-example](https://github.com/vrothberg/skillctl-consumer-example) for a complete end-to-end example with Renovate integration and CI-driven skill updates.

## GitHub Action

This repository doubles as a GitHub Action, so CI can update skills without installing anything:

```yaml
permissions:
  contents: write

steps:
  - uses: actions/checkout@v7

  - uses: vrothberg/skillctl@main
```

The action runs `skillctl update --force` and commits changed skill files and the manifest.
Configure it via inputs:

| Input | Default | Description |
|-------|---------|-------------|
| `args` | `update --force` | Arguments passed to skillctl |
| `commit-and-push` | `true` | Commit and push updated skill files and the manifest |
| `commit-message` | `Update extracted skill files` | Commit message used when skill files changed |

Until releases are cut, reference the action by `@main`; release tags will serve as action refs as well.

### Use with Renovate

Renovate can bump skill versions in `skillctl.yaml` the same way it bumps container image tags, and the action turns those bumps into fully updated PRs.

Point Renovate's regex manager at the manifest, in `renovate.json`:

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:recommended"],
  "customManagers": [
    {
      "customType": "regex",
      "fileMatch": ["skillctl\\.yaml$"],
      "matchStrings": [
        "image:\\s*(?<depName>[^\\s:@]+):(?<currentValue>[^\\s@]+)(@(?<currentDigest>sha256:[a-f0-9]+))?"
      ],
      "datasourceTemplate": "docker"
    }
  ]
}
```

Then run the action only on Renovate's branches, so the updated skill files are committed to the PR:

```yaml
name: update-skills

on:
  push:
    branches:
      - renovate/**
    paths:
      - skillctl.yaml

permissions:
  contents: write

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7

      - uses: vrothberg/skillctl@main
```

The resulting flow: Renovate opens a PR bumping the tag in `skillctl.yaml`, the push triggers the action, which pulls the new version and commits the extracted skill files and the pinned digest to the PR branch. The merged PR therefore contains both the lockfile and the content changes, always in sync.

See [skillctl-consumer-example](https://github.com/vrothberg/skillctl-consumer-example) for a live setup of this flow.

## Trust and signature verification

skillctl is fail-closed about trust: every skill must declare how it is verified, and a skill without a declaration is refused.

| Declaration | Meaning |
|-------------|---------|
| `trust: {key: keys/foo.pub}` | Require a valid sigstore signature by that public key |
| `trust: none` | Explicitly opt out of verification |
| no `trust` entry | `pull` and `update` refuse the skill |

Trust is declared when a skill is added and then lives in the manifest:

```console
$ skillctl pull --trust key=keys/translator.pub quay.io/redhat/skills/translator:1.0
```

Pulling a new version of a tracked skill without `--trust` reuses the declaration recorded in the manifest, and `skillctl update` verifies every skill against its pinned key. Verification happens transparently at pull time via the `containers/image` library, the same mechanism used by `podman pull`.

skillctl derives the signature policy and `registries.d` configuration from the manifest on every run and keeps them in its internal store (`~/.local/share/skillctl/store/`). Ambient system configuration, such as `/etc/containers/policy.json`, is not consulted: trust lives in the repository, so every machine, laptop or CI runner, verifies exactly the same way. See the [containers-policy.json(5)](https://github.com/podman-container-tools/container-libs/blob/main/image/docs/containers-policy.json.5.md) man page for the policy format skillctl generates.

Verification keys are public keys: commit them next to `skillctl.yaml` so CI verifies the same way as local pulls.

Trust is scoped per repository: skills sharing an OCI repository must declare the same trust.

## Store

skillctl keeps its OCI artifact store and the generated signature policy and `registries.d` configuration in `~/.local/share/skillctl/store/`. Override the location with the `--store` flag or the `SKILLCTL_STORE` environment variable, which is what the test suite uses to inspect the generated configuration:

```console
$ skillctl --store /tmp/skillctl-store pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
$ cat /tmp/skillctl-store/policy.json
```

### Example: verifying Red Hat skills

Red Hat signs all images on `registry.access.redhat.com` with sigstore, so their skills can be verified the same way you would verify them with `cosign verify`:

```console
$ curl -sLO https://security.access.redhat.com/data/63405576.txt
$ skillctl pull --trust key=63405576.txt registry.access.redhat.com/hi/static:latest
Pulled static (sha256:f7a764cae676)
Extracted to .agents/skills/static
```

## Building

```console
$ make build
```

## Testing

Run unit tests and BATS integration tests locally:

```console
$ make test
```

Run everything inside a container for a reproducible environment:

```console
$ make container-test
```

## License

Apache-2.0

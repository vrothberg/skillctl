# Usage guide

For the project's purpose, installation, and a short quick start, see the
[README](../README.md).

## Building from source

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

Declare trust on the first pull: `--trust key=KEY.pub` requires a signature, while `--trust none` opts out of verification. See [Trust and signature verification](#trust-and-signature-verification) for details.

Skills are extracted to `.agents/skills/`, the cross-client discovery convention of the [Agent Skills](https://agentskills.io) ecosystem.
Agents that scan `.agents/skills/`, such as OpenAI Codex and opencode, pick up skillctl-managed skills automatically.

## Commands

| Command | Description |
|---------|-------------|
| `skillctl init` | Create `skillctl.yaml` and `.agents/skills/` in the current directory |
| `skillctl pull --trust ... IMAGE` | Pull a skill from an OCI registry, extract it, and update the manifest |
| `skillctl publish IMAGE DIRECTORY --sign ...` | Package a directory and publish it to an OCI registry |
| `skillctl update` | Pull and extract all skills listed in the manifest |
| `skillctl remove NAME` | Remove a skill from the manifest and delete its extracted files |
| `skillctl list` | List skills tracked in the manifest |
| `skillctl version` | Print the version |

## Extracted files and local changes

Skills are installed into `.agents/skills/<name>/`. When `pull` or `update` extracts a skill, it replaces the existing skill directory, removing local edits and files absent from the new version.
Make lasting changes in the skill's source and publish a new version; commit the extracted files in consuming projects for review.

Both commands pull the image and compare its resolved digest with the recorded digest. If the digests match, they skip extraction, even if local files have been changed or deleted.
Use `skillctl pull --force IMAGE` or `skillctl update --force` to restore files from the artifact even when the digest is unchanged.

## Publishing skills

Both the tagged image reference and source directory are required:

```console
$ skillctl publish quay.io/team/my-skill:v1 ./my-skill \
    --sign key=publisher.key --sign passphrase-file=publisher.pass
Published quay.io/team/my-skill:v1
Digest: sha256:...
Reference: quay.io/team/my-skill:v1@sha256:...
```

Use `--sign none` to publish unsigned. Signing supports sigstore/cosign keys
and unencrypted PKCS#8 private keys. Supply `--sign passphrase-file=FILE` for
keys with a nonempty passphrase; publishing never prompts. Signing failures
cause publishing to fail. Registry uploads are not atomic, so a failed
publish can leave partially uploaded content.

The complete source directory is included, including hidden files and ignore
files. No ignore rules or skill validation are applied. Publishing accepts
regular files, directories, and links. File permissions and link targets are
preserved; ownership and timestamps are normalized. The compressed layer and
OCI manifest are reproducible for identical input trees.

Publishing replaces existing tags and leaves
`skillctl.yaml` and installed skills unchanged. It uses the usual container
registry credentials, such as those configured by `podman login`.
Pass the printed `Reference` directly to `skillctl pull --trust ...` to
install the exact published content.

## Artifact format

Each OCI layer must contain a tar archive, optionally compressed with gzip or
zstd. Archive paths are relative to the skill directory, with `SKILL.md` at
the root. Layers are extracted in manifest order; later entries replace
earlier files. Nested directories, executable permissions, and link targets
are preserved.

`skillctl publish` packages a source directory as one gzip-compressed tar
layer. Paths are relative to the directory itself, without a wrapping folder.

## Manifest format

`skillctl.yaml` tracks skills with their OCI image references, resolved digests, and trust declarations:

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

The `image` field determines what is pulled. The separate `digest` field records the content retrieved by the last successful pull or update and is used to detect changes; it does not constrain future pulls.
If a tag points to new content, `skillctl update` retrieves that content and updates the recorded digest, even when the image reference in the manifest has not changed.

To pin immutable content, use a digest-qualified image reference instead of a tag. For example, using the digest from the manifest above:

```yaml
image: quay.io/redhat/skills/translator@sha256:c696d5bb34aa30f6fc23e1f0a0aafd91c642d7d0341e42c2ac009cddc743619f
```

References containing both a tag and a digest (`repository:tag@sha256:...`) are also supported. The digest selects the content; the tag is ignored when pulling, but preserved in the manifest for readability and update tooling.

See [Trust and signature verification](#trust-and-signature-verification) for verification rules and [Use with Renovate](#use-with-renovate) for automated updates.

## GitHub Action

This repository doubles as a GitHub Action, so CI can update skills without a separate CLI installation step:

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
| `commit-and-push` | `true` | Commit and push updated skill files and the manifest |
| `commit-message` | `Update extracted skill files` | Commit message used when skill files changed |

Until releases are cut, reference the action by `@main`; release tags will serve as action refs as well.

The action installs the CLI with `go install ...@latest` on each run. Pinning the action to a tag or commit pins the workflow implementation, but does not pin the installed CLI version.

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

See [skillctl-consumer-example](https://github.com/vrothberg/skillctl-consumer-example) for a live setup of this flow.

## Trust and signature verification

skillctl is fail-closed about trust: every skill must declare how it is verified, and a skill without a declaration is refused.

| Declaration | Meaning |
|-------------|---------|
| `trust: {key: keys/foo.pub}` | Require a valid sigstore signature by that public key |
| `trust: none` | Explicitly opt out of verification |
| no `trust` entry | `pull` and `update` refuse the skill |

Signature verification establishes that the artifact was signed by the configured key and has not been modified since signing.
It does not evaluate the safety or correctness of the skill's instructions. Review skill content and changes before allowing an agent to use them.

Trust is declared when a skill is added and then lives in the manifest:

```console
$ skillctl pull --trust key=keys/translator.pub quay.io/redhat/skills/translator:1.0
```

Pulling a new version of a tracked skill without `--trust` reuses the declaration recorded in the manifest, and `skillctl update` verifies skills with a pinned key against that key. Skills declaring `trust: none` are pulled without signature verification. Verification happens transparently at pull time via the `containers/image` library, the same mechanism used by `podman pull`.

Verification is derived from the manifest; machine-wide configuration such as `/etc/containers/policy.json` is not consulted. Commit the public verification keys next to `skillctl.yaml` so local pulls and CI use the same trust declarations.

Trust is scoped per repository: skills sharing an OCI repository must declare the same trust.

## Store

skillctl keeps its OCI artifact store in `~/.local/share/skillctl/store/`. Override the location with the `--store` flag or the `SKILLCTL_STORE` environment variable:

```console
$ skillctl --store /tmp/skillctl-store pull --trust none quay.io/vrothberg/skillctl-example-skill:v1.0.0
```

### Example: verifying Red Hat skills

Red Hat signs all images on `registry.access.redhat.com` with sigstore, so their skills can be verified the same way you would verify them with `cosign verify`:

```console
$ curl -sLO https://security.access.redhat.com/data/63405576.txt
$ skillctl pull --trust key=63405576.txt registry.access.redhat.com/hi/static:latest
Pulled static (sha256:f7a764cae676)
Extracted to .agents/skills/static
```

## Testing

Run unit tests, BATS integration tests, and lint in the development container:

```console
$ make container-test
$ make container-lint
```

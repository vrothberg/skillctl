<p align="center">
  <img src="logo.svg" alt="skillctl logo" width="400">
</p>
Manage AI agent skills as OCI artifacts with the same supply-chain guarantees as container images.

skillctl is a thin CLI built on top of podman's [libartifact](https://pkg.go.dev/go.podman.io/common/pkg/libartifact) package.
It pulls skills from OCI registries, extracts them locally, and tracks pinned versions in a manifest file that tools like Renovate or Dependabot can monitor for version bumps.

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

$ skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
Pulling quay.io/vrothberg/skillctl-example-skill:v1.0.0 ...
Pulled skillctl-example-skill (sha256:c696d5bb34aa)
Extracted to .agents/skills/skillctl-example-skill

$ skillctl list
NAME                    IMAGE                                            DIGEST               PATH
skillctl-example-skill  quay.io/vrothberg/skillctl-example-skill:v1.0.0  sha256:c696d5bb34aa  .agents/skills/skillctl-example-skill

$ skillctl remove skillctl-example-skill
Removed skillctl-example-skill
```

Skills are extracted to `.agents/skills/`, the cross-client discovery convention of the [Agent Skills](https://agentskills.io) ecosystem.
Agents that scan `.agents/skills/`, such as OpenAI Codex and opencode, pick up skillctl-managed skills automatically.

## Commands

| Command | Description |
|---------|-------------|
| `skillctl init` | Create `skillctl.yaml` and `.agents/skills/` in the current directory |
| `skillctl pull IMAGE` | Pull a skill from an OCI registry, extract it, and update the manifest |
| `skillctl update` | Pull and extract all skills listed in the manifest |
| `skillctl remove NAME` | Remove a skill from the manifest and delete its extracted files |
| `skillctl list` | List skills tracked in the manifest |
| `skillctl version` | Print the version |

## Manifest format

`skillctl.yaml` tracks pinned skills with their OCI image references and digests:

```yaml
apiVersion: v1
skills:
    - name: skillctl-example-skill
      image: quay.io/vrothberg/skillctl-example-skill:v1.0.0
      digest: sha256:c696d5bb34aa30f6fc23e1f0a0aafd91c642d7d0341e42c2ac009cddc743619f
      extractPath: .agents/skills/skillctl-example-skill
```

This file serves as a lockfile. Renovate and Dependabot can detect OCI image references, open PRs to bump versions, and CI can run `skillctl update` to pull and extract all skills.

See [skillctl-consumer-example](https://github.com/vrothberg/skillctl-consumer-example) for a complete end-to-end example with Renovate integration and CI-driven skill updates.

## Signature verification

Signature verification happens transparently at pull time via the `containers/image` library, the same mechanism used by `podman pull`.
See the [containers-policy.json(5)](https://github.com/podman-container-tools/container-libs/blob/main/image/docs/containers-policy.json.5.md) man page for the full specification.

On systems without container tooling installed, skillctl creates a permissive default policy.
To enforce signature verification, configure a `policy.json` and a `registries.d` directory.

### System-wide configuration

The standard paths are `/etc/containers/policy.json` (or `~/.config/containers/policy.json`) and `/etc/containers/registries.d/`.
When these are configured, skillctl and podman both use them automatically, no flags needed.

For per-invocation overrides, pass `--signature-policy` and `--registries-dir` to `pull` or `update`.

### Example: verifying Red Hat image signatures

Red Hat signs all images on `registry.access.redhat.com` with sigstore.
You can verify these signatures with skillctl the same way you would with `cosign verify`.

Download the Red Hat signing key:

```console
$ curl -sLO https://security.access.redhat.com/data/63405576.txt
```

On RHEL and Fedora, the signing key is distributed with the operating system, so this step can be skipped.

Create a policy requiring sigstore verification for `registry.access.redhat.com`:

```console
$ cat > policy.json <<EOF
{
    "default": [{"type": "insecureAcceptAnything"}],
    "transports": {
        "docker": {
            "registry.access.redhat.com": [{
                "type": "sigstoreSigned",
                "keyPath": "63405576.txt"
            }]
        }
    }
}
EOF
```

Enable sigstore attachment lookup for the registry.
This can go into `/etc/containers/registries.d/` for system-wide use or into a custom directory passed via `--registries-dir`:

```console
$ mkdir -p registries.d
$ cat > registries.d/redhat.yaml <<EOF
docker:
    registry.access.redhat.com:
        use-sigstore-attachments: true
EOF
```

Pull with signature verification:

```console
$ skillctl pull \
    --signature-policy policy.json \
    --registries-dir registries.d \
    registry.access.redhat.com/hi/static:latest
Pulled static (sha256:f7a764cae676)
Extracted to .agents/skills/static
```

When `policy.json` and `registries.d/` are installed at the standard system paths, the flags can be omitted entirely.

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

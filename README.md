# skillctl

Manage AI agent skills as OCI artifacts with the same supply-chain guarantees as container images.

skillctl is a thin CLI built on top of podman's [libartifact](https://pkg.go.dev/go.podman.io/common/pkg/libartifact) package.
It pulls skills from OCI registries, extracts them locally, and tracks pinned versions in a manifest file that tools like Renovate or Dependabot can monitor for version bumps.

## Quick start

```console
$ skillctl init
Initialized skillctl project: skillctl.yaml and skills/

$ skillctl pull quay.io/vrothberg/skillctl-example-skill:v1.0.0
Pulling quay.io/vrothberg/skillctl-example-skill:v1.0.0 ...
Pulled skillctl-example-skill (c696d5bb34aa)
Extracted to skills/skillctl-example-skill

$ skillctl list
NAME                    IMAGE                                            DIGEST               PATH
skillctl-example-skill  quay.io/vrothberg/skillctl-example-skill:v1.0.0  sha256:c696d5bb34aa  skills/skillctl-example-skill
```

## Commands

| Command | Description |
|---------|-------------|
| `skillctl init` | Create `skillctl.yaml` and `skills/` in the current directory |
| `skillctl pull IMAGE` | Pull a skill from an OCI registry, extract it, and update the manifest |
| `skillctl list` | List skills tracked in the manifest |

## Manifest format

`skillctl.yaml` tracks pinned skills with their OCI image references and digests:

```yaml
apiVersion: v1
skills:
    - name: skillctl-example-skill
      image: quay.io/vrothberg/skillctl-example-skill:v1.0.0
      digest: sha256:c696d5bb34aa30f6fc23e1f0a0aafd91c642d7d0341e42c2ac009cddc743619f
      extractPath: skills/skillctl-example-skill
```

This file serves as a lockfile. Renovate and Dependabot can detect OCI image references, open PRs to bump versions, and CI can run `skillctl pull` to update extracted files.

## Signature verification

Signature verification happens transparently at pull time via the `containers/image` library, the same mechanism used by `podman pull`.
Configure verification through your system's `/etc/containers/policy.json` or `~/.config/containers/policy.json`.

On systems without container tooling installed, skillctl creates a permissive default policy. To enforce signature verification, create a policy file at one of the standard paths.

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

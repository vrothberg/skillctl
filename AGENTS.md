# AGENTS.md

Instructions for AI agents working in this repository.

## What is skillctl

skillctl is a CLI that manages AI agent skills as OCI artifacts with the same
supply-chain guarantees as container images. It pulls skills from OCI registries,
extracts them locally, and tracks pinned versions in a manifest file
(`skillctl.yaml`). Renovate and Dependabot can monitor this manifest for version
bumps, enabling automated skill updates in CI.

Built on top of podman's [libartifact](https://pkg.go.dev/go.podman.io/common/pkg/libartifact)
package.

## Project layout

```
action.yml                    Composite GitHub Action wrapping skillctl for CI
cmd/skillctl/main.go          Entry point
internal/cli/                  Cobra commands (init, pull, list, update)
internal/manifest/             Manifest read/write/update (skillctl.yaml), trust declarations
internal/store/                Wraps libartifact.ArtifactStore (pull, extract, trust config synthesis)
test/                          BATS integration tests
hack/                          Helper scripts and example skill
  example-skill/               Example skill pushed to quay.io for testing
  build-example-skill.sh       Builds and pushes the example skill
  tree_status.sh               Fails if git tree is dirty
vendor/                        Vendored Go dependencies
```

## Building and testing

All dependencies are vendored. All builds use `-mod=vendor`.

Tests and lint run inside a container to ensure a reproducible environment
across local dev and CI. The container image is defined in `Containerfile` and
includes Go, BATS, golangci-lint (via multi-stage build from the official
image), and the C libraries required by `containers/image`.

```bash
make container-test           # Run unit + integration tests inside the container
make container-lint           # Run golangci-lint inside the container
make container-shell          # Interactive shell in the dev container
```

The `container-%` pattern rule in the Makefile wraps any make target to run
inside the container. For example, `make container-build` runs `make build`
inside the container.

**Before committing any Go changes, always run `make container-lint` and fix
all errors.** CI runs lint as a separate job and will reject code with lint
failures. Common issues include unchecked error return values (`errcheck`),
which the linter enforces strictly, even for cleanup calls like `os.Remove` in
error paths. Use blank-identifier assignments (e.g., `_ = os.Remove(...)`) to
explicitly discard errors that are intentionally ignored.

To run a single test file without rebuilding everything:

```bash
podman run --rm -v $(pwd):/src/skillctl skillctldev bash -c "make build && bats test/pull.bats"
```

### CI

CI runs three parallel jobs via a GitHub Actions matrix: `validate-vendor`,
`lint`, and `test`. Each job builds the container image and runs the
corresponding make target. See `.github/workflows/ci.yml`.

## Dependency management

- All Go dependencies are vendored in `vendor/`.
- Run `make vendor` to tidy and re-vendor after changing dependencies.
- `make validate-vendor` runs `make vendor` then checks the tree is clean.
- Only use tagged releases for dependencies, not pseudo-versions from main branches.
- Dependabot tracks Go modules, container image versions in `Containerfile`,
  and GitHub Actions versions. See `.github/dependabot.yml`.
- golangci-lint is not a Go dependency. Its binary is copied from the official
  container image via a multi-stage build in the Containerfile. Dependabot
  tracks the image version there.

## Key design decisions

### Trust and signature verification

Trust lives in the manifest, not on the machine: every skill declares how it
is verified (`trust: {key: ...}` for a pinned public key, `trust: none` for
an explicit opt-out), and skills without a declaration are refused by both
`pull` and `update`. At pull time skillctl synthesizes the `containers/image`
signature policy and the `registries.d` configuration from the manifest's
trust declarations and keeps them in `~/.local/share/skillctl/store/`.
Ambient system configuration (`/etc/containers/policy.json` and friends) is
deliberately not consulted, so verification behaves identically on every
machine. The store location can be overridden with the `--store` flag or the
`SKILLCTL_STORE` environment variable; the test suite does so to inspect the
generated configuration. See `internal/manifest/manifest.go` (the Trust type)
and `internal/store/trust.go` (the config synthesis).

### Local artifact store

The OCI Image Layout store lives at `~/.local/share/skillctl/store/`. This is
an implementation detail; the user-facing artifacts are `skillctl.yaml` and the
extracted files in `./.agents/skills/`.

### Renovate integration

The companion repo [skillctl-consumer-example](https://github.com/vrothberg/skillctl-consumer-example)
demonstrates the full Renovate integration: Renovate detects OCI image
references in `skillctl.yaml` via a custom regex manager, opens PRs to bump
versions, and CI runs `skillctl update` to pull and extract updated skills.

## Testing conventions

- Always check command output and error messages, not just exit statuses.
  Exit codes confirm pass/fail but do not catch regressions in user-facing
  messages or error paths. Match the expected output string or substring in
  every test assertion.

## Git conventions

- All changes go through pull requests. Never push directly to main.
- Never work on the main branch, even locally. Always create a local
  branch for every change before touching any file.
- Never create a pull request on your own initiative. Always wait for
  explicit instructions first.
- Commits must be signed off (`git commit -s`).
- Attribute AI contributions to the agent and model actually used. For Codex,
  add `Co-Authored-By: Codex <model> <noreply@openai.com>` to commit messages,
  substituting the actual model name. Never attribute work to another agent
  or provider.
- Use descriptive commit messages in imperative style.
- Keep commit subjects and body lines within 72 characters. Separate the
  subject from the body with a blank line. Keep trailers and URLs intact
  when they cannot fit within that limit.
- Write temporary files to `/tmp/`, never to the repo directory.

## Example skill

`hack/example-skill/` contains a toy skill pushed to
`quay.io/vrothberg/skillctl-example-skill`. It has two files: `skill.md`
(prompt) and `counter.txt` (an integer). Two versions exist: v1.0.0
(counter=1) and v1.0.1 (counter=2). The integration tests pull these real
artifacts from quay.io. The artifacts are unsigned, so tests declare
`--trust none` for them.

To push a new version:

```bash
./hack/build-example-skill.sh v1.0.2
```

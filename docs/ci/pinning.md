# Pinning actions and images

[English](./pinning.md) | [日本語](./pinning.ja.md)

Back to the [documentation index](../README.md).

Workflow actions and container images are pinned not by tag but by a value that identifies their content (the commit SHA, or the image digest). Updates come in as Dependabot PRs, which are read before they are merged.

## Why pin

- A tag can be moved to different content if the owner of the action or image is compromised. In March 2025, the moved tags of `tj-actions/changed-files` leaked secrets into job logs. In March 2026, the tags of `aquasecurity/setup-trivy`, which this repository also uses, were moved as well
- The `ci-infra` plan, `cd-backend` and `cd-notifier` assume AWS roles through OIDC. A compromised action in those jobs could steal temporary AWS credentials
- A commit SHA or a digest cannot be moved later. GitHub's "Secure use reference" also recommends pinning to a full-length commit SHA when a lockfile is not used
- SHA pinning is used instead of a lockfile (`.github/workflows/actions.lock`) because Dependabot's updating of the SHA together with the version comment is well established, and no extra tool (`gh actions-lock`) is needed

### References (Why pin)

- DOC: <https://docs.github.com/en/actions/reference/security/secure-use>
- DOC: <https://github.com/advisories/GHSA-mrrh-fwg8-r2c3>
- DOC: <https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23>

## How to write them

| Target                                | Form                                              | Example                                                              |
| ------------------------------------- | ------------------------------------------------- | -------------------------------------------------------------------- |
| `uses:` in `.github/workflows/*.yaml` | `owner/repo@<40-character SHA> # v<full version>` | `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1` |
| `FROM` in `backend/Dockerfile`        | `<image>:<tag>@sha256:<digest>`                   | `golang:1.27.4@sha256:5f1b98fa...`                                   |
| `FROM` in `loadtest/Dockerfile` (k6)  | Same as above                                     | `grafana/k6:2.4.0@sha256:d41bb79c...`                                |

- The comment on `uses:` is a full version such as `# v7.0.1`, not `# v7`. Dependabot rewrites a comment that ends with the most specific tag on the old SHA to the one for the new version
- The SHA is the commit the tag points to. For an annotated tag, use the commit on the line ending in `^{}`, not the tag object
- The image digest is that of the multi-architecture index (manifest list). `backend/Dockerfile` uses `golang` for the architecture of the build machine (`--platform=$BUILDPLATFORM`) and distroless for `linux/arm64`, so a single-architecture digest would not resolve on other architectures
- The tag is kept so the version stays readable and so Dependabot can compare versions. When both a tag and a digest are given, Docker pulls by the digest

### The k6 image

- The k6 image is on the `FROM` line of `loadtest/Dockerfile`, and `k6_image` in `loadtest/justfile` reads that line. `loadtest/Dockerfile` is never built
- Dependabot does not read justfiles, so the image is kept in a Dockerfile that Dependabot can read. Updating it by hand would make a missed update go unnoticed
- If there is no `FROM` line, reading `k6_image` fails and just stops before running any recipe

### Looking up values for a new entry

```sh
# Action: the commit the tag points to (the ^{} line for an annotated tag), and the full-version tag on the same commit
git ls-remote --tags https://github.com/actions/checkout | grep -E 'refs/tags/v7(\.|$|\^)'

# Image: the digest of the index (the Digest: line)
docker buildx imagetools inspect golang:1.27.4
```

- Use a SHA from the action's own repository. A commit in a fork can also be fetched under the original repository's name, so do not copy SHAs from elsewhere; look them up with `git ls-remote` against the original repository

### References (How to write them)

- DOC: <https://docs.github.com/en/code-security/dependabot/working-with-dependabot/keeping-your-actions-up-to-date-with-dependabot>
- DOC: <https://github.com/dependabot/dependabot-core/blob/main/github_actions/lib/dependabot/github_actions/file_updater/workflow_updater/version_commenter.rb>
- DOC: <https://github.com/dependabot/dependabot-core/blob/main/docker/lib/dependabot/docker/file_parser.rb>
- DOC: <https://docs.docker.com/reference/cli/docker/buildx/imagetools/inspect/>

## Updating them (Dependabot)

The configuration is in [`.github/dependabot.yml`](../../.github/dependabot.yml).

| Target                               | `package-ecosystem` | `directory` | Labels                              |
| ------------------------------------ | ------------------- | ----------- | ----------------------------------- |
| `uses:` in `.github/workflows/`      | `github-actions`    | `/`         | <ul><li>`ci`</li><li>`cd`</li></ul> |
| `FROM` in `backend/Dockerfile`       | `docker`            | `/backend`  | `backend`                           |
| `FROM` in `loadtest/Dockerfile` (k6) | `docker`            | `/loadtest` | `loadtest`                          |

- Checked once a week (Monday 09:00 JST)
- `cooldown` with `default-days: 7` skips versions published less than 7 days ago, so that a compromised release is not picked up before it is found and withdrawn. It does not apply to security updates
- Labels are taken from `.claude/rules/github/labels.md`. When labels are set, the default `dependencies` label is not added
- In the PR, read what changed in the new version (release notes, commit diff) before merging, and check that CI (`ci-actions`, `ci-backend`, `ci-loadtest`, ...) passes
- In a PR that bumps the `golang` image, also align `golang` in `.tool-versions` and the `go` version in `backend/go.mod` in the same PR. Local machines (asdf) read `.tool-versions` and `setup-go` in CI reads `backend/go.mod`, so bumping only the image makes the Go version inside and outside the image differ

### References (Updating them)

- DOC: <https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference>

## What is not pinned, and what is lost

- **Indirect dependencies are not pinned.** Actions called from inside an action, and binaries an action downloads at run time (Trivy itself for `setup-trivy`, golangci-lint for `golangci-lint-action`, ...), are not pinned by our SHA. Tool versions are pinned separately, such as in `.tool-versions`
- **`# syntax=docker/dockerfile:1` in `backend/Dockerfile` stays a tag.** It is the BuildKit frontend image, and Dependabot only reads `FROM` lines, so pinning it would leave it never updated
- **Dependabot security alerts are not raised for actions pinned to a SHA.** Version update PRs are still opened. Docker images have no Dependabot security updates in the first place
- **actionlint's check of `with:` inputs no longer applies.** actionlint checks the inputs of popular actions (typos, missing required inputs) only for major-version tags such as `@v4`. Before pinning, 4 of the 12 actions in this repository were checked (`docker/setup-buildx-action@v4`, `docker/build-push-action@v7`, `aws-actions/configure-aws-credentials@v6`, `golangci/golangci-lint-action@v9`), confirmed with actionlint 1.7.12 by adding an input that does not exist. After pinning, none are checked. When changing inputs, read the action's `action.yml`

### References (What is not pinned, and what is lost)

- DOC: <https://docs.github.com/en/actions/reference/security/secure-use>
- DOC: <https://github.com/rhysd/actionlint/blob/main/docs/checks.md#check-popular-action-inputs>
- DOC: <https://docs.github.com/en/code-security/dependabot/ecosystems-supported-by-dependabot/supported-ecosystems-and-repositories>

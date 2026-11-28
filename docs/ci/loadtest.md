# Checking the k6 scripts

[English](./loadtest.md) | [日本語](./loadtest.ja.md)

Back to the [documentation index](../README.md).

`ci-loadtest` checks the k6 scripts under `loadtest/`. For what the scripts do, see [Load testing during deployments (k6)](../loadtest/k6.md).

## What is checked

| Job                | Runner             | What it runs                                                                | Fails when                                                             |
| ------------------ | ------------------ | --------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `loadtest-inspect` | `ubuntu-latest`    | `just loadtest::inspect 5 5s`                                               | An import path or the syntax is wrong, or `options` cannot be built    |
| `loadtest-run`     | `ubuntu-24.04-arm` | `just backend::build old` and `backend::run old`, then `loadtest::run 5 5s` | Any request fails (99), or `setup()` cannot read the old version (108) |

- `loadtest-run` builds and starts the application image inside the runner, waits for `/health` to return 200, and runs k6 at 5 RPS for 5 seconds. Requests go only to the container inside the runner. AWS is not touched (no OIDC role is used)
- The image is built for `linux/arm64`, the same as ECS. An arm64 runner is used so that it runs without emulation (QEMU)
- On failure, the last step prints the application logs

## When it runs

It runs on PRs that change any of the following.

- `loadtest/**`
- The application and the image inputs: `backend/cmd/api/**`, `backend/Dockerfile`, `backend/.dockerignore`, `backend/go.mod`
- The `internal` packages the application imports: `backend/internal/server/**`, `backend/internal/health/**`, `backend/internal/checkout/**`. Other `internal` packages are used only by the notification Lambda, so they are left out
- The build and start recipes: `backend/justfile`, `justfile`
- `.github/workflows/ci-loadtest.yaml`

The application is included so that a change to it (such as the shape of the response body) that breaks k6 is caught in the PR.

## Keeping versions in one place

- The k6 version is set only by the `FROM` line of `loadtest/Dockerfile`, which `k6_image` in `loadtest/justfile` reads. The workflow only calls the just recipes and does not name a k6 version. The image is pinned by tag and digest and updated by Dependabot ([Pinning actions and images](./pinning.md))
- The just version is pinned in the `env` of `ci-loadtest.yaml` (`JUST_VERSION`, 1.46.0)

## Running locally

The same steps run with Docker on a local machine, using two terminals.

```sh
just loadtest::inspect 5 5s

# terminal 1
just backend::build old
just backend::run old

# terminal 2
just loadtest::run 5 5s
```

`loadtest::run` starts the k6 container with `--add-host=host.docker.internal:host-gateway`. On Linux (the CI runner) `host.docker.internal` does not resolve, so this lets k6 reach the local application.

## References

- [GitHub-hosted runners (GitHub Docs)](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
- [--add-host of docker container run (Docker Docs)](https://docs.docker.com/reference/cli/docker/container/run/#add-host)
- [Modules (k6)](https://grafana.com/docs/k6/latest/using-k6/modules/)

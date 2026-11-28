# Versions that fail on purpose

[English](./fail-mode.md) | [日本語](./fail-mode.ja.md)

Back to the [documentation index](../README.md).

The test application fails on purpose only when its version name (the tag name) ends in a fixed suffix. This is used to check whether the ECS deployment circuit breaker treats the deployment as failed and rolls back to the previous version.

## How it works

- `cd-backend` passes the tag name to the build as `VERSION`, and the application receives it through `-ldflags "-X main.version=..."`
- On startup, the application decides its behavior from the end of the version name (`backend/internal/failmode`)
- So a failing version needs only a tag name, with no change to Terraform or CD. Pushing the next normal tag brings the usual behavior back

| End of the version name | Example          | Behavior                                                                                                                    | Failure it imitates                                                                       |
| ----------------------- | ---------------- | --------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `-fail-exit`            | `v9-fail-exit`   | Writes one log line and exits with code 1 without starting the HTTP server                                                  | Crashing during startup (missing environment variable, failed config load, panic in init) |
| `-fail-health`          | `v9-fail-health` | Starts as usual, but `/health` keeps returning 500. `/` and checkout (`POST /v1/carts/{cart_id}/checkout`) respond as usual | The process runs but fails the health check (wrong settings for a dependency, ...)        |
| Anything else           | `v9`             | Runs as usual                                                                                                               | -                                                                                         |

- The suffix is case-sensitive. `v9-FAIL-EXIT` and `v9-fail-exit-2` run as usual
- The `mode` field in the logs (`normal`, `fail-exit`, `fail-health`) shows which behavior the application started with. These lines are from the local container (`time` is left out)

```json
{"level":"ERROR","msg":"exiting on purpose before starting the server","version":"v9-fail-exit","mode":"fail-exit"}
{"level":"INFO","msg":"server started","addr":":8080","version":"v9-fail-health","mode":"fail-health"}
```

`/health` on a `-fail-health` version returns `500 Internal Server Error` with this body.

```text
fail-health: this version fails the health check on purpose
```

## Checking locally

```sh
# -fail-exit: exits right away with code 1
just backend::build v9-fail-exit
just backend::run v9-fail-exit

# -fail-health: keep it running and curl from another terminal (Ctrl-C to stop)
just backend::build v9-fail-health
just backend::run v9-fail-health
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/health   # 500
curl -s localhost:8080/                                          # {"version":"v9-fail-health"}
```

## Using it in production

Pushing a tag for a failing version makes `cd-backend` deploy it as usual. Only admins can push tags ([Ruleset for v\* tags](../github/tag-ruleset.md)).

```sh
git tag v9-fail-exit
git push origin v9-fail-exit
```

The expected behavior is below. Whether it actually happens is what the verification checks.

- `-fail-exit`: The container exits right away, so tasks do not stay RUNNING and keep stopping and restarting. The circuit breaker counts them as failures and rolls back to the previous version
- `-fail-health`: Tasks reach RUNNING but fail the ALB health check (`/health` returning 200 means healthy). After the health check grace period (`health_check_grace_period`), ECS stops and replaces the tasks, and the circuit breaker counts them as failures

After the rollback, check that the previous version is running by the version that `https://ecs-lab.tamaco-blog.jp/` returns.

> [!NOTE]
> The number of the next normal version after a failing tag (the same number or the next one) is decided at verification time.

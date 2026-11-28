# Versions that fail on purpose

[English](./fail-mode.md) | [日本語](./fail-mode.ja.md)

Back to the [documentation index](../README.md).

The test application fails on purpose only when its version name (the tag name) ends in a fixed suffix. This is used to check whether the ECS deployment circuit breaker or the target 5xx alarm treats the deployment as failed and rolls back to the previous version.

## How it works

- `cd-backend` passes the tag name to the build as `VERSION`, and the application receives it through `-ldflags "-X main.version=..."`
- On startup, the application decides its behavior from the end of the version name (`backend/internal/failmode`)
- So a failing version needs only a tag name, with no change to Terraform or CD. Pushing the next normal tag brings the usual behavior back

| End of the version name | Example          | Behavior                                                                                                                    | Failure it imitates                                                                       |
| ----------------------- | ---------------- | --------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `-fail-exit`            | `v9-fail-exit`   | Writes one log line and exits with code 1 without starting the HTTP server                                                  | Crashing during startup (missing environment variable, failed config load, panic in init) |
| `-fail-health`          | `v9-fail-health` | Starts as usual, but `/health` keeps returning 500. `/` and checkout (`POST /v1/carts/{cart_id}/checkout`) respond as usual | The process runs but fails the health check (wrong settings for a dependency, ...)        |
| `-fail-500`             | `v10-fail-500`   | Starts as usual and `/health` returns 200, but `/` and checkout keep returning 500                                          | Startup and the health check pass, but real requests fail (a bug in request handling)     |
| Anything else           | `v9`             | Runs as usual                                                                                                               | -                                                                                         |

- The suffix is case-sensitive. `v9-FAIL-EXIT` and `v9-fail-exit-2` run as usual
- The `mode` field in the logs (`normal`, `fail-exit`, `fail-health`, `fail-500`) shows which behavior the application started with. These lines are from the local container (`time` is left out)

```json
{"level":"ERROR","msg":"exiting on purpose before starting the server","version":"v9-fail-exit","mode":"fail-exit"}
{"level":"INFO","msg":"server started","addr":":8080","version":"v9-fail-health","mode":"fail-health"}
{"level":"INFO","msg":"server started","addr":":8080","version":"v10-fail-500","mode":"fail-500"}
```

### 500 returned on purpose

`/health` on a `-fail-health` version, and `/` and checkout on a `-fail-500` version, return `500 Internal Server Error` with these bodies.

```text
fail-health: this version fails the health check on purpose
fail-500: this version returns 500 on purpose
```

Each time it returns 500, the application writes one `ERROR` line, separate from the access log (`msg` is `access`). The `msg` and `mode` fields show that the failure is on purpose for a verification, not a bug in the application. These lines are from the local container (`time` is left out).

```json
{"level":"ERROR","msg":"failing the health check on purpose","mode":"fail-health","method":"GET","path":"/health","version":"v9-fail-health"}
{"level":"INFO","msg":"access","method":"GET","path":"/health","status":500,"duration_ms":0,"version":"v9-fail-health"}
{"level":"ERROR","msg":"returning 500 on purpose","mode":"fail-500","method":"POST","path":"/v1/carts/cart_01/checkout","version":"v10-fail-500"}
{"level":"INFO","msg":"access","method":"POST","path":"/v1/carts/cart_01/checkout","status":500,"duration_ms":0,"version":"v10-fail-500"}
```

- A `-fail-health` version writes one line for every ALB health check (every 10 seconds). Failing versions run only during a verification, so the volume is not a concern

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

# -fail-500: keep it running and curl from another terminal (Ctrl-C to stop)
just backend::build v10-fail-500
just backend::run v10-fail-500
curl -s localhost:8080/health                                    # ok
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/         # 500
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/v1/carts/cart_01/checkout \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"sku":"TSHIRT-BLK-M","quantity":2}],"shipping_address_id":"addr_01"}'   # 500
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
- `-fail-500`: Tasks reach RUNNING and pass the health check, so the circuit breaker does not act. Requests that reach the new version get 500, and when there are `target_5xx_threshold` (5 by default) or more in a minute, the target 5xx alarm (`prd-amber-api-target-5xx`) fires and the deployment rolls back to the previous version. The alarm does not fire without requests, so keep sending them (with k6, for example) during the deployment

After the rollback, check that the previous version is running by the version that `https://ecs-lab.tamaco-blog.jp/` returns.

> [!NOTE]
> The number of the next normal version after a failing tag (the same number or the next one) is decided at verification time.

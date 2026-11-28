# わざと失敗する版

[English](./fail-mode.md) | [日本語](./fail-mode.ja.md)

[ドキュメント一覧](../README.ja.md) に戻る。

検証用のアプリは、版の名前 (タグ名) が決まった形で終わるときだけ、わざと失敗します。ECS のデプロイのサーキットブレーカーや、ターゲットの 5xx のアラームが、失敗と判定して元の版に戻すかを確かめるために使います。

## 仕組み

- `cd-backend` は、タグ名を `VERSION` としてビルドに渡し、アプリは `-ldflags "-X main.version=..."` で受け取ります
- アプリは起動するときに、版の名前の末尾から動きを決めます (`backend/internal/failmode`)
- そのため、Terraform も CD も変えずに、タグ名だけで失敗する版を作れます。次にふつうのタグを push すれば、元の動きに戻ります

| 版の名前の末尾 | 例               | 動き                                                                                                                       | 真似する失敗                                                                    |
| -------------- | ---------------- | -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `-fail-exit`   | `v9-fail-exit`   | HTTP サーバーを立てずに、ログを 1 行出して終了コード 1 で終わる                                                            | 起動の途中で落ちる (環境変数の不足、設定の読み込みの失敗、初期化の panic)       |
| `-fail-health` | `v9-fail-health` | ふつうに起動するが、`/health` は 500 を返し続ける。`/` と注文の確定 (`POST /v1/carts/{cart_id}/checkout`) は、ふつうに返す | プロセスは動くが、ヘルスチェックに通らない (依存先の設定の間違いなど)           |
| `-fail-500`    | `v10-fail-500`   | ふつうに起動し、`/health` は 200 を返すが、`/` と注文の確定は 500 を返し続ける                                             | 起動もヘルスチェックも通るが、実際のリクエストで失敗する (処理の中の不具合など) |
| それ以外       | `v9`             | ふつうに動く                                                                                                               | -                                                                               |

- 末尾は大文字・小文字を区別します。`v9-FAIL-EXIT` や `v9-fail-exit-2` は、ふつうに動きます
- どの動きで起動したかは、ログの `mode` (`normal`、`fail-exit`、`fail-health`、`fail-500`) で見分けます。次は手元のコンテナのログです (`time` は省いています)

```json
{"level":"ERROR","msg":"exiting on purpose before starting the server","version":"v9-fail-exit","mode":"fail-exit"}
{"level":"INFO","msg":"server started","addr":":8080","version":"v9-fail-health","mode":"fail-health"}
{"level":"INFO","msg":"server started","addr":":8080","version":"v10-fail-500","mode":"fail-500"}
```

### わざと返した 500

`-fail-health` の版の `/health` と、`-fail-500` の版の `/` と注文の確定は、`500 Internal Server Error` と次の本文を返します。

```text
fail-health: this version fails the health check on purpose
fail-500: this version returns 500 on purpose
```

500 を返すたびに、アクセスログ (`msg` が `access`) とは別に、`ERROR` のログを 1 行出します。`msg` と `mode` で、アプリの不具合ではなく、検証のためにわざと失敗させたものだと見分けます。次は手元のコンテナのログです (`time` は省いています)。

```json
{"level":"ERROR","msg":"failing the health check on purpose","mode":"fail-health","method":"GET","path":"/health","version":"v9-fail-health"}
{"level":"INFO","msg":"access","method":"GET","path":"/health","status":500,"duration_ms":0,"version":"v9-fail-health"}
{"level":"ERROR","msg":"returning 500 on purpose","mode":"fail-500","method":"POST","path":"/v1/carts/cart_01/checkout","version":"v10-fail-500"}
{"level":"INFO","msg":"access","method":"POST","path":"/v1/carts/cart_01/checkout","status":500,"duration_ms":0,"version":"v10-fail-500"}
```

- `-fail-health` の版は、ALB のヘルスチェック (10 秒ごと) のたびに 1 行出ます。失敗する版は検証の間だけ動かすため、量は問題にしません

## 手元で確かめる

```sh
# -fail-exit: すぐ終わり、終了コードは 1
just backend::build v9-fail-exit
just backend::run v9-fail-exit

# -fail-health: 動かしたまま、別の端末で curl する (止めるときは Ctrl-C)
just backend::build v9-fail-health
just backend::run v9-fail-health
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/health   # 500
curl -s localhost:8080/                                          # {"version":"v9-fail-health"}

# -fail-500: 動かしたまま、別の端末で curl する (止めるときは Ctrl-C)
just backend::build v10-fail-500
just backend::run v10-fail-500
curl -s localhost:8080/health                                    # ok
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/         # 500
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/v1/carts/cart_01/checkout \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"sku":"TSHIRT-BLK-M","quantity":2}],"shipping_address_id":"addr_01"}'   # 500
```

## 本番で使う

失敗する版のタグを push すると、`cd-backend` がいつもどおりデプロイします。タグを push できるのは管理者だけです ([v\* のタグのルールセット](../github/tag-ruleset.ja.md))。

```sh
git tag v9-fail-exit
git push origin v9-fail-exit
```

想定している動きは次のとおりです。実際にそうなるかは、検証で確かめます。

- `-fail-exit`: コンテナがすぐ終わるため、タスクは RUNNING のまま続かず、止まっては起動し直します。サーキットブレーカーが失敗と数え、元の版に戻します
- `-fail-health`: タスクは RUNNING になりますが、ALB のヘルスチェック (`/health` が 200 なら正常) に通りません。ヘルスチェックの猶予 (`health_check_grace_period`) が過ぎると、ECS がタスクを止めて起動し直し、サーキットブレーカーが失敗と数えます
- `-fail-500`: タスクは RUNNING になり、ヘルスチェックにも通るため、サーキットブレーカーは働きません。新しい版にリクエストが届くと 500 が返り、1 分間に `target_5xx_threshold` 件 (既定は 5 件) 以上になると、ターゲットの 5xx のアラーム (`prd-amber-api-target-5xx`) が発火し、元の版に戻します。リクエストが届かないとアラームは発火しないため、デプロイの間は k6 などで送り続けます

戻ったあとに、元の版で動いていることを `https://ecs-lab.tamaco-blog.jp/` の返すバージョンで確かめます

> [!NOTE]
> 失敗する版のタグのあとに、次のふつうの版の番号をどうするか (同じ番号か、次の番号か) は、検証のときに決めます。

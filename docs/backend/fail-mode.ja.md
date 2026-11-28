# わざと失敗する版

[English](./fail-mode.md) | [日本語](./fail-mode.ja.md)

[ドキュメント一覧](../README.ja.md) に戻る。

検証用のアプリは、版の名前 (タグ名) が決まった形で終わるときだけ、わざと失敗します。ECS のデプロイのサーキットブレーカーが、失敗と判定して元の版に戻すかを確かめるために使います。

## 仕組み

- `cd-backend` は、タグ名を `VERSION` としてビルドに渡し、アプリは `-ldflags "-X main.version=..."` で受け取ります
- アプリは起動するときに、版の名前の末尾から動きを決めます (`backend/internal/failmode`)
- そのため、Terraform も CD も変えずに、タグ名だけで失敗する版を作れます。次にふつうのタグを push すれば、元の動きに戻ります

| 版の名前の末尾 | 例               | 動き                                                                                                                       | 真似する失敗                                                              |
| -------------- | ---------------- | -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| `-fail-exit`   | `v9-fail-exit`   | HTTP サーバーを立てずに、ログを 1 行出して終了コード 1 で終わる                                                            | 起動の途中で落ちる (環境変数の不足、設定の読み込みの失敗、初期化の panic) |
| `-fail-health` | `v9-fail-health` | ふつうに起動するが、`/health` は 500 を返し続ける。`/` と注文の確定 (`POST /v1/carts/{cart_id}/checkout`) は、ふつうに返す | プロセスは動くが、ヘルスチェックに通らない (依存先の設定の間違いなど)     |
| それ以外       | `v9`             | ふつうに動く                                                                                                               | -                                                                         |

- 末尾は大文字・小文字を区別します。`v9-FAIL-EXIT` や `v9-fail-exit-2` は、ふつうに動きます
- どの動きで起動したかは、ログの `mode` (`normal`、`fail-exit`、`fail-health`) で見分けます。次は手元のコンテナのログです (`time` は省いています)

```json
{"level":"ERROR","msg":"exiting on purpose before starting the server","version":"v9-fail-exit","mode":"fail-exit"}
{"level":"INFO","msg":"server started","addr":":8080","version":"v9-fail-health","mode":"fail-health"}
```

`-fail-health` の版の `/health` は、`500 Internal Server Error` と次の本文を返します。

```text
fail-health: this version fails the health check on purpose
```

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

戻ったあとに、元の版で動いていることを `https://ecs-lab.tamaco-blog.jp/` の返すバージョンで確かめます

> [!NOTE]
> 失敗する版のタグのあとに、次のふつうの版の番号をどうするか (同じ番号か、次の番号か) は、検証のときに決めます。

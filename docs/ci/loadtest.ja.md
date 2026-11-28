# k6 のスクリプトの検査

[English](./loadtest.md) | [日本語](./loadtest.ja.md)

[ドキュメント一覧](../README.ja.md)に戻る。

`loadtest/` の k6 のスクリプトを、`ci-loadtest` で検査します。スクリプトの中身は [デプロイ中の負荷試験 (k6)](../loadtest/k6.ja.md) を見てください。

## 検査の内容

| ジョブ             | runner             | 流すもの                                                                    | 失敗する例                                                    |
| ------------------ | ------------------ | --------------------------------------------------------------------------- | ------------------------------------------------------------- |
| `loadtest-inspect` | `ubuntu-latest`    | `just loadtest::inspect 5 5s`                                               | import のパスの誤り、構文の誤り、`options` を組み立てられない |
| `loadtest-run`     | `ubuntu-24.04-arm` | `just backend::build old` と `backend::run old` のあと `loadtest::run 5 5s` | 失敗が 1 件でも出る (99)、`setup()` で古い版を読めない (108)  |

- `loadtest-run` は、アプリのイメージを runner の中でビルドして動かし、`/health` が 200 を返してから、k6 を 5 RPS で 5 秒流します。送り先は runner の中のコンテナだけで、AWS には触りません (OIDC のロールも使いません)
- イメージは ECS と同じ `linux/arm64` でビルドします。エミュレーション (QEMU) を使わずに動かすため、arm64 の runner を使います
- 失敗したときは、アプリのログを最後のステップで出します

## 動くとき

PR で、次を変えたときに動きます。

- `loadtest/**`
- アプリとイメージの材料: `backend/cmd/api/**`、`backend/Dockerfile`、`backend/.dockerignore`、`backend/go.mod`
- アプリが読み込む `internal` のパッケージ: `backend/internal/server/**`、`backend/internal/health/**`、`backend/internal/checkout/**`、`backend/internal/failmode/**`。ほかの `internal` のパッケージは通知の Lambda だけが使うため、含めません
- ビルドと起動のレシピ: `backend/justfile`、`justfile`
- `.github/workflows/ci-loadtest.yaml`

アプリを含めるのは、アプリの変更 (本文の形など) で k6 が通らなくなるのに、PR の時点で気づくためです。

## 版のそろえ方

- k6 の版は `loadtest/Dockerfile` の `FROM` の行 (`loadtest/justfile` の `k6_image` が読む) だけで決めます。ワークフローは just のレシピを呼ぶだけで、k6 の版を書きません。イメージはタグとダイジェストで固定し、Dependabot で上げます ([action とイメージの固定](./pinning.ja.md))
- just の版は `ci-loadtest.yaml` の `env` (`JUST_VERSION`) で固定しています (1.46.0)

## 手元での流し方

CI と同じ手順を、手元の Docker で流せます。端末を 2 つ使います。

```sh
just loadtest::inspect 5 5s

# 端末 1
just backend::build old
just backend::run old

# 端末 2
just loadtest::run 5 5s
```

`loadtest::run` は、`--add-host=host.docker.internal:host-gateway` を付けて k6 のコンテナを動かします。Linux (CI の runner) では `host.docker.internal` を引けないため、これで手元のアプリに送れるようにしています。

## 参考

- [GitHub-hosted runners (GitHub Docs)](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
- [docker container run の --add-host (Docker Docs)](https://docs.docker.com/reference/cli/docker/container/run/#add-host)
- [Modules (k6)](https://grafana.com/docs/k6/latest/using-k6/modules/)

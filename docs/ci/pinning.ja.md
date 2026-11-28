# action とイメージの固定

[English](./pinning.md) | [日本語](./pinning.ja.md)

[ドキュメント一覧](../README.ja.md) に戻る。

ワークフローの action と、コンテナのイメージは、タグではなく、中身を一意に決める値 (commit の SHA、イメージのダイジェスト) で固定します。上げるときは、Dependabot が出す PR を読んで取り込みます。

## なぜ固定するか

- タグは、action やイメージの持ち主の権限を乗っ取られると、別の中身に付け替えられます。2025-03 の `tj-actions/changed-files` では、付け替えられたタグから秘密の値がジョブのログに漏れました。2026-03 には、このリポジトリも使う `aquasecurity/setup-trivy` のタグも付け替えられました
- `ci-infra` の plan、`cd-backend`、`cd-notifier` は、OIDC で AWS のロールを引き受けます。そこで動く action が乗っ取られると、AWS の一時的な認証情報を盗まれえます
- commit の SHA とダイジェストは、あとから付け替えられません。GitHub の「Secure use reference」も、ロックファイルを使わない場合は、完全な commit の SHA で固定するよう勧めています
- ロックファイル (`.github/workflows/actions.lock`) ではなく SHA で固定するのは、Dependabot が SHA と版のコメントを一緒に書き換える仕組みに実績があり、ほかの道具 (`gh actions-lock`) が要らないためです

### 参考 (なぜ固定するか)

- DOC: <https://docs.github.com/en/actions/reference/security/secure-use>
- DOC: <https://github.com/advisories/GHSA-mrrh-fwg8-r2c3>
- DOC: <https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23>

## 書き方

| 対象                                  | 書き方                                             | 例                                                                   |
| ------------------------------------- | -------------------------------------------------- | -------------------------------------------------------------------- |
| `.github/workflows/*.yaml` の `uses:` | `owner/repo@<40 文字の SHA> # v<完全なバージョン>` | `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1` |
| `backend/Dockerfile` の `FROM`        | `<イメージ>:<タグ>@sha256:<ダイジェスト>`          | `golang:1.27.4@sha256:5f1b98fa...`                                   |
| `loadtest/Dockerfile` の `FROM` (k6)  | 同上                                               | `grafana/k6:2.4.0@sha256:d41bb79c...`                                |

- `uses:` のコメントは、`# v7` ではなく `# v7.0.1` のような完全なバージョンにします。Dependabot は、古い SHA に付いた一番細かいタグで終わるコメントを、新しい版のものに書き換えます
- SHA は、そのタグが指す commit にします。annotated tag は、タグのオブジェクトではなく、`^{}` の付いた行の commit を使います
- イメージのダイジェストは、マルチアーキテクチャのインデックス (manifest list) のものにします。`backend/Dockerfile` は `--platform=$BUILDPLATFORM` で実行する環境のアーキテクチャの `golang` を使い、`linux/arm64` の distroless を使うため、1 つのアーキテクチャのダイジェストにすると、ほかのアーキテクチャで引けなくなります
- タグも残すのは、どの版かを読めるようにするためと、Dependabot が版を比べて上げられるようにするためです。タグとダイジェストを両方書くと、Docker はダイジェストで引きます

### k6 のイメージ

- k6 のイメージは `loadtest/Dockerfile` の `FROM` の行に置き、`loadtest/justfile` の `k6_image` がその行を読みます。`loadtest/Dockerfile` はビルドしません
- Dependabot は justfile を読まないため、Dependabot が読める Dockerfile に置いています。手で上げる運用にすると、上げ忘れても気づけないためです
- `FROM` の行が無いと、`k6_image` の読み取りが失敗し、just はレシピを動かす前に止まります

### 新しく足すときの引き方

```sh
# action: タグが指す commit (annotated tag は ^{} の行) と、同じ commit を指す完全なバージョンのタグを見る
git ls-remote --tags https://github.com/actions/checkout | grep -E 'refs/tags/v7(\.|$|\^)'

# イメージ: インデックスのダイジェスト (Digest: の行)
docker buildx imagetools inspect golang:1.27.4
```

- SHA は、その action のリポジトリのものを使います。フォークの commit も元のリポジトリの名前で取り出せてしまうため、ほかの場所から写さず、元のリポジトリの `git ls-remote` で引きます

### 参考 (書き方)

- DOC: <https://docs.github.com/en/code-security/dependabot/working-with-dependabot/keeping-your-actions-up-to-date-with-dependabot>
- DOC: <https://github.com/dependabot/dependabot-core/blob/main/github_actions/lib/dependabot/github_actions/file_updater/workflow_updater/version_commenter.rb>
- DOC: <https://github.com/dependabot/dependabot-core/blob/main/docker/lib/dependabot/docker/file_parser.rb>
- DOC: <https://docs.docker.com/reference/cli/docker/buildx/imagetools/inspect/>

## 更新のしかた (Dependabot)

設定は [`.github/dependabot.yml`](../../.github/dependabot.yml) に置きます。

| 対象                                 | `package-ecosystem` | `directory` | 付けるラベル                        |
| ------------------------------------ | ------------------- | ----------- | ----------------------------------- |
| `.github/workflows/` の `uses:`      | `github-actions`    | `/`         | <ul><li>`ci`</li><li>`cd`</li></ul> |
| `backend/Dockerfile` の `FROM`       | `docker`            | `/backend`  | `backend`                           |
| `loadtest/Dockerfile` の `FROM` (k6) | `docker`            | `/loadtest` | `loadtest`                          |

- 週に 1 回 (月曜の 09:00 JST) 見ます
- `cooldown` の `default-days: 7` で、公開から 7 日たっていない版は取り込みません。乗っ取られた版が公開されても、見つかって取り下げられるまでの間に入れないためです。セキュリティの更新には効きません
- ラベルは `.claude/rules/github/labels.md` にあるものを付けます。ラベルを指定すると、既定の `dependencies` のラベルは付きません
- PR では、上がった版の変更点 (release のノート、commit の差分) を読んでから取り込みます。CI (`ci-actions`、`ci-backend`、`ci-loadtest` など) が通ることも確かめます
- `golang` のイメージを上げる PR では、`.tool-versions` の `golang` と、`backend/go.mod` の `go` の版も、同じ PR でそろえます。手元 (asdf) は `.tool-versions` を、CI の `setup-go` は `backend/go.mod` を読むため、イメージだけを上げると、イメージの中と外で Go の版がずれます

### 参考 (更新のしかた)

- DOC: <https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference>

## 固定できないもの・失うもの

- **間接的な依存は固定できません。** action の中で呼ぶ action や、action が実行時に取ってくるバイナリ (`setup-trivy` の Trivy 本体、`golangci-lint-action` の golangci-lint など) は、こちらの SHA では固定されません。ツールの版は `.tool-versions` などで別に固定します
- **`backend/Dockerfile` の `# syntax=docker/dockerfile:1` は、タグのままです。** BuildKit のフロントエンドのイメージで、Dependabot は `FROM` の行しか読まないため、固定すると上げられなくなります
- **SHA で固定した action には、Dependabot のセキュリティアラートが出ません。** 版を上げる PR は出ます。Docker のイメージには、もともと Dependabot のセキュリティの更新がありません
- **actionlint の `with:` の入力の検査が効かなくなります。** actionlint は、よく使われる action の入力 (打ち間違い、必須の入力の漏れ) を、`@v4` のようなメジャーのタグのときだけ検査します。SHA にする前は、このリポジトリの 12 個のうち 4 個 (`docker/setup-buildx-action@v4`、`docker/build-push-action@v7`、`aws-actions/configure-aws-credentials@v6`、`golangci/golangci-lint-action@v9`) が検査されていました (actionlint 1.7.12 で、存在しない入力を足して確かめた)。SHA にしたあとは、どれも検査されません。入力を変えるときは、その action の `action.yml` を読んで確かめます

### 参考 (固定できないもの・失うもの)

- DOC: <https://docs.github.com/en/actions/reference/security/secure-use>
- DOC: <https://github.com/rhysd/actionlint/blob/main/docs/checks.md#check-popular-action-inputs>
- DOC: <https://docs.github.com/en/code-security/dependabot/ecosystems-supported-by-dependabot/supported-ecosystems-and-repositories>

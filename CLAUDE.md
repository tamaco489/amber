# amber

ECS Fargate のデプロイ方式 (ローリング、ブルー/グリーン、カナリア、リニア) を、ECS ネイティブの仕組みで確かめる検証用リポジトリ。
AWS (東京) に Terraform で構築し、小さな Go のアプリを動かす。環境は `prd` の 1 つだけ。

## 規約とドキュメントの場所

規約は `.claude/rules/` にあり、すべて読み込まれる。**このファイルには規約を再掲しない。**

| ファイル                              | 内容                                                             |
| ------------------------------------- | ---------------------------------------------------------------- |
| `.claude/rules/notion/design-docs.md` | 設計とタスクの置き場所 (Notion)                                  |
| `.claude/rules/github/labels.md`      | ラベルの一覧                                                     |
| `.claude/rules/github/workflow.md`    | ブランチと PR の運用 (gh-flow との差分)                          |
| `.claude/rules/aws/cost.md`           | 動かす時間を短くする検証の進め方                                 |
| `.claude/rules/code/comments.md`      | コードのコメントの書き方 (出典の `DOC:`)                         |
| `.claude/rules/code/go.md`            | Go のコードの書き方 (パッケージ、分岐、戻り値、コメント、テスト) |

`docs/` は規約ではなく現状の説明を持つ。**構成を変更したら一緒に更新する。** 英語 (`.md`) と日本語 (`.ja.md`) の両方を揃える。

| ドキュメント                                                   | 内容                                                                             | 更新する条件                                          |
| -------------------------------------------------------------- | -------------------------------------------------------------------------------- | ----------------------------------------------------- |
| [docs/README.ja.md](docs/README.ja.md)                         | ドキュメントの索引                                                               | 文書を追加・移動したとき                              |
| [docs/infra/runtime.ja.md](docs/infra/runtime.ja.md)           | `enable_runtime` で時間課金のリソースを止める運用                                | `enable_runtime` で作る・消すリソースを増減させたとき |
| [docs/infra/tfstate.ja.md](docs/infra/tfstate.ja.md)           | tfstate のバケットのライフサイクル (`infra/tfstate/lifecycle.json`) と入れる手順 | `infra/tfstate/lifecycle.json` を変えたとき           |
| [docs/infra/notification.ja.md](docs/infra/notification.ja.md) | デプロイの Slack への通知 (`modules/notification/`) とボットトークンを入れる手順 | 通知の内容・ルール・送り先を変えたとき                |
| [docs/cd/task-definition.ja.md](docs/cd/task-definition.ja.md) | Terraform で変えたタスク定義を、`cd-backend` でサービスに届ける手順              | `cd-backend` の複製元を変えたとき                     |
| [docs/cd/notifier.ja.md](docs/cd/notifier.ja.md)               | 通知の Lambda のコードを置く `cd-notifier` (動くとき、ロール、手順、流し直し)    | `cd-notifier` の動くとき・ロール・手順を変えたとき    |
| [docs/ci/workflow-lint.ja.md](docs/ci/workflow-lint.ja.md)     | `ci-actions` の検査 (actionlint、shellcheck) と手元での流し方                    | 検査の対象・ツール・バージョンを変えたとき            |
| [docs/loadtest/k6.ja.md](docs/loadtest/k6.ja.md)               | デプロイ中の k6 の負荷試験 (`loadtest/`) の閾値・失敗の種類・流し方              | 閾値・失敗の種類・k6 の版・レシピを変えたとき         |

## 費用を止める運用

> [!IMPORTANT]
>
> - 時間課金のリソース (ALB、インターフェース型エンドポイント、ECS) は、`infra/envs/prd/terraform.tfvars` の `enable_runtime` で、まとめて作る・消す ([docs/infra/runtime.ja.md](docs/infra/runtime.ja.md))
> - **検証が一区切りついたら、`enable_runtime = false` にして apply するようユーザーに促す。** 動かしたままだと 1 時間あたり約 \$0.14 (1 日あたり約 \$3.4) かかる
> - PR は `enable_runtime = false` の状態で出す。`true` のままだと、`ci-infra` の plan に作り直しの差分が出て、本当の差分が埋もれる
> - 時間課金のリソースや、置いておくだけで料金がかかるリソース (CloudWatch アラームなど) を足すときは、`enable_runtime` で消えるモジュールに入れる

## このリポジトリの禁止事項

- `main` ブランチへの直接コミット・push の禁止
- コミットと push は、毎回ユーザーの承認を得てから行う。ファイルの修正の承認を、コミットの承認とみなさない
- `rm -rf` の使用禁止。ファイルの削除は `rm -f` を使う
- **AWS を変更する操作の禁止。** 以下はユーザーだけが実行する。PreToolUse フック (`.claude/hooks/block-aws-changes.py`) が Bash での実行を止める
  - `terraform apply` / `terraform destroy` (`just infra::apply` と `just infra::destroy-target` を含む)
  - AWS のリソースを作成・変更・削除する CLI (`just infra::allow-my-ip`、`just backend::push` を含む)
  - 読み取り (`describe` / `list` / `get`) と、`terraform plan`、`fmt`、`validate`、tflint、trivy は、確認のために Claude が実行してよい
- 公開リポジトリのため、自分の IP などの個人の情報を、コードと PR のコメントに書かない

## GitHub の運用 (gh-flow)

コミット・Issue・PR は gh-flow プラグインの規約に従う。規約は gh-flow のスキルが実行時に読み込む。

| 作業                      | スキル                           |
| ------------------------- | -------------------------------- |
| コミットから PR まで      | `/gh-flow:smart-commit`          |
| メイン Issue とサブ Issue | `/gh-flow:create-release-issues` |
| リリース PR               | `/gh-flow:create-release-pr`     |
| ラベルの GitHub への反映  | `/gh-flow:sync-labels`           |

- 規約に合わない操作は gh-flow の hooks が拒否する
- ラベルの一覧は `.claude/rules/github/labels.md` に置く
- 直下に新しい領域 (ディレクトリ、CI、CD) を追加したら `/gh-flow:setup` を実行する

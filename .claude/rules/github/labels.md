# GitHub ラベル一覧

Issue と PR に付ける GitHub のラベルと、コミットメッセージの末尾に付けるスコープはこのファイルから選択する。
**Issue と PR のタイトルにはラベルを書かない** (gh-flow の `issue.md`)。ラベルを追加・変更した場合はこのファイルを更新すること。
選び方は gh-flow の `labels.md` に従う。

**このファイルが GitHub 上のラベルの正。** 定義を変更したら `/gh-flow:sync-labels` スキルで GitHub に反映する。
反映は一方向で、GitHub 側の状態をこのファイルへ書き戻さない。

| ラベル     | 色        | 対象スコープ                                                                             |
| ---------- | --------- | ---------------------------------------------------------------------------------------- |
| `backend`  | `#1d3557` | Go の実装 (`backend/` 配下。検証用アプリとフックの Lambda)                               |
| `infra`    | `#8b4513` | Terraform (`infra/` 配下)                                                                |
| `loadtest` | `#c2e812` | k6 の負荷試験 (`loadtest/` 配下)                                                         |
| `ci`       | `#3c1361` | CI ワークフロー (`.github/workflows/ci-*.yaml`)                                          |
| `cd`       | `#24292f` | CD ワークフロー・デプロイ (`.github/workflows/cd-*.yaml`)                                |
| `claude`   | `#f49c51` | Claude Code の設定 (`CLAUDE.md`、`.claude/` 配下)                                        |
| `repo`     | `#0e8a16` | リポジトリ全体 (`README`、`docs/` 配下、`.github/` の非ワークフロー、直下の設定ファイル) |

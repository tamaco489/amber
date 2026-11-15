# ブランチと PR の運用 (gh-flow との差分)

このプロジェクトでは、gh-flow のリリースブランチ方式を使わない。まだ動いているサービスではなく、変更をまとめて出す必要がないため。

## 進め方

- Issue の分け方は gh-flow のとおり (メイン Issue 1 本 + サブ Issue 複数本)。メイン Issue は、サブ Issue の進み具合をまとめる親として残す
- サブ Issue ごとに `main` からブランチを切り、`main` へ直接マージする
- メイン Issue は、すべてのサブ Issue が閉じたら手で閉じる

## ブランチ名と PR

gh-flow の hook はサブ PR (`feature/sub-issue-<番号>/...`) の base が `main` だと拒否するため、サブ Issue でも単体 Issue の形式を使う。

| 項目       | 形式                                                                  |
| ---------- | --------------------------------------------------------------------- |
| ブランチ   | `<種別>/issue-<サブ Issue の番号>/<名前>` (例: `ci/issue-6/ci-infra`) |
| PR の base | `main`                                                                |
| PR の題名  | `[issue-<番号>] [<種別>] <タイトル>`                                  |
| PR の本文  | `.github/PULL_REQUEST_TEMPLATE/single.md`                             |
| Issue      | 本文に `Closes #<番号>` を書き、マージと同時に自動で閉じる            |

- 種別は gh-flow の `commit-types.md` の type を使う (`feat` だけは `feature` と綴る)
- サブ Issue のタイトル (`[sub] [<種別>] ...`) と、メイン Issue との親子の紐付けはそのまま残す
- `/gh-flow:create-release-pr` は使わない

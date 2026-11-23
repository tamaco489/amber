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

## push したあとの CI の監視

> [!IMPORTANT]
> **PR のブランチに push したら、毎回 CI を監視する。** 初回の push も、PR を出したあとの追加の push も同じ。

- 監視は、バックグラウンドの別エージェントに任せる。メインの会話は、監視を待たずに次の作業を進める
- エージェントには、読み取りだけを頼む (ファイルの修正、コミット、push、workflow の再実行、PR へのコメント、AWS と Terraform の操作はさせない)
- エージェントに頼む内容
  - `gh pr checks <番号> --watch` で、すべてのチェックが終わるまで待つ
  - 失敗したチェックは、`gh run view <run-id> --log-failed` で失敗したステップとエラーを取る
  - `ci-infra` の plan のコメントを読み、差分 (作る・変える・作り直す・消すリソース) が想定どおりかを確かめる。想定 (例: apply 済みなら `No changes`) をエージェントに伝えておく
  - plan のコメントは push のたびに同じものを書き換える。見出しの下の `Commit:` が最新のコミットと一致するかで、今回の push の結果かを確かめる (作成日時は最初の push のまま変わらない)
- 結果が届いたら、ユーザーに伝える。失敗していれば、原因と直し方を示す

## サブエージェントの作業用のコピー (git worktree) の片付け

> [!IMPORTANT]
> **サブエージェントが作業用のコピー (`.claude/worktrees/`) で PR を作ったら、メインのセッションがその作業用のコピーをローカルから消す。** 残しておくと、そのブランチを使っていることになり、メインのチェックアウトで `git switch` できなくなる。

- 消すのは、PR を作り終え、エージェントの報告を受け取ったあと。PR のマージは待たない
- 消す前に、次の 2 つを確かめる。どちらかを満たさなければ消さず、ユーザーに伝える
  - `git -C <作業用のコピー> status --short` が空 (未コミットの変更がない)
  - `git merge-base --is-ancestor <作業用のコピーの HEAD> origin/<ブランチ>` が成功する (中身が push 済み)
- 消し方は `git worktree remove <パス>` のあと `git worktree prune`。`rm` では消さない
- 作業用のコピーと一緒に作られる、ローカルだけのブランチ (`worktree-agent-...` など) も、中身が push 済みのブランチに含まれていることを確かめてから `git branch -D` で消す
- エージェントがまだ作業している作業用のコピー (`git worktree list` で `locked` と出るもの) は消さない

# v* のタグのルールセット

[English](./tag-ruleset.md) | [日本語](./tag-ruleset.ja.md)

[ドキュメント一覧](../README.ja.md) に戻る。

`cd-backend` は `v*` のタグの push で本番にデプロイします。deploy 用のロールの信頼ポリシーは `refs/tags/v*` のトークンしか受け付けませんが、タグを push できる人なら、どのコミットにでも `v*` を付けてデプロイできます。
そこで、GitHub のルールセットで `v*` のタグを作成・更新・削除できる人を、リポジトリの管理者だけに絞ります。デプロイの守りを、AWS 側 (信頼ポリシー) と GitHub 側 (ルールセット) の 2 段にします。

## 設定の内容

定義は [`.github/rulesets/tag-v.json`](../../.github/rulesets/tag-v.json) に置き、これを正とします。画面で変更したら、JSON にも反映します。

| 項目             | 値                                                                 |
| ---------------- | ------------------------------------------------------------------ |
| 名前             | `tag-v`                                                            |
| 対象             | タグ (`target: tag`)、`refs/tags/v*`                               |
| ルール           | `creation` / `update` / `deletion` (作成・更新・削除を制限)        |
| 有効化           | `active`                                                           |
| バイパスできる人 | リポジトリの管理者だけ (`RepositoryRole`、`actor_id: 5`、`always`) |

- `RepositoryRole` の `actor_id` は、`maintain` が `2`、`write` が `4`、`admin` が `5` です。GitHub の REST API のドキュメントには値の一覧がなく、[Terraform の GitHub プロバイダーのドキュメント](https://registry.terraform.io/providers/integrations/github/latest/docs/resources/repository_ruleset) の値を使っています。作ったあとに画面で、バイパスが「Repository admin」になっていることを確かめます
- ルールは push のときに効きます。すでにある `v*` のタグと、`workflow_dispatch` でタグを選んで流す再デプロイには影響しません

API の仕様: [Rules for a repository (GitHub 公式)](https://docs.github.com/en/rest/repos/rules)

## 作る・更新する

リポジトリの管理者が手元で実行します。

```sh
# 作る
gh api -X POST repos/tamaco489/amber/rulesets --input .github/rulesets/tag-v.json

# 更新する (JSON を変えたとき)
id=$(gh api repos/tamaco489/amber/rulesets --jq '.[] | select(.name == "tag-v") | .id')
gh api -X PUT "repos/tamaco489/amber/rulesets/${id}" --input .github/rulesets/tag-v.json
```

## 確かめる

### 1. ルールセットが有効になっている

```sh
id=$(gh api repos/tamaco489/amber/rulesets --jq '.[] | select(.name == "tag-v") | .id')
gh api "repos/tamaco489/amber/rulesets/${id}" \
  --jq '{name, target, enforcement, conditions, rules, bypass_actors, current_user_can_bypass}'
```

`enforcement` が `active`、`bypass_actors` が `actor_id: 5` の `RepositoryRole` だけ、管理者で実行したときの `current_user_can_bypass` が `always` なら想定どおりです。

### 2. 管理者は push でき、cd-backend がこれまでどおり動く

次にデプロイするときに、いつもどおり管理者がタグを push します。

```sh
git tag v<次の番号>
git push origin v<次の番号>
gh run list --workflow cd-backend.yaml --limit 1
```

push が拒否されず、`cd-backend` の run が始まって成功すれば想定どおりです。

### 3. 管理者以外は止まる

管理者以外が `v*` のタグを push すると、`GH013: Repository rule violations found` で拒否されます。
collaborator がオーナーだけのため、次のどちらかで確かめます。

- **別のアカウントで確かめる (推奨)**: `write` の権限で招待し、そのアカウントで `git push origin v0-ruleset-test` を実行して拒否されることを見る。終わったら招待を外す
- **バイパスを一時的に外して確かめる**: JSON の `bypass_actors` を `[]` にしたものを `PUT` し、オーナーで `git push origin v0-ruleset-test` を実行して拒否されることを見る。終わったら元の JSON を `PUT` し直す

> [!IMPORTANT]
> 拒否されずに push できてしまうと、`cd-backend` が動いてデプロイが始まります。確かめるのは `enable_runtime = false` のとき (ECS が無く、デプロイが途中で失敗する) にします。push できてしまったら、`gh run cancel` で run を止め、`git push origin :refs/tags/v0-ruleset-test` でタグを消します。

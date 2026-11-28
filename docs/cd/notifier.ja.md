# 通知の Lambda のデプロイ

[English](./notifier.md) | [日本語](./notifier.ja.md)

[ドキュメント一覧](../README.ja.md)に戻る。

デプロイの通知の Lambda (`backend/cmd/notifier`) のコードを、`cd-notifier` のワークフローで AWS に置きます。
関数そのものは Terraform (`infra/modules/notification/lambda.tf`) が作り、コードだけをワークフローが置き換えます。

## 仕組み

- Terraform は、関数 `prd-amber-notification` を、動かない仮のコード (`bootstrap` だけを入れた zip) で作ります。コードの変更 (`filename`、`source_code_hash`) は無視するため、apply してもワークフローが置いたコードは戻りません
- ランタイム、メモリ、タイムアウト、環境変数、ロール、ログの送り先は Terraform が管理します。変えるときは Terraform で apply します (ワークフローを流し直す必要はありません)
- 関数は置いておくだけでは料金がかからない (リクエストと実行時間の課金) ため、`enable_runtime` では消さずに常に置きます

| 項目     | 値                                                                        |
| -------- | ------------------------------------------------------------------------- |
| 関数     | `prd-amber-notification` (`provided.al2023`、`arm64`、128 MB、30 秒)      |
| ロール   | `prd-amber-github-actions-notifier`                                       |
| 信頼条件 | このリポジトリの `main` のブランチから発行された OIDC のトークンだけ      |
| 権限     | この関数の `lambda:UpdateFunctionCode`、`lambda:GetFunctionConfiguration` |

## 動くとき

- `main` への push で、次のどれかが変わったとき
  - `backend/cmd/notifier/**`、`backend/internal/**`、`backend/go.mod`、`backend/go.sum`
  - `backend/internal/server/**`、`backend/internal/health/**`、`backend/internal/checkout/**`、`backend/internal/failmode/**` だけの変更では動きません。検証用のアプリだけが読み込むパッケージで、notifier のコードは変わらないためです
  - `.github/workflows/cd-notifier.yaml`、`.github/workflows/scripts/cd-notifier/**`
- `workflow_dispatch` で手で流したとき。ref には `main` を選びます。ロールは `main` のトークンしか信頼しないため、ほかのブランチやタグを選ぶと AWS の認証で失敗します
- 同時には 1 つずつ流れます (`concurrency: cd-notifier`)。途中の実行は打ち切りません

## 手順 (ワークフローの中身)

1. `backend/go.mod` の Go で、`linux/arm64` の `bootstrap` をビルドし、zip にする (`scripts/cd-notifier/build.sh`)。条件は `just backend::build-notifier` と同じです
2. ロール `prd-amber-github-actions-notifier` を引き受ける。ARN は、リポジトリの Variables の `AWS_NOTIFIER_DEPLOY_ROLE_ARN` に置きます
3. `aws lambda update-function-code` でコードを置き換え、`aws lambda wait function-updated` で、`LastUpdateStatus` が `Successful` になるまで待つ (`scripts/cd-notifier/update-function-code.sh`)
4. 関数、コミット、コードのハッシュ (`CodeSha256`) を、実行結果のサマリーに書く

## 初めて置くとき

1. ユーザーが Terraform で apply する (`just infra::apply prd`)。関数とロールができます。この時点の関数は、仮のコードのため動きません
2. ロールの ARN を、リポジトリの Variables に入れる

   ```sh
   gh variable set AWS_NOTIFIER_DEPLOY_ROLE_ARN \
     --body "$(terraform -chdir=infra/envs/prd output -raw notifier_deploy_role_arn)"
   ```

3. GitHub の Actions で `cd-notifier` を開き、`Run workflow` の ref に `main` を選んで実行する

## 流し直す

コードを変えずに置き直すときも、`cd-notifier` を `main` で `workflow_dispatch` します。置いたコードは、次のコマンドで確かめられます。

```sh
aws lambda get-function-configuration \
  --function-name prd-amber-notification \
  --profile tamaco489 --region ap-northeast-1 \
  | jq '{State, LastUpdateStatus, CodeSha256, LastModified}'
```

`CodeSha256` が、実行結果のサマリーの値と同じなら、そのコードが置かれています。

## 参考

- [Deploy Go Lambda functions with .zip file archives (AWS 公式)](https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html)
- [aws lambda wait function-updated (AWS 公式)](https://docs.aws.amazon.com/cli/latest/reference/lambda/wait/function-updated.html)
- [Fine-tuning the Resources and Conditions sections of policies (AWS 公式)](https://docs.aws.amazon.com/lambda/latest/dg/lambda-api-permissions-ref.html)

# デプロイの Slack への通知

[English](./notification.md) | [日本語](./notification.ja.md)

[ドキュメント一覧](../README.ja.md) に戻る。

デプロイの状態の変化を、Slack のチャンネル `#amber-prd-deploy` に投稿します。
EventBridge のルールがイベントを拾い、API 送信先で Slack の `chat.postMessage` を呼びます。Lambda は使いません。
定義は `infra/modules/notification/` にあります。

## 通知する内容

| 通知                         | 出どころ                                                        | 本文に載せるもの                                                 |
| ---------------------------- | --------------------------------------------------------------- | ---------------------------------------------------------------- |
| デプロイを始めた             | `cd-backend` が `update-service` のあとに送るイベント           | バージョン、起動した人、デプロイ ID、タスク定義、実行の URL など |
| デプロイの進行中・完了・失敗 | ECS の `ECS Deployment State Change` (サービス `prd-amber-api`) | イベント名、デプロイ ID、理由 (`reason`)、時刻 (UTC)             |

- ECS が送るのは `SERVICE_DEPLOYMENT_IN_PROGRESS`、`SERVICE_DEPLOYMENT_COMPLETED`、`SERVICE_DEPLOYMENT_FAILED` の 3 つです
- **ロールバックは専用のイベントがありません。** ロールバックも `SERVICE_DEPLOYMENT_IN_PROGRESS` で届き、違いは `reason` だけです (例: `ECS deployment circuit breaker: rolling back to deploymentId ...`)。そのため、`reason` を本文に載せて見分けます
- `reason` は説明文で、文面が変わることがあります。そのため、ルールの絞り込みには使わず、そのまま載せています

仕様: [Amazon ECS service deployment state change events (AWS 公式)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html)

## 構成

| リソース                                                         | 役割                                                                                   |
| ---------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| 接続 `prd-amber-slack`                                           | ボットトークンを `Authorization: Bearer ...` のヘッダーで付ける                        |
| API 送信先 `prd-amber-slack-post-message`                        | `chat.postMessage` を、1 秒に 1 回までの頻度で呼ぶ                                     |
| ルール `prd-amber-ecs-deployment-{in-progress,completed,failed}` | ECS のデプロイの状態の変化を、`eventName` ごとに拾う (default のイベントバス)          |
| ルール `prd-amber-deploy-started`                                | `cd-backend` が送るイベント (`amber.cd-backend` / `Deployment Started`) を拾う         |
| IAM ロール `prd-amber-notification-invoke`                       | EventBridge が API 送信先を呼ぶ。信頼するのは通知のルールだけ                          |
| SQS キュー `prd-amber-notification-dlq`                          | 再試行しても届けられなかった通知を 3 日残す (デッドレターキュー)                       |
| deploy 用のロールの `events:PutEvents`                           | `cd-backend` から送れるのは、上の `source` と `detail-type` の組み合わせのイベントだけ |

- 本文は、ルールのターゲットの入力変換で、Slack の Block Kit の JSON に組みます。色の帯を付けるため `attachments` に入れます (開始: 青、進行中: 黄、完了: 緑、失敗: 赤)
- ECS のルールを `eventName` ごとに分けるのは、色と見出しを変えるためです。`eventName` は決まった値なので、絞り込みに使えます
- 届けられなかったときは、1 時間まで、最大 10 回まで再試行します。それでも届かなければ、デッドレターキューに入ります
- どれも、置いておくだけでは料金がかかりません (呼び出しやリクエストごとの課金)。そのため `enable_runtime` で消さず、常に置きます。接続のトークンの料金 (Secrets Manager) は、API 送信先の料金に含まれます

### これから置く Lambda の土台

通知は、デプロイごとに 1 つのスレッドにまとめる Lambda (Go) に移ります。Lambda が使うリソースは先に置いてありますが、関数はまだ置いておらず、今の通知は変わりません。

| リソース                                                       | 役割                                                                                           |
| -------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| DynamoDB のテーブル `prd-amber-notification`                   | デプロイごとの状態 (キーはデプロイ ID)。オンデマンドで、古い項目は TTL (`expires_at`) で消える |
| SSM のパラメータ `/prd-amber/notification/slack-bot-token`     | Slack のボットトークン (標準の `SecureString`)。ダミー値で作り、本物の値は CLI で入れる        |
| IAM ロール `prd-amber-notification-lambda`                     | 実行ロール。テーブル、パラメータ、ロググループ、デッドレターキューへの送信                     |
| CloudWatch のロググループ `/aws/lambda/prd-amber-notification` | 関数 `prd-amber-notification` のログを 3 日残す                                                |

- Terraform は、パラメータを書き込み専用の `value_wo` で書くため、CLI で入れた本物のトークンは state に入りません
- これらも置いておくだけなら $0 のため、`enable_runtime` では消しません

## ボットトークンを入れる

Terraform は、接続をダミーのトークンで作ります。本物のトークンを state に残さないためです。
Terraform で接続を作ったあと、ユーザーが AWS CLI で本物のトークンを入れます (AWS を変更する操作)。最初の 1 回だけで済みます。Terraform は、入れたトークンをダミーに戻しません (`ignore_changes`)。

Slack アプリの作り方と、トークンの控え方は、Notion の実装タスク「Slack への通知を作る」にあります。トークンは `tmp/slack-app-settings/bot-user-oauth-token` (Git の対象外) に控えている前提です。

```sh
SLACK_BOT_TOKEN=$(cat tmp/slack-app-settings/bot-user-oauth-token)

aws events update-connection \
  --name prd-amber-slack \
  --authorization-type API_KEY \
  --auth-parameters "ApiKeyAuthParameters={ApiKeyName=Authorization,ApiKeyValue=Bearer ${SLACK_BOT_TOKEN}}" \
  --profile tamaco489 --region ap-northeast-1 \
  | jq '{ConnectionArn, ConnectionState}'

unset SLACK_BOT_TOKEN
```

`ConnectionState` が `AUTHORIZED` になれば入っています。

## 届かないときに確かめること

- **デッドレターキュー:** 再試行しても届かなかった通知が入ります。メッセージの属性 (`ERROR_CODE`、`ERROR_MESSAGE`) に理由が付きます

  ```sh
  aws sqs receive-message \
    --queue-url "$(terraform -chdir=infra/envs/prd output -raw notification_dead_letter_queue_url)" \
    --message-attribute-names All --max-number-of-messages 10 \
    --profile tamaco489 --region ap-northeast-1
  ```

- **ルールのメトリクス:** CloudWatch の `AWS/Events` の `Invocations`、`FailedInvocations` (ディメンション `RuleName`) で、ルールが動いたかと、呼び出しに失敗したかが分かります

## 制約

- **Slack は、多くのエラーを HTTP 200 で返します。** トークンの誤り (`invalid_auth`)、ボットがチャンネルにいない (`not_in_channel`) などは、本文の `ok` が `false` になるだけです。EventBridge は成功として扱うため、再試行もデッドレターキューへの送信もしません。届かないときは、Notion の手順 (Slack アプリの作り方の 4) のとおり `chat.postMessage` を手で呼び、`error` を確かめます
- **EventBridge は、埋め込む値をエスケープしません。** `reason` などに `"` が入ると本文の JSON が壊れ、その通知は届きません。`cd-backend` が送る値 (タグ、ARN、ユーザー名、URL) には `"` は入りません

## 参考

- [Amazon EventBridge input transformation (AWS 公式)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-transform-target-input.html)
- [API destinations (AWS 公式)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-api-destinations.html)
- [How Amazon EventBridge uses AWS Secrets Manager (AWS 公式)](https://docs.aws.amazon.com/secretsmanager/latest/userguide/integrating_how-services-use-secrets_events.html)
- [chat.postMessage (Slack 公式)](https://docs.slack.dev/reference/methods/chat.postMessage)

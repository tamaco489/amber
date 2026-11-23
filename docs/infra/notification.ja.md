# デプロイの Slack への通知

[English](./notification.md) | [日本語](./notification.ja.md)

[ドキュメント一覧](../README.ja.md) に戻る。

デプロイの状態の変化を、Slack のチャンネル `#amber-prd-deploy` に投稿します。
EventBridge のルールがイベントを拾い、通知の Lambda (`backend/cmd/notifier`) を呼びます。Lambda は Slack の Web API を呼び、1 回のデプロイの通知を 1 つのスレッドにまとめます。
定義は `infra/modules/notification/` にあります。

1. ECS と `cd-backend` が、デプロイのイベントを default のイベントバスに送る
2. EventBridge のルール (4 本) が拾い、イベントをそのまま Lambda `prd-amber-notification` に渡す (非同期の呼び出し)
3. Lambda が、Slack の `chat.postMessage` (親と返信)、`chat.update` (親の書き換え)、`reactions.add` (スタンプ) を呼ぶ
4. 失敗したものは、SQS の DLQ (デッドレターキュー) `prd-amber-notification-dlq` に入る (ルールが Lambda に届けられなかったときと、Lambda が再試行しても失敗したときの両方)

## 通知する内容

| 通知                         | 出どころ                                                        |
| ---------------------------- | --------------------------------------------------------------- |
| デプロイを始めた             | `cd-backend` が `update-service` のあとに送るイベント           |
| デプロイの進行中・完了・失敗 | ECS の `ECS Deployment State Change` (サービス `prd-amber-api`) |

- ECS が送るのは `SERVICE_DEPLOYMENT_IN_PROGRESS`、`SERVICE_DEPLOYMENT_COMPLETED`、`SERVICE_DEPLOYMENT_FAILED` の 3 つです
- **ロールバックは専用のイベントがありません。** ロールバックも `SERVICE_DEPLOYMENT_IN_PROGRESS` で届き、違いは `reason` だけです (例: `ECS deployment circuit breaker: rolling back to deploymentId ...`)。Lambda が `reason` を見て、ロールバックとして扱います
- `reason` は説明文で、文面が変わることがあります。そのため、ルールの絞り込みには使いません

仕様: [Amazon ECS service deployment state change events (AWS 公式)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html)

## スレッドのまとめ方

デプロイ ID (`ecs-svc/...`) ごとに、親の投稿を 1 つ作り、以降の通知はそのスレッドに返信します。

| 届いたイベント        | スレッドへの返信                                     | 親の投稿                                                                    |
| --------------------- | ---------------------------------------------------- | --------------------------------------------------------------------------- |
| 最初のイベント        | (下の行と同じ)                                       | 作る (青、`Deploying`)                                                      |
| デプロイを始めた      | `Deploy started` (同じタグの流し直しなら `Redeploy`) | バージョン、起動した人、タスク定義、実行のリンクを埋める                    |
| 進行中                | `Deployment in progress` (黄)                        | 変えない                                                                    |
| 進行中 (ロールバック) | `Rollback started` (赤)                              | 赤、`Rolling back`                                                          |
| 完了                  | `Deployment completed` (緑)                          | 緑、`Completed`。完了した時刻を足し、スタンプ `:white_check_mark:` を付ける |
| 失敗                  | `Deployment failed` (赤)                             | 赤、`Failed`。スタンプ `:x:` を付ける                                       |

- 親を作るのは、最初に届いたイベントです。ECS は `update-service` の時点で進行中のイベントを出すため、多くの場合、開始より進行中が先に届きます。そのときの親の見出しには、開始が届くまでバージョンが入りません
- 親の状態は進む方向にだけ変わります (完了のあとに遅れて届いた進行中で、`Deploying` に戻りません)
- 親の `ts` と、手順 (親の作成、返信、書き換え、スタンプ) ごとの済んだ印を、DynamoDB のテーブルに残します。再試行で同じイベントが届いても、同じ返信は 2 回付きません
- 本文は Lambda が組み立てるため、`reason` などの値に `"` が入っても壊れません

## 構成

| リソース                                                         | 役割                                                                                                                                     |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| ルール `prd-amber-ecs-deployment-{in-progress,completed,failed}` | ECS のデプロイの状態の変化を、`eventName` ごとに拾う (default のイベントバス)                                                            |
| ルール `prd-amber-deploy-started`                                | `cd-backend` が送るイベント (`amber.cd-backend` / `Deployment Started`) を拾う                                                           |
| ルールのターゲット                                               | 関数 `prd-amber-notification`。入力変換はせず、イベントをそのまま渡す。届けられなければ 1 時間・10 回まで再試行し、 DLQ へ               |
| 関数のリソースベースのポリシー                                   | EventBridge (`events.amazonaws.com`) が、上の 4 本のルールからだけ関数を呼べる (`source_arn` で絞る)                                     |
| Lambda の関数 `prd-amber-notification`                           | `provided.al2023`、`arm64`、128 MB、タイムアウト 30 秒。コードは `cd-notifier` が置く ([通知の Lambda のデプロイ](../cd/notifier.ja.md)) |
| 関数の非同期の呼び出しの設定                                     | 関数がエラーを返したら 2 回まで再試行し、1 時間より古いものは捨てる。失敗したものは DLQ に送る                                           |
| DynamoDB のテーブル `prd-amber-notification`                     | デプロイごとの親の投稿と、手順ごとの済んだ印 (キーは `pk`)。オンデマンドで、3 日後に TTL (`expires_at`) で消える                         |
| SSM のパラメータ `/prd-amber/notification/slack-bot-token`       | Slack のボットトークン (標準の `SecureString`)。ダミー値で作り、本物の値は CLI で入れる                                                  |
| IAM ロール `prd-amber-notification-lambda`                       | 実行ロール。テーブル、パラメータ、ロググループ、 DLQ への送信                                                                            |
| CloudWatch のロググループ `/aws/lambda/prd-amber-notification`   | 関数のログを 3 日残す                                                                                                                    |
| SQS キュー `prd-amber-notification-dlq`                          | 通知に失敗したイベントを 3 日残す (DLQ)。中身は下の「 DLQ 」の節                                                                         |
| deploy 用のロールの `events:PutEvents`                           | `cd-backend` から送れるのは、上の `source` と `detail-type` の組み合わせのイベントだけ                                                   |

- ECS のルールを `eventName` ごとに分けているのは、`eventName` が決まった値で、絞り込みに使えるためです
- Lambda のターゲットはロールを使いません。呼ぶ許可は、関数のリソースベースのポリシーで与えます
- Terraform は、パラメータを書き込み専用の `value_wo` で書くため、CLI で入れた本物のトークンは state に入りません
- Terraform は、関数を動かない仮のコードで作り、コードの変更は無視します。コードは `cd-notifier` が置きます
- 関数のタイムアウト (30 秒) は、手順の印の期限 (`backend/internal/dynamodb` の 60 秒) より短くしています。長く動くと、再試行の呼び出しが同じ手順を並べて進めるためです
- どれも、置いておくだけでは料金がかかりません (呼び出し、リクエスト、実行時間ごとの課金)。そのため `enable_runtime` で消さず、常に置きます

## Slack アプリの権限

ボットトークンには、次のスコープが要ります。足したら、Slack アプリをワークスペースに入れ直します (入れ直すまで、新しいスコープはトークンに付きません)。

| スコープ          | 使う API                                                    |
| ----------------- | ----------------------------------------------------------- |
| `chat:write`      | `chat.postMessage` (親と返信)、`chat.update` (親の書き換え) |
| `reactions:write` | `reactions.add` (完了と失敗のスタンプ)                      |

ボットは、チャンネル `#amber-prd-deploy` に招待しておきます。

## ボットトークンを入れる

Terraform は、パラメータをダミーのトークンで作ります。本物のトークンを state に残さないためです。
Terraform でパラメータを作ったあと、ユーザーが AWS CLI で本物のトークンを入れます (AWS を変更する操作)。Terraform は、入れたトークンをダミーに戻しません (`value_wo_version` を上げない限り書き込まない)。

Slack アプリの作り方と、トークンの控え方は、Notion の実装タスク「Slack への通知を作る」にあります。トークンは `tmp/slack-app-settings/bot-user-oauth-token` (Git の対象外) に控えている前提です。

```sh
SLACK_BOT_TOKEN=$(cat tmp/slack-app-settings/bot-user-oauth-token)

aws ssm put-parameter \
  --name /prd-amber/notification/slack-bot-token \
  --type SecureString \
  --value "${SLACK_BOT_TOKEN}" \
  --overwrite \
  --profile tamaco489 --region ap-northeast-1 \
  | jq .

unset SLACK_BOT_TOKEN
```

- 出力の `Version` が 1 つ増えていれば入っています
- 関数は、トークンをコールドスタートのときに 1 回だけ読みます。入れ替えた直後は、動いている実行環境が古いトークンを使い続けます。すぐに切り替えたいときは、`cd-notifier` を流し直します (コードを置き直すと、実行環境が作り直されます)
- アプリを入れ直してもトークンの値が変わらなければ、入れ直す必要はありません。値が変わったら、上の手順で入れ直します

## DLQ

通知に失敗したイベントは、キュー `prd-amber-notification-dlq` に 3 日残ります。入る経路は 2 つあり、メッセージの形が違います。

| 経路                           | 入るとき                                                                                        | メッセージ                                                                                                                              |
| ------------------------------ | ----------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| ルール → キュー                | EventBridge が、再試行しても Lambda に届けられなかったとき (スロットリング、呼ぶ許可の不足など) | 本文は元のイベント。属性 `RULE_ARN`、`TARGET_ARN`、`ERROR_CODE`、`ERROR_MESSAGE` に理由が付く                                           |
| Lambda の失敗の送り先 → キュー | 関数がエラーを返し、2 回の再試行でも失敗したとき (Slack が `ok: false` や 5xx を返した、など)   | 本文は呼び出しの記録。`requestContext.condition` (`RetriesExhausted` など)、`requestPayload` (元のイベント)、`responsePayload` (エラー) |

- EventBridge は、Lambda が非同期の呼び出しを受け付けた時点で成功とみなします。そのため、関数のエラーは EventBridge の再試行に乗らず、Lambda の側の再試行と失敗の送り先で扱います
- Slack は、多くのエラー (`invalid_auth`、`not_in_channel`、`missing_scope` など) を HTTP 200 と `ok: false` で返します。Lambda は `ok: false` をエラーとして返すため、再試行のあとにキューに入ります
- キューのポリシーは、通知のルールからの送信だけを許しています。Lambda からの送信は、実行ロールの許可で届きます (同じアカウントのため)

### 中を見る

件数を見ます。

```sh
aws sqs get-queue-attributes \
  --queue-url "$(terraform -chdir=infra/envs/prd output -raw notification_dead_letter_queue_url)" \
  --attribute-names ApproximateNumberOfMessages \
  --profile tamaco489 --region ap-northeast-1 \
  | jq .Attributes
```

メッセージを見ます。受け取ったメッセージは、可視性タイムアウトの間だけ見えなくなります (消えません)。

```sh
aws sqs receive-message \
  --queue-url "$(terraform -chdir=infra/envs/prd output -raw notification_dead_letter_queue_url)" \
  --message-attribute-names All \
  --max-number-of-messages 10 \
  --profile tamaco489 --region ap-northeast-1 \
  | jq '.Messages[] | {attributes: .MessageAttributes, body: (.Body | fromjson)}'
```

## 届かないときに確かめること

- **関数のログ:** 1 つのイベントごとに `handle event` と `handled event` (親の `ts`、親を作ったか) が出ます。失敗すると、エラーが出ます

  ```sh
  aws logs tail /aws/lambda/prd-amber-notification \
    --since 1h \
    --profile tamaco489 --region ap-northeast-1
  ```

- **DLQ:** 上の「中を見る」のとおりです
- **ルールのメトリクス:** CloudWatch の `AWS/Events` の `Invocations`、`FailedInvocations` (ディメンション `RuleName`) で、ルールが動いたかと、Lambda に届けられたかが分かります
- **関数のメトリクス:** CloudWatch の `AWS/Lambda` の `Errors`、`AsyncEventsDropped`、`DestinationDeliveryFailures` (ディメンション `FunctionName`) で、関数のエラーと、失敗の送り先に送れたかが分かります

## 参考

- [Using resource-based policies for Amazon EventBridge (AWS 公式)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-use-resource-based.html)
- [Configuring a dead-letter queue for an EventBridge rule (AWS 公式)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-rule-dlq.html)
- [Handling errors for asynchronous invocations (AWS 公式)](https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-error-handling.html)
- [Capturing records of Lambda asynchronous invocations (AWS 公式)](https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-retain-records.html)
- [chat.postMessage (Slack 公式)](https://docs.slack.dev/reference/methods/chat.postMessage)
- [chat.update (Slack 公式)](https://docs.slack.dev/reference/methods/chat.update)
- [reactions.add (Slack 公式)](https://docs.slack.dev/reference/methods/reactions.add)

# Deployment notifications to Slack

[English](./notification.md) | [日本語](./notification.ja.md)

Back to the [documentation index](../README.md).

Deployment state changes are posted to the Slack channel `#amber-prd-deploy`.
EventBridge rules pick up the events, and an API destination calls Slack's `chat.postMessage`. No Lambda is used.
The definitions are in `infra/modules/notification/`.

## What is notified

| Notification                         | Source                                                      | Included in the message                                       |
| ------------------------------------ | ----------------------------------------------------------- | ------------------------------------------------------------- |
| Deployment started                   | The event `cd-backend` sends after `update-service`         | Version, actor, deployment ID, task definition, run URL, etc. |
| Deployment in progress, done, failed | ECS `ECS Deployment State Change` (service `prd-amber-api`) | Event name, deployment ID, reason (`reason`), time (UTC)      |

- ECS sends three events: `SERVICE_DEPLOYMENT_IN_PROGRESS`, `SERVICE_DEPLOYMENT_COMPLETED`, and `SERVICE_DEPLOYMENT_FAILED`
- **There is no dedicated rollback event.** A rollback also arrives as `SERVICE_DEPLOYMENT_IN_PROGRESS`, and only `reason` differs (e.g. `ECS deployment circuit breaker: rolling back to deploymentId ...`). So the message includes `reason` to tell them apart
- `reason` is free text whose wording may change. It is therefore not used to filter the rules, only shown as is

Spec: [Amazon ECS service deployment state change events (AWS)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html)

## Resources

| Resource                                                        | Role                                                                                          |
| --------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Connection `prd-amber-slack`                                    | Adds the bot token as the `Authorization: Bearer ...` header                                  |
| API destination `prd-amber-slack-post-message`                  | Calls `chat.postMessage`, at most once per second                                             |
| Rules `prd-amber-ecs-deployment-{in-progress,completed,failed}` | Pick up ECS deployment state changes, one rule per `eventName` (default event bus)            |
| Rule `prd-amber-deploy-started`                                 | Picks up the events `cd-backend` sends (`amber.cd-backend` / `Deployment Started`)            |
| IAM role `prd-amber-notification-invoke`                        | Lets EventBridge call the API destination. Trusts only the notification rules                 |
| SQS queue `prd-amber-notification-dlq`                          | Keeps notifications that could not be delivered after retries, for 3 days (dead-letter queue) |
| `events:PutEvents` on the deploy role                           | `cd-backend` can send only events with the `source` and `detail-type` above                   |

- The message is built as Slack Block Kit JSON by the input transformer of each rule target. It goes in `attachments` to get a color bar (started: blue, in progress: yellow, completed: green, failed: red)
- The ECS rules are split by `eventName` to change the color and heading. `eventName` is a fixed value, so it is safe to filter on
- Failed deliveries are retried for up to 1 hour and 10 attempts. If they still fail, they go to the dead-letter queue
- None of these cost anything while idle (billed per invocation or request). So they are not removed by `enable_runtime` and always exist. The cost of the connection's token (Secrets Manager) is included in the API destination charge

### Base resources for the upcoming Lambda

The notifications will move to a Lambda (Go) that groups them into one thread per deployment. The resources it uses already exist, but no function is deployed yet and the current notifications do not change.

| Resource                                                  | Role                                                                                           |
| --------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| DynamoDB table `prd-amber-notification`                   | Per-deployment state (key: deployment ID). On-demand, old items expire by TTL (`expires_at`)   |
| SSM parameter `/prd-amber/notification/slack-bot-token`   | Slack bot token (standard `SecureString`). Created with a dummy value; set the real one by CLI |
| IAM role `prd-amber-notification-lambda`                  | Execution role: the table, the parameter, the log group, and sending to the dead-letter queue  |
| CloudWatch log group `/aws/lambda/prd-amber-notification` | Logs of the function `prd-amber-notification`, kept for 3 days                                 |

- Terraform writes the parameter with the write-only `value_wo`, so the real token put by the CLI never reaches the state
- These also cost $0 while idle, so they are not removed by `enable_runtime`

## Set the bot token

Terraform creates the connection with a dummy token, so that the real token is not stored in the state.
After Terraform creates the connection, the user sets the real token with the AWS CLI (an operation that changes AWS). This is needed only once. Terraform does not revert the token to the dummy (`ignore_changes`).

How to create the Slack app and save the token is described in the Notion implementation task "Slack への通知を作る". The token is assumed to be saved in `tmp/slack-app-settings/bot-user-oauth-token` (ignored by Git).

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

The token is set when `ConnectionState` is `AUTHORIZED`.

## When notifications do not arrive

- **Dead-letter queue:** Notifications that could not be delivered after retries go here. The message attributes (`ERROR_CODE`, `ERROR_MESSAGE`) carry the reason

  ```sh
  aws sqs receive-message \
    --queue-url "$(terraform -chdir=infra/envs/prd output -raw notification_dead_letter_queue_url)" \
    --message-attribute-names All --max-number-of-messages 10 \
    --profile tamaco489 --region ap-northeast-1
  ```

- **Rule metrics:** `Invocations` and `FailedInvocations` in the CloudWatch namespace `AWS/Events` (dimension `RuleName`) show whether a rule fired and whether the call failed

## Limitations

- **Slack returns many errors with HTTP 200.** A wrong token (`invalid_auth`) or a bot not in the channel (`not_in_channel`) only sets `ok` to `false` in the body. EventBridge treats it as success, so it neither retries nor sends it to the dead-letter queue. If notifications do not arrive, call `chat.postMessage` by hand as in the Notion steps (step 4 of the Slack app setup) and check `error`
- **EventBridge does not escape the values it inserts.** If `reason` or another value contains `"`, the JSON of the message breaks and that notification is not delivered. The values `cd-backend` sends (tag, ARN, user name, URL) do not contain `"`

## References

- [Amazon EventBridge input transformation (AWS)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-transform-target-input.html)
- [API destinations (AWS)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-api-destinations.html)
- [How Amazon EventBridge uses AWS Secrets Manager (AWS)](https://docs.aws.amazon.com/secretsmanager/latest/userguide/integrating_how-services-use-secrets_events.html)
- [chat.postMessage (Slack)](https://docs.slack.dev/reference/methods/chat.postMessage)

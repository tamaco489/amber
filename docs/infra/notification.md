# Deployment notifications to Slack

[English](./notification.md) | [日本語](./notification.ja.md)

Back to the [documentation index](../README.md).

Deployment state changes are posted to the Slack channel `#amber-prd-deploy`.
EventBridge rules pick up the events and invoke the notification Lambda (`backend/cmd/notifier`). The Lambda calls the Slack Web API and groups the notifications of one deployment into one thread.
The definitions are in `infra/modules/notification/`.

1. ECS and `cd-backend` send deployment events to the default event bus
2. Four EventBridge rules pick them up and pass each event as is to the Lambda `prd-amber-notification` (asynchronous invocation)
3. The Lambda calls Slack's `chat.postMessage` (parent and replies), `chat.update` (rewriting the parent), and `reactions.add` (reactions)
4. Failures go to the SQS dead-letter queue `prd-amber-notification-dlq` (both when a rule cannot deliver to the Lambda and when the Lambda still fails after its retries)

## What is notified

| Notification                         | Source                                                      |
| ------------------------------------ | ----------------------------------------------------------- |
| Deployment started                   | The event `cd-backend` sends after `update-service`         |
| Deployment in progress, done, failed | ECS `ECS Deployment State Change` (service `prd-amber-api`) |

- ECS sends three events: `SERVICE_DEPLOYMENT_IN_PROGRESS`, `SERVICE_DEPLOYMENT_COMPLETED`, and `SERVICE_DEPLOYMENT_FAILED`
- **There is no dedicated rollback event.** A rollback also arrives as `SERVICE_DEPLOYMENT_IN_PROGRESS`, and only `reason` differs (e.g. `ECS deployment circuit breaker: rolling back to deploymentId ...`). The Lambda looks at `reason` and treats it as a rollback
- `reason` is free text whose wording may change. It is therefore not used to filter the rules

Spec: [Amazon ECS service deployment state change events (AWS)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html)

## How the thread is built

For each deployment ID (`ecs-svc/...`), one parent post is created, and later notifications are replies in its thread.

| Event                  | Reply in the thread                                       | Parent post                                                                        |
| ---------------------- | --------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| The first event        | (same as the rows below)                                  | Created (black, `Deploying`)                                                       |
| Deployment started     | `Deploy started` (`Redeploy` when the same tag is re-run) | Fills in the version, actor, task definition, and run link                         |
| In progress            | `Deployment in progress` (yellow)                         | Unchanged                                                                          |
| In progress (rollback) | `Rollback started` (red)                                  | Red, `Rolling back`                                                                |
| Completed              | `Deployment completed` (green)                            | Green, `Completed`. Adds the completion time and the `:white_check_mark:` reaction |
| Failed                 | `Deployment failed` (red)                                 | Red, `Failed`. Adds the `:x:` reaction                                             |

- The parent is created by whichever event arrives first. ECS sends the in-progress event at `update-service`, so it usually arrives before the started event. In that case the parent heading has no version until the started event arrives
- The parent status only moves forward (a late in-progress event after completion does not bring it back to `Deploying`)
- The parent `ts` and a done marker for each step (creating the parent, replying, rewriting, reacting) are kept in the DynamoDB table. Even if a retry delivers the same event again, the same reply is not posted twice
- The Lambda builds the message, so a `"` in `reason` or other values does not break it

## Resources

| Resource                                                        | Role                                                                                                                                                                  |
| --------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Rules `prd-amber-ecs-deployment-{in-progress,completed,failed}` | Pick up ECS deployment state changes, one rule per `eventName` (default event bus)                                                                                    |
| Rule `prd-amber-deploy-started`                                 | Picks up the events `cd-backend` sends (`amber.cd-backend` / `Deployment Started`)                                                                                    |
| Rule targets                                                    | The function `prd-amber-notification`. No input transformer; the event is passed as is. Retries delivery for up to 1 hour and 10 attempts, then the dead-letter queue |
| Resource-based policy of the function                           | Lets EventBridge (`events.amazonaws.com`) invoke the function only from the four rules above (scoped with `source_arn`)                                               |
| Lambda function `prd-amber-notification`                        | `provided.al2023`, `arm64`, 128 MB, 30-second timeout. The code is deployed by `cd-notifier` ([Deploying the notification Lambda](../cd/notifier.md))                 |
| Asynchronous invocation config of the function                  | Retries up to 2 times when the function returns an error and drops events older than 1 hour. Failed ones go to the dead-letter queue                                  |
| DynamoDB table `prd-amber-notification`                         | Per-deployment parent posts and per-step markers (key: `pk`). On-demand; items expire after 3 days by TTL (`expires_at`)                                              |
| SSM parameter `/prd-amber/notification/slack-bot-token`         | Slack bot token (standard `SecureString`). Created with a dummy value; set the real one by CLI                                                                        |
| IAM role `prd-amber-notification-lambda`                        | Execution role: the table, the parameter, the log group, and sending to the dead-letter queue                                                                         |
| CloudWatch log group `/aws/lambda/prd-amber-notification`       | Logs of the function, kept for 3 days                                                                                                                                 |
| SQS queue `prd-amber-notification-dlq`                          | Keeps failed notifications for 3 days (dead-letter queue). See "Dead-letter queue" below                                                                              |
| `events:PutEvents` on the deploy role                           | `cd-backend` can send only events with the `source` and `detail-type` above                                                                                           |

- The ECS rules are split by `eventName` because `eventName` is a fixed value that is safe to filter on
- Lambda targets do not use a role. The permission to invoke is granted by the function's resource-based policy
- Terraform writes the parameter with the write-only `value_wo`, so the real token put by the CLI never reaches the state
- Terraform creates the function with placeholder code that does not run and ignores code changes. The code is deployed by `cd-notifier`
- The function timeout (30 seconds) is shorter than the step lease (60 seconds in `backend/internal/dynamodb`). Running longer would let a retried invocation work on the same step in parallel
- None of these cost anything while idle (billed per invocation, request, or duration). So they are not removed by `enable_runtime` and always exist

## Slack app scopes

The bot token needs the following scopes. After adding a scope, reinstall the Slack app to the workspace (the token does not get the new scope until then).

| Scope             | API                                                                 |
| ----------------- | ------------------------------------------------------------------- |
| `chat:write`      | `chat.postMessage` (parent and replies), `chat.update` (the parent) |
| `reactions:write` | `reactions.add` (reactions on completion and failure)               |

Invite the bot to the channel `#amber-prd-deploy`.

## Set the bot token

Terraform creates the parameter with a dummy token, so that the real token is not stored in the state.
After Terraform creates the parameter, the user sets the real token with the AWS CLI (an operation that changes AWS). Terraform does not revert the token to the dummy (it writes only when `value_wo_version` is raised).

How to create the Slack app and save the token is described in the Notion implementation task "Slack への通知を作る". The token is assumed to be saved in `tmp/slack-app-settings/bot-user-oauth-token` (ignored by Git).

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

- The token is set when `Version` in the output has gone up by one
- The function reads the token only once, at a cold start. Right after a change, running execution environments keep using the old token. To switch immediately, run `cd-notifier` again (redeploying the code recreates the execution environments)
- If reinstalling the app does not change the token value, there is nothing to set again. If the value changes, set it again with the steps above

## Dead-letter queue

Failed notifications stay in the queue `prd-amber-notification-dlq` for 3 days. They arrive by two paths, with different message shapes.

| Path                                | When                                                                                                       | Message                                                                                                                                                       |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Rule to queue                       | EventBridge could not deliver the event to the Lambda after retries (throttling, missing permission, etc.) | The body is the original event. The attributes `RULE_ARN`, `TARGET_ARN`, `ERROR_CODE`, and `ERROR_MESSAGE` carry the reason                                   |
| Lambda failure destination to queue | The function returned an error and still failed after 2 retries (Slack returned `ok: false` or 5xx, etc.)  | The body is an invocation record: `requestContext.condition` (`RetriesExhausted`, etc.), `requestPayload` (the original event), `responsePayload` (the error) |

- EventBridge counts the delivery as successful once Lambda accepts the asynchronous invocation. So function errors do not go through the EventBridge retries; the retries and the failure destination on the Lambda side handle them
- Slack returns many errors (`invalid_auth`, `not_in_channel`, `missing_scope`, etc.) as HTTP 200 with `ok: false`. The Lambda returns `ok: false` as an error, so it ends up in the queue after the retries
- The queue policy allows sends only from the notification rules. Sends from the Lambda get through with the permission on its execution role (same account)

### Inspect it

Count the messages.

```sh
aws sqs get-queue-attributes \
  --queue-url "$(terraform -chdir=infra/envs/prd output -raw notification_dead_letter_queue_url)" \
  --attribute-names ApproximateNumberOfMessages \
  --profile tamaco489 --region ap-northeast-1 \
  | jq .Attributes
```

Read the messages. Received messages are hidden only for the visibility timeout (they are not deleted).

```sh
aws sqs receive-message \
  --queue-url "$(terraform -chdir=infra/envs/prd output -raw notification_dead_letter_queue_url)" \
  --message-attribute-names All \
  --max-number-of-messages 10 \
  --profile tamaco489 --region ap-northeast-1 \
  | jq '.Messages[] | {attributes: .MessageAttributes, body: (.Body | fromjson)}'
```

## When notifications do not arrive

- **Function logs:** Each event logs `handle event` and `handled event` (the parent `ts` and whether the parent was created). Failures log the error

  ```sh
  aws logs tail /aws/lambda/prd-amber-notification \
    --since 1h \
    --profile tamaco489 --region ap-northeast-1
  ```

- **Dead-letter queue:** See "Inspect it" above
- **Rule metrics:** `Invocations` and `FailedInvocations` in the CloudWatch namespace `AWS/Events` (dimension `RuleName`) show whether a rule fired and whether the event reached the Lambda
- **Function metrics:** `Errors`, `AsyncEventsDropped`, and `DestinationDeliveryFailures` in the namespace `AWS/Lambda` (dimension `FunctionName`) show function errors and whether failures reached the failure destination

## References

- [Using resource-based policies for Amazon EventBridge (AWS)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-use-resource-based.html)
- [Configuring a dead-letter queue for an EventBridge rule (AWS)](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-rule-dlq.html)
- [Handling errors for asynchronous invocations (AWS)](https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-error-handling.html)
- [Capturing records of Lambda asynchronous invocations (AWS)](https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-retain-records.html)
- [chat.postMessage (Slack)](https://docs.slack.dev/reference/methods/chat.postMessage)
- [chat.update (Slack)](https://docs.slack.dev/reference/methods/chat.update)
- [reactions.add (Slack)](https://docs.slack.dev/reference/methods/reactions.add)

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

## Mentioning the person who started the deployment

The person who started the deployment (the GitHub user in `Triggered by`) is mentioned in Slack. Only one new post per deployment is sent with the mention.

| Arrival order                       | Post with the mention                                             |
| ----------------------------------- | ----------------------------------------------------------------- |
| The started event arrives first     | At the start of the parent post (`chat.postMessage`)              |
| The in-progress event arrives first | At the start of the started reply (`Deploy started` / `Redeploy`) |

- A Slack mention must use the Slack user ID (`<@U...>`). A GitHub username does not notify anyone. So a map from GitHub usernames to Slack user IDs is kept in the SSM parameter `/prd-amber/notification/slack-user-ids` (see "Add a person to the map" below)
- When the in-progress event arrives first, the actor is unknown when the parent is created. Filling in the actor later with `chat.update` does not send a mention notification. So only in that case, the started reply carries the mention
- The other replies do not mention anyone
- The mention is followed by a line break, so the heading is on the next line (line 1 is `@user`, line 2 is a heading such as `:white_check_mark: Deployed v8: prd-amber-api`). On one line the heading is hard to read. The parent post, the started reply and the parent rewrite all use this layout
- When the parent is rewritten (`chat.update`), the mention at the start of its text is added again if the parent has the actor. `chat.update` replaces the whole message, so without this the mention would disappear. This keeps it clear later whose deployment it was
  - When the in-progress event arrives first, the parent also gets the mention when it is rewritten after the started event
  - Slack's developer docs (`chat.update`, mention formatting) do not say whether a mention added again by a rewrite sends another notification
- A person not in the map, and a deployment with no known actor (`Triggered by` is `none`), get no mention; the notifications are sent as before
- When the map cannot be read (the parameter is missing, it is not JSON, or a value is not a Slack user ID), notifications are also sent without mentions. The function logs `notify without mentions`

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
| SSM parameter `/prd-amber/notification/slack-user-ids`          | Map from GitHub usernames to Slack user IDs (JSON, standard `SecureString`). Created with an empty map (`{}`); set the real one by CLI                                |
| IAM role `prd-amber-notification-lambda`                        | Execution role: the table, the parameters (two), the log group, and sending to the dead-letter queue                                                                  |
| CloudWatch log group `/aws/lambda/prd-amber-notification`       | Logs of the function, kept for 3 days                                                                                                                                 |
| SQS queue `prd-amber-notification-dlq`                          | Keeps failed notifications for 3 days (dead-letter queue). See "Dead-letter queue" below                                                                              |
| `events:PutEvents` on the deploy role                           | `cd-backend` can send only events with the `source` and `detail-type` above                                                                                           |

- The ECS rules are split by `eventName` because `eventName` is a fixed value that is safe to filter on
- Lambda targets do not use a role. The permission to invoke is granted by the function's resource-based policy
- Terraform writes the parameters with the write-only `value_wo`, so the real token and map put by the CLI never reach the state
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

## Add a person to the map

Terraform creates the parameter `/prd-amber/notification/slack-user-ids` with an empty map (`{}`), because this is a public repository and Slack user IDs must not be written in the code or tfvars.
To add a person, the user rewrites the map with the AWS CLI (an operation that changes AWS). Terraform does not revert the map to empty (it writes only when `value_wo_version` is raised).

The value is JSON with GitHub usernames as keys and Slack user IDs as values. GitHub usernames are matched case-insensitively.

```json
{ "octocat": "U0123456789" }
```

To get a Slack user ID, open the person's profile in Slack and choose "Copy member ID" from `⋮` (More). It starts with `U` or `W`.

Write the current map to `tmp/` (ignored by Git).

```sh
mkdir -p tmp/slack-app-settings

aws ssm get-parameter \
  --name /prd-amber/notification/slack-user-ids \
  --with-decryption \
  --query Parameter.Value --output text \
  --profile tamaco489 --region ap-northeast-1 \
  > tmp/slack-app-settings/slack-user-ids.json
```

Add the person and write the map back. Set `GITHUB_LOGIN` and `SLACK_USER_ID` to the person's values.

```sh
GITHUB_LOGIN=octocat
SLACK_USER_ID=U0123456789

jq -c --arg login "${GITHUB_LOGIN}" --arg id "${SLACK_USER_ID}" '. + {($login): $id}' \
  tmp/slack-app-settings/slack-user-ids.json \
  > tmp/slack-app-settings/slack-user-ids.next.json

aws ssm put-parameter \
  --name /prd-amber/notification/slack-user-ids \
  --type SecureString \
  --value "$(cat tmp/slack-app-settings/slack-user-ids.next.json)" \
  --overwrite \
  --profile tamaco489 --region ap-northeast-1 \
  | jq .

rm -f tmp/slack-app-settings/slack-user-ids.json tmp/slack-app-settings/slack-user-ids.next.json
```

- The map is set when `Version` in the output has gone up by one
- To remove a person, use `'del(.[$login])'` as the `jq` expression (`--arg id` is not needed)
- The function reads the map only once, at a cold start. To apply it immediately, run `cd-notifier` again, as with the bot token

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
- [Formatting text for app surfaces: mentioning users (Slack)](https://docs.slack.dev/messaging/formatting-message-text#mentioning-users)
- [Formatting text for app surfaces: line breaks (Slack)](https://docs.slack.dev/messaging/formatting-message-text#line-breaks)

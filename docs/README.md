# Documentation index

[English](./README.md) | [日本語](./README.ja.md)

## About this repository

A sandbox for trying out deployment strategies on ECS Fargate by actually running them.

ECS can now run blue/green (July 2025), canary and linear (October 2025) deployments without CodeDeploy (ECS native deployments).

References:

- [Amazon ECS blue/green deployments (AWS official)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-blue-green.html)
- [Amazon ECS canary deployments (AWS official)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/canary-deployment.html)
- [Amazon ECS linear deployments (AWS official)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-linear.html)
- [Amazon ECS enables built-in blue/green deployments (July 2025)](https://aws.amazon.com/about-aws/whats-new/2025/07/amazon-ecs-built-in-blue-green-deployments/)
- [Amazon ECS now supports built-in Linear and Canary deployments (October 2025)](https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-ecs-built-in-linear-canary-deployments/)

This repository runs a small Go application on Fargate and checks how traffic shifting and rollback behave for each strategy.

## Goals

- **Learn how each strategy behaves**: when, and at what ratio, traffic moves to the new revision with rolling, blue/green, canary and linear
- **Learn when rollback kicks in**: at which point the circuit breaker (tasks fail to start) and CloudWatch alarms (5xx increase) each act
- **Learn how to intervene in a deployment**: stopping at set points, adding checks and approving before moving on, with lifecycle hooks (Lambda) and PAUSE hooks
- **Measure the impact on users**: deploying under load and checking with k6 whether errors or latency appear

The findings are kept as material for choosing settings.

## Out of scope

- Running a production service. The environment is torn down after the experiments and runs only while experimenting ([Stopping and restarting the environment](./infra/runtime.md))
- Deployments with CodeDeploy
- Multiple environments (dev / stg). There is a single `prd` environment

## Tools

Tools used on local machines. Go, golangci-lint, Terraform, tflint and trivy are pinned in `.tool-versions` (asdf), which both local machines and CI read.
actionlint and shellcheck are pinned in `ci-actions.yaml`, k6 by the container image in `loadtest/Dockerfile` (read by `loadtest/justfile`), and just in CI by `ci-loadtest.yaml`. The others are not pinned; the versions shown are the ones confirmed to work.

| Tool          | Version | Used for                                                                                                                 |
| ------------- | ------- | ------------------------------------------------------------------------------------------------------------------------ |
| Go            | 1.27.4  | The test application (`/health`, `/` returning its version, and `POST /v1/carts/{cart_id}/checkout` confirming an order) |
| golangci-lint | 2.15.0  | Linting and format checks for Go                                                                                         |
| Docker        | 29.8.2  | Building `linux/arm64` images                                                                                            |
| Terraform     | 1.17.2  | Defining AWS resources                                                                                                   |
| tflint        | 0.66.0  | Linting Terraform (including the AWS ruleset)                                                                            |
| trivy         | 0.76.2  | Scanning Terraform for misconfigurations                                                                                 |
| AWS CLI       | 2.40.0  | Checking state, updating the prefix list, logging in to ECR                                                              |
| k6            | 2.4.0   | Load testing during deployments (`loadtest/Dockerfile`)                                                                  |
| just          | 1.46.0  | Task recipes (`just backend::push`, `just infra::plan`, ...)                                                             |
| actionlint    | 1.7.12  | Linting GitHub Actions workflows (pinned in `ci-actions.yaml`)                                                           |
| shellcheck    | 0.11.0  | Linting shell scripts (pinned in `ci-actions.yaml`)                                                                      |
| asdf          | 0.18.0  | Managing tool versions (`.tool-versions`)                                                                                |

## Documents

| Document                                                      | What it covers                                                                                                                                                               |
| ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Stopping and restarting the environment](./infra/runtime.md) | Creating and destroying the billed resources with `enable_runtime`, and what always stays                                                                                    |
| [Lifecycle of the tfstate bucket](./infra/tfstate.md)         | The lifecycle rule that keeps up to 10 old versions in the tfstate bucket, and how to put it in place and check it                                                           |
| [Deployment notifications to Slack](./infra/notification.md)  | How deployment start, progress, completion and failure are posted to Slack, the mention of the person who started it, and how to set the bot token and the Slack user ID map |
| [Applying task definition changes](./cd/task-definition.md)   | Getting task definition changes made in Terraform onto the service with `cd-backend`                                                                                         |
| [Deploying the notification Lambda](./cd/notifier.md)         | How `cd-notifier` puts the code of the notification Lambda on a push to `main`, and how to redeploy it                                                                       |
| [Checking workflows and scripts](./ci/workflow-lint.md)       | What `ci-actions` checks with actionlint and shellcheck, and how to run it locally                                                                                           |
| [Pinning actions and images](./ci/pinning.md)                 | Why workflow actions and container images are pinned by commit SHA and digest, how to write them, taking updates through Dependabot PRs, and what pinning does not cover     |
| [Checking the k6 scripts](./ci/loadtest.md)                   | What `ci-loadtest` checks with `k6 inspect` and a short load test against the application inside the runner, and how to run it locally                                       |
| [Load testing during deployments (k6)](./loadtest/k6.md)      | Running k6 during a deployment and counting failures per kind and responses per version                                                                                      |
| [Ruleset for v\* tags](./github/tag-ruleset.md)               | Limiting who can create, update and delete `v*` tags (the deployment trigger) to admins                                                                                      |

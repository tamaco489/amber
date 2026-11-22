# Applying task definition changes

[English](./task-definition.md) | [日本語](./task-definition.ja.md)

Back to the [documentation index](../README.md).

How to get a task definition change made in Terraform (CPU, memory, environment variables, logging, ...) onto the service.

## How it works

- The ECS service is set to `ignore_changes = [task_definition]` in Terraform. Applying creates a new task definition revision, but **the service does not switch to it.** The plan shows no diff either
- `cd-backend` copies the latest ACTIVE revision of the family (`prd-amber-api`), replaces only the image, and registers a new revision. So Terraform changes reach the service on the next deployment
- Revisions created by Terraform use the `bootstrap` image. Deploying such a revision directly puts the application back to `bootstrap`. The image must always be replaced by `cd-backend`

## Steps

1. Change the task definition in Terraform and apply it (`just infra::apply prd`). A new revision (`:N`) is added to the family
2. Find the tag that is running now (the version returned by `https://ecs-lab.tamaco-blog.jp/`, or the job summary of the last `cd-backend` run)
3. Open `cd-backend` in GitHub Actions, choose that tag as the ref in `Run workflow`, and run it (`workflow_dispatch`)
4. Check that the `copied from` line in the log shows `:N`, and that the task definition in the job summary has a number newer than `:N`

Pushing a new tag to deploy also picks up the Terraform changes in the same way.

> [!NOTE]
> When the same tag is run again, the image already in ECR is used and nothing is built.

# Deploying the notification Lambda

[English](./notifier.md) | [日本語](./notifier.ja.md)

Back to the [documentation index](../README.md).

The `cd-notifier` workflow puts the code of the deployment notification Lambda (`backend/cmd/notifier`) on AWS.
Terraform (`infra/modules/notification/lambda.tf`) creates the function itself, and the workflow only replaces its code.

## How it works

- Terraform creates the function `prd-amber-notification` with placeholder code that does not run (a zip with only `bootstrap`). It ignores code changes (`filename`, `source_code_hash`), so applying does not revert the code the workflow deployed
- Terraform manages the runtime, memory, timeout, environment variables, role and log destination. Change them with a Terraform apply (no need to rerun the workflow)
- The function costs nothing while idle (billed per request and duration), so it is not removed by `enable_runtime` and always exists

| Item        | Value                                                                             |
| ----------- | --------------------------------------------------------------------------------- |
| Function    | `prd-amber-notification` (`provided.al2023`, `arm64`, 128 MB, 30 seconds)         |
| Role        | `prd-amber-github-actions-notifier`                                               |
| Trust       | Only OIDC tokens issued for the `main` branch of this repository                  |
| Permissions | `lambda:UpdateFunctionCode` and `lambda:GetFunctionConfiguration` on the function |

## When it runs

- On a push to `main` that changes any of these
  - `backend/cmd/notifier/**`, `backend/internal/**`, `backend/go.mod`, `backend/go.sum`
  - Changes only under `backend/internal/server/**`, `backend/internal/health/**`, `backend/internal/checkout/**`, or `backend/internal/failmode/**` do not trigger it. Only the test application imports them, so the notifier code does not change
  - `.github/workflows/cd-notifier.yaml`, `.github/workflows/scripts/cd-notifier/**`
- When run by hand with `workflow_dispatch`. Choose `main` as the ref. The role trusts only tokens for `main`, so choosing another branch or a tag fails at AWS authentication
- Runs go one at a time (`concurrency: cd-notifier`). A running one is not cancelled

## Steps (inside the workflow)

1. Build `bootstrap` for `linux/arm64` with the Go version in `backend/go.mod` and zip it (`scripts/cd-notifier/build.sh`). The build matches `just backend::build-notifier`
2. Assume the role `prd-amber-github-actions-notifier`. Its ARN is kept in the repository variable `AWS_NOTIFIER_DEPLOY_ROLE_ARN`
3. Replace the code with `aws lambda update-function-code`, and wait with `aws lambda wait function-updated` until `LastUpdateStatus` is `Successful` (`scripts/cd-notifier/update-function-code.sh`)
4. Write the function, commit and code hash (`CodeSha256`) to the job summary

## First deployment

1. The user applies with Terraform (`just infra::apply prd`). The function and the role are created. At this point the function has the placeholder code and does not run
2. Register the role ARN as a repository variable

   ```sh
   gh variable set AWS_NOTIFIER_DEPLOY_ROLE_ARN \
     --body "$(terraform -chdir=infra/envs/prd output -raw notifier_deploy_role_arn)"
   ```

3. Open `cd-notifier` in GitHub Actions, choose `main` as the ref in `Run workflow`, and run it

## Redeploying

To put the code again without changing it, also run `cd-notifier` on `main` with `workflow_dispatch`. Check the deployed code with this command.

```sh
aws lambda get-function-configuration \
  --function-name prd-amber-notification \
  --profile tamaco489 --region ap-northeast-1 \
  | jq '{State, LastUpdateStatus, CodeSha256, LastModified}'
```

If `CodeSha256` matches the value in the job summary, that code is deployed.

## References

- [Deploy Go Lambda functions with .zip file archives (AWS)](https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html)
- [aws lambda wait function-updated (AWS)](https://docs.aws.amazon.com/cli/latest/reference/lambda/wait/function-updated.html)
- [Fine-tuning the Resources and Conditions sections of policies (AWS)](https://docs.aws.amazon.com/lambda/latest/dg/lambda-api-permissions-ref.html)

# Stopping and restarting the environment

[English](./runtime.md) | [日本語](./runtime.ja.md)

Back to the [documentation index](../README.md).

This environment exists only to try out deployment strategies. It does not need to stay up, so **the hourly billed resources are created only while experimenting and destroyed afterwards.**
A single variable, `enable_runtime` in `infra/envs/prd/terraform.tfvars`, switches them on and off.

## Workflow

```sh
# 1. Start: set enable_runtime = true and apply
just infra::apply prd

# 2. Run the experiments

# 3. Finish: set enable_runtime = false and apply
just infra::apply prd
```

**Set it back to `false` on the same day.** Leaving it running costs about \$0.14 per hour (\$3.4 per day).

With `false` committed, the `ci-infra` plan shows `No changes`. With `true`, every PR shows 37 resources to add and real changes get buried.

## What the switch creates and destroys

| Module    | Resources                                                                    | Cost (Tokyo)                       |
| --------- | ---------------------------------------------------------------------------- | ---------------------------------- |
| `alb`     | ALB, listeners, target groups, security group                                | about $0.034 per hour (incl. IPv4) |
| `dns`     | ACM certificate, Route 53 record (`ecs-lab.tamaco-blog.jp`)                  | free                               |
| `network` | Interface endpoints (`ecr.api`, `ecr.dkr`, `logs`) in two AZs                | about $0.084 per hour              |
| `ecs`     | Cluster, service, task definition, IAM roles, auto scaling, alarm, log group | about $0.025 per hour for 2 tasks  |

The total is about $0.14 per hour.

ECS refers to the ALB rules and target groups, and pulls images and sends logs through the endpoints. The three are only useful together, so a single switch covers them all.

## What stays regardless of the switch

| Resource                                                      | Cost                                                  |
| ------------------------------------------------------------- | ----------------------------------------------------- |
| VPC, subnets, IGW, route tables, security groups              | free                                                  |
| S3 gateway endpoint                                           | free                                                  |
| ECR repository (`prd-amber-api`)                              | $0.10 per GB-month; images are a few MB, so almost $0 |
| IAM role for plan (`prd-amber-github-actions-plan`)           | free                                                  |
| Prefix list for the test listener (managed outside Terraform) | free                                                  |
| tfstate bucket (`prd-amber-tfstate`)                          | under 1 MB, so almost $0                              |

While stopped, the cost is almost $0 (under one cent per month).

## What changes when recreated

Every switch creates the resources anew.

- **Changes**: ARNs, IDs, the ALB DNS name, the certificate, the target groups
- **Stays the same**: the domain (`https://ecs-lab.tamaco-blog.jp`), resource names, images in ECR

The ECS service starts with the image tagged `bootstrap` in ECR (Terraform looks up its digest when planning). Even if CD has deployed another version, recreating brings it back to `bootstrap`. To keep verifying a version, deploy it again with CD after recreating (`workflow_dispatch` with the tag selected).

The ECR lifecycle policy never expires the `bootstrap` image (it only expires `v*` tags). If it were gone, the plan with `enable_runtime = true` would fail. Push it again with `just backend::push bootstrap`.

## Adding new resources

Put resources billed by the hour, or billed just for existing, in a module that `enable_runtime` destroys.

- Billed by the hour: ALB, NAT gateway, interface endpoints, Fargate tasks
- Billed for existing: CloudWatch alarms ($0.10 per alarm-month)
- Not yet confirmed: EventBridge connections (creating one also creates a Secrets Manager secret)

## References

- [Elastic Load Balancing pricing](https://aws.amazon.com/elasticloadbalancing/pricing/)
- [AWS PrivateLink pricing](https://aws.amazon.com/privatelink/pricing/)
- [AWS Fargate pricing](https://aws.amazon.com/fargate/pricing/)
- [Amazon VPC pricing (public IPv4 addresses)](https://aws.amazon.com/vpc/pricing/)
- [Amazon CloudWatch pricing](https://aws.amazon.com/cloudwatch/pricing/)

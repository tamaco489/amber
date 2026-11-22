# Ruleset for v* tags

[English](./tag-ruleset.md) | [日本語](./tag-ruleset.ja.md)

Back to the [documentation index](../README.md).

`cd-backend` deploys to production when a `v*` tag is pushed. The trust policy of the deploy role only accepts tokens for `refs/tags/v*`, but anyone who can push tags can put a `v*` tag on any commit and deploy it.
A GitHub ruleset therefore limits who can create, update and delete `v*` tags to repository admins. Deployments are guarded at two levels: AWS (the trust policy) and GitHub (the ruleset).

## Settings

The definition lives in [`.github/rulesets/tag-v.json`](../../.github/rulesets/tag-v.json) and is the source of truth. If you change the ruleset in the web UI, update the JSON too.

| Item        | Value                                                                      |
| ----------- | -------------------------------------------------------------------------- |
| Name        | `tag-v`                                                                    |
| Target      | Tags (`target: tag`), `refs/tags/v*`                                       |
| Rules       | `creation` / `update` / `deletion` (restrict creating, updating, deleting) |
| Enforcement | `active`                                                                   |
| Bypass      | Repository admins only (`RepositoryRole`, `actor_id: 5`, `always`)         |

- For `RepositoryRole`, `actor_id` is `2` for `maintain`, `4` for `write` and `5` for `admin`. The GitHub REST API docs do not list these values; they come from the [Terraform GitHub provider docs](https://registry.terraform.io/providers/integrations/github/latest/docs/resources/repository_ruleset). After creating the ruleset, check in the web UI that the bypass list shows "Repository admin"
- The rules apply on push. Existing `v*` tags and redeployments through `workflow_dispatch` with a tag selected are not affected

API reference: [Rules for a repository (GitHub official)](https://docs.github.com/en/rest/repos/rules)

## Creating and updating

A repository admin runs these locally.

```sh
# Create
gh api -X POST repos/tamaco489/amber/rulesets --input .github/rulesets/tag-v.json

# Update (after changing the JSON)
id=$(gh api repos/tamaco489/amber/rulesets --jq '.[] | select(.name == "tag-v") | .id')
gh api -X PUT "repos/tamaco489/amber/rulesets/${id}" --input .github/rulesets/tag-v.json
```

## Checking

### 1. The ruleset is active

```sh
id=$(gh api repos/tamaco489/amber/rulesets --jq '.[] | select(.name == "tag-v") | .id')
gh api "repos/tamaco489/amber/rulesets/${id}" \
  --jq '{name, target, enforcement, conditions, rules, bypass_actors, current_user_can_bypass}'
```

Expected: `enforcement` is `active`, `bypass_actors` contains only `RepositoryRole` with `actor_id: 5`, and `current_user_can_bypass` is `always` when run as an admin.

### 2. Admins can push and cd-backend works as before

At the next deployment, an admin pushes the tag as usual.

```sh
git tag v<next number>
git push origin v<next number>
gh run list --workflow cd-backend.yaml --limit 1
```

Expected: the push is accepted, and a `cd-backend` run starts and succeeds.

### 3. Non-admins are blocked

When a non-admin pushes a `v*` tag, it is rejected with `GH013: Repository rule violations found`.
The owner is the only collaborator, so check it in one of these ways.

- **With another account (recommended)**: invite it with the `write` role, run `git push origin v0-ruleset-test` from that account and see it rejected. Remove the invitation afterwards
- **By removing the bypass for a moment**: `PUT` the JSON with `bypass_actors` set to `[]`, run `git push origin v0-ruleset-test` as the owner and see it rejected. `PUT` the original JSON again afterwards

> [!IMPORTANT]
> If the push goes through, `cd-backend` runs and starts a deployment. Check only while `enable_runtime = false` (there is no ECS, so the deployment fails partway). If the push goes through, stop the run with `gh run cancel` and delete the tag with `git push origin :refs/tags/v0-ruleset-test`.

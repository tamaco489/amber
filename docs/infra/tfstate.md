# Lifecycle of the tfstate bucket

[English](./tfstate.md) | [日本語](./tfstate.ja.md)

Back to the [documentation index](../README.md).

The Terraform state lives in the S3 bucket `prd-amber-tfstate`. Versioning is enabled on the bucket, so every apply leaves an old version behind. The `.tflock` lock file also leaves a delete marker and an old version each time the lock is released.
A lifecycle configuration therefore keeps only the 10 newest old versions and deletes the rest. Keeping 10 makes it possible to go back a few applies if the state gets broken.

The bucket cannot be managed by the state it holds, so it was created with the AWS CLI outside Terraform (`infra/envs/prd/backend.tf`). The lifecycle configuration is also put in place by the user with the CLI.

## Settings

The definition lives in [`infra/tfstate/lifecycle.json`](../../infra/tfstate/lifecycle.json) and is the source of truth. If you change it in the console or with the CLI, update the JSON too.

| Item                             | Value                                              | Meaning                                                   |
| -------------------------------- | -------------------------------------------------- | --------------------------------------------------------- |
| Scope                            | `Filter: {}`                                       | The whole bucket (the state and `.tflock`)                |
| `NoncurrentVersionExpiration`    | `NewerNoncurrentVersions: 10`, `NoncurrentDays: 1` | Keep the 10 newest old versions and delete the older ones |
| `Expiration`                     | `ExpiredObjectDeleteMarker: true`                  | Delete delete markers that have no old versions left      |
| `AbortIncompleteMultipartUpload` | `DaysAfterInitiation: 1`                           | Abort incomplete multipart uploads after 1 day            |

- The current version (the latest state) is never deleted. `Expiration` has no days set and only cleans up delete markers
- An old version is deleted once it exceeds both `NoncurrentDays` (days since it became old) and `NewerNoncurrentVersions` (the number of newer old versions). `NoncurrentDays` cannot be omitted, so it is set to the minimum, `1`. How long deletion takes is described in "[2. Old versions are deleted as the lifecycle says](#2-old-versions-are-deleted-as-the-lifecycle-says)"
- `NewerNoncurrentVersions` requires a `Filter`. An empty `Filter` covers the whole bucket
- All four actions fit in one rule (`ExpiredObjectDeleteMarker` and `AbortIncompleteMultipartUpload` only cannot be combined with a tag `Filter`)
- For `.tflock`, the current version is a delete marker with 10 old versions (lock files and delete markers) under it. While old versions remain, the current delete marker is not removed

### References (Settings)

- DOC: <https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html>

## Putting it in place and updating it

The user runs this locally. It replaces the whole current configuration.

```sh
aws s3api put-bucket-lifecycle-configuration \
  --bucket prd-amber-tfstate \
  --lifecycle-configuration file://infra/tfstate/lifecycle.json \
  --profile tamaco489 --region ap-northeast-1
```

## Checking

### 1. The configuration is in place

```sh
aws s3api get-bucket-lifecycle-configuration \
  --bucket prd-amber-tfstate \
  --profile tamaco489 --region ap-northeast-1
```

It should return the same rule as the JSON. Before it is put in place, this fails with `NoSuchLifecycleConfiguration`.

### 2. Old versions are deleted as the lifecycle says

An old version is deleted only when it meets **both** of the following conditions.

| Condition                     | When it is met                                                                                                               |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `NoncurrentDays: 1`           | The time it became old (when the next newer version was created) plus 1 day, rounded up to the next midnight UTC (09:00 JST) |
| `NewerNoncurrentVersions: 10` | The time 10 newer old versions exist                                                                                         |

- Lifecycle runs once a day, asynchronously. There is no guarantee of when a version is actually deleted after it meets the conditions
- So right after a day with many applies, having more than 10 old versions is normal. For example, a version that became old at 2026-11-28 17:00 (JST) meets the days condition at 2026-11-30 09:00 (JST) and is deleted by a lifecycle run after that
- The AWS documentation does not confirm whether delete markers count toward `NewerNoncurrentVersions`. `/cost-check` counts delete markers as old versions too
- `/cost-check` does not judge by the number of old versions. It reports NG only when a version is still there more than 3 days after it met both conditions (see "tfstate の古い版の判定" in `.claude/skills/cost-check/SKILL.md`)

Check the number of old versions with this command.

```sh
aws s3api list-object-versions \
  --bucket prd-amber-tfstate \
  --profile tamaco489 --region ap-northeast-1 --output json \
  | jq '[(.Versions // [])[], (.DeleteMarkers // [])[]]
      | group_by(.Key)
      | map({key: .[0].Key, current: map(select(.IsLatest)) | length, noncurrent: map(select(.IsLatest | not)) | length})'
```

It is as expected if `noncurrent` is 10 or less for every key after about 3 days without an apply. Before the change (2026-11-22), the state had 34 old versions and `.tflock` had 123 (62 lock files and 61 delete markers).

### References (2. Old versions are deleted as the lifecycle says)

- DOC: <https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html>
- DOC: <https://repost.aws/knowledge-center/s3-lifecycle-rule-delay>

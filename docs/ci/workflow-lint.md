# Checking workflows and scripts

[English](./workflow-lint.md) | [日本語](./workflow-lint.ja.md)

Back to the [documentation index](../README.md).

`ci-actions` checks the GitHub Actions workflows and the shell scripts they call.

## What is checked

| Target                                         | Tool       | What it looks at                                                |
| ---------------------------------------------- | ---------- | --------------------------------------------------------------- |
| `.github/workflows/*.yaml`                     | actionlint | Syntax, `${{ }}` expressions, action inputs, runner labels, ... |
| Shell inside `run:` in workflows               | shellcheck | Missing quotes, undefined variables, ... (called by actionlint) |
| `.sh` files under `.github/workflows/scripts/` | shellcheck | Same as above (checked directly)                                |

- A single finding fails the check, including shellcheck `info`
- It runs on PRs that change files under `.github/workflows/`, and on pushes to `main`
- Versions are pinned in the `env` of `ci-actions.yaml` (actionlint 1.7.12, shellcheck 0.11.0). Downloads are verified with checksums. When bumping a version, update the checksum too

## Running locally

Install actionlint and shellcheck (for example `brew install actionlint shellcheck`) and run from the repository root.

```sh
just actions::check       # runs the two below
just actions::lint        # actionlint only
just actions::shellcheck  # .sh files under scripts/ only
```

## Suppressing false positives

When intended code triggers a finding, put `# shellcheck disable=<code>` on the line before it, with a comment explaining why.

Example: the "Update service" step in `cd-backend.yaml` wraps `--query` in single quotes on purpose, so that the JMESPath literal `` `PRIMARY` `` is not expanded. SC2016 is suppressed there.

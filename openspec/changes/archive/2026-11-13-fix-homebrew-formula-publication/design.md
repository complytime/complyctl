# Design

## Context

The Homebrew release job configures Git to use `gh` as its credential helper,
then pushes a formula branch. `GH_TOKEN` is scoped to the helper-setup command,
so the helper cannot read it when the later `git push` runs.

## Goals / Non-Goals

**Goals:**

- Make the existing App token available while Git pushes the formula branch.
- Keep the current branch-and-PR publication flow and App permissions intact.
- Add a regression check for the credential-helper environment.

**Non-Goals:**

- Change formula generation, provenance, or Homebrew installation behavior.
- Change release branch naming, pull-request handling, or token permissions.
- Redefine the existing GitHub Actions runner trust boundary for Git hooks or
  credential helpers; existing Homebrew publication commands use the same
  command-scoped App token pattern.

## Decisions

Run `git push` with `GH_TOKEN` set to the existing `TAP_TOKEN`. Git invokes
the configured `gh` credential helper as a child process, so it receives the
token without placing credentials in the remote URL or Git configuration.

The token MUST be scoped to the push command environment rather than exported
for the job or step. The workflow MUST NOT enable shell tracing around the
credential-bearing command or write the token to logs, diagnostics, files, or
the regression fixture. Regression coverage MUST use a synthetic test value
and verify availability to the credential helper without printing or persisting
that value. The workflow retains its existing trusted-runner assumption for
Git children, matching its other `GH_TOKEN="${TAP_TOKEN}"` invocations.

Using an authenticated remote URL is rejected because it can expose the token
through configuration or diagnostics. Exporting a persistent credential is
rejected because the scoped command environment is sufficient.

## Risks / Trade-offs

- The regression check must observe the helper environment without logging the
  token value → use a controlled test token and assert only its availability.

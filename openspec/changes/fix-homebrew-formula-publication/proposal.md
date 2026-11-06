# Proposal

## Why

Homebrew formula publication fails when Git invokes the `gh` credential helper
without the GitHub App token. The configured `GH_TOKEN` applies only to
`gh auth setup-git`, not the later `git push` command.

## What Changes

- Pass the existing scoped App token to the Git push that publishes the
  existing formula branch.
- Add regression coverage that proves push authentication receives the token.

## Capabilities

No externally visible behavior changes. This is a workflow repair that keeps
the existing formula pull-request publication flow intact.

## Impact

- `.github/workflows/release.yml` Homebrew publishing job
- Focused release-workflow regression coverage

# Contributing to Complyctl

Thank you for your interest in contributing to complyctl! We welcome contributions from the community.

## How to Contribute

1. **Fork the repository** and clone it to your local machine.
2. **Create a new branch** for your feature or bugfix.
3. **Make your changes** with clear and concise commit messages.
4. **Push your changes** to your forked repository.
5. **Open a pull request** with a detailed description of your changes.

Before you start, please make sure to check our [STYLE_GUIDE.md](STYLE_GUIDE.md) to follow the project's coding standards.

## Pull Request Guidelines

All pull requests (PRs) must meet the following requirements before being merged:

1. Follow the project's coding standards outlined in [`STYLE_GUIDE.md`](STYLE_GUIDE.md).
2. Ensure all required tests pass.
3. Receive **at least two approvals** from project maintainers or reviewers.

## Provider API Compatibility

Changes to provider-facing APIs must preserve compatibility with the latest
stable release in the current major version. Run the narrow target for the
surface you changed, or run both checks before submitting a change that touches
both contracts:

```bash
make api-compatibility-proto
make api-compatibility-go
make api-compatibility
```

The protobuf target compares `api/plugin` with Buf. The Go target compares the
exported `pkg/provider` package with `apidiff`. The combined target uses one
baseline for both checks. Generated protobuf Go code is covered by the protobuf
comparison and is not a separate Go API surface.

The checks select the highest stable semantic-version tag in the candidate
major version, ignore prerelease tags, and report the selected tag and commit.
Set `API_COMPATIBILITY_CANDIDATE_VERSION` when the nearest Git tag cannot
identify the candidate major version. For example:

```bash
API_COMPATIBILITY_CANDIDATE_VERSION=v1.2.0 make api-compatibility
```

These commands fail closed. An unavailable baseline, missing or wrong-version
tool, package-loading error, or analyzer error fails the command instead of
being treated as a compatible API. Install stable Buf `1.50.0` or newer and the pinned
`golang.org/x/exp` `apidiff` tool (`v0.0.0-20251023183803-a4bb9ffd2546`)
before running the relevant target.

Local Buf versions are compared numerically after semantic-version validation;
prerelease and malformed versions are rejected. For an older version, the error
reports `buf >= 1.50.0 required, found <version>`. Valid build metadata does not
affect version precedence. CI still installs exactly Buf `1.50.0`, so use that
version to reproduce CI's analyzer behavior. A passing local run with a newer
Buf does not replace the required CI result. The `apidiff` revision remains an
exact requirement locally and in CI.

By default, release tags come from `upstream`, or from `origin` if only that
remote points to `github.com/complytime/complyctl`. The checks accept HTTPS and
SSH URLs and reject automatic selection of a fork. For a fork-based checkout,
add the canonical remote before running the checks:

```bash
git remote add upstream https://github.com/complytime/complyctl.git
```

Set `API_COMPATIBILITY_REMOTE` to explicitly trust another configured remote
for release tags. This selects the baseline source; it does not disable the
compatibility checks.

Local CI runs the `api-compatibility` job for pull requests and pushes to
`main`, the supported release line. The job uses SHA-pinned checkout, Buf, and
Go setup actions, Buf `1.50.0`, and the pinned `apidiff` version above. It
publishes the selected baseline tag and commit plus the complete analysis log
to the GitHub Actions job summary, including diagnostics from a failed run.

For a provider API change, a successful `api-compatibility` check is required
merge evidence. After pushing the workflow change, maintainers must confirm
that GitHub reports the check as `api-compatibility` and that a failed check
blocks release preflight before relying on the check-name configuration.

### Static-analysis limits and downstream validation

Static analysis cannot prove all Go source or runtime compatibility. `apidiff`
detects exported API changes that affect struct comparability, but it cannot
detect downstream use of unkeyed composite literals. It also does not verify
provider builds, protocol interoperability, or runtime behavior.

Keep provider-side build and integration validation with the downstream work
in [complytime-providers#187](https://github.com/complytime/complytime-providers/issues/187).
The local checks complement that validation; they do not replace it.

### Future major-version breaking changes

The current policy has no breaking-change override: an incompatible provider
API fails on `main`, the current supported release line. When a future major
migration is approved, maintainers must design and review an authorization
workflow before enabling it. That workflow must retain analyzer diagnostics,
bind authorization to the reviewed commit, require documented consumer impact
and migration steps, and keep operational analyzer failures fail-closed.

## Code of Conduct

Please adhere to our [Code of Conduct](CODE_OF_CONDUCT.md) in all your interactions with the project.

## Reporting Issues

If you encounter any issues, please [open an issue](https://github.com/complytime/complyctl/issues) on GitHub.

Thank you for contributing!

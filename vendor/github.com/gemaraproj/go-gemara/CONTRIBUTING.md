# Contributing to go-gemara

Thanks for your interest in contributing! Whether it's a bug report, feature request, or code change, we appreciate your help.

## Code of Conduct

This project follows the [OpenSSF Code of Conduct](https://openssf.org/community/code-of-conduct/).

## Getting Started

1. [Fork the repository](https://github.com/gemaraproj/go-gemara/fork)
2. Clone your fork and create a branch
3. Make your changes
4. Open a pull request

## Development

### Prerequisites

- Go 1.25+
- [golangci-lint](https://golangci-lint.run/)
- [CUE CLI](https://cuelang.org/) (for type generation)

### Common Commands

```bash
make test          # run tests
make testcov       # run tests with coverage
make lint          # run golangci-lint
make build         # build CLI binaries
make generate      # regenerate Go types from Gemara CUE schemas
make ci-local      # run the same checks CI runs
```

### Updating to a New Gemara Spec Version

When a new version of the [Gemara CUE module](https://registry.cue.works/docs/github.com/gemaraproj/gemara) is released, follow these steps to update go-gemara:

1. Update `SPECVERSION` in the `Makefile` to the new version (e.g. `v1.1.0`)
2. Run `make generate` to regenerate `generated_types.go` from the new CUE schemas
3. Run `make build` to verify the project compiles with the new types
4. Run `make test` to check for test failures caused by schema changes
5. Fix any compilation errors or test failures due to renamed, removed, or restructured types
6. Add or update test data files in `test-data/` if the new spec version introduces new artifact types
7. Update OSCAL/SARIF conversion logic in `gemaraconv/` if field mappings have changed
8. Run `make ci-local` to confirm everything passes (formatting, vet, lint, tests, coverage)
9. Commit the changes with a message like `feat: update to Gemara spec vX.Y.Z`

## Code Style

- Run `goimports` (or `gofmt`) before committing -- `make fmtcheck` will catch unformatted files
- Follow the patterns already in the codebase (error handling, naming, package layout)
- Run `make lint` to check for common issues via [golangci-lint](https://golangci-lint.run/)

## Testing

- Add or update tests for any behavioral change
- Run `make testcov` to verify coverage stays above the threshold (currently 71%)
- Use table-driven tests where appropriate
- Test data files go in `test-data/` with a `good-` or `bad-` prefix

## Documentation

- Update godoc comments for any new or changed exported API
- Add or update `Example*` test functions for user-facing functionality (see `fetcher/example_test.go`)
- Update the README if your change adds new commands, flags, or usage patterns

## Pull Request Guidelines

PRs must meet the following criteria:

- **Clear title** conforming to [Conventional Commits](https://www.conventionalcommits.org/) (e.g. `feat:`, `fix:`, `docs:`, `test:`, `refactor:`)
- **DCO sign-off** via `git commit -s` ([OSPS-LE-01](https://baseline.openssf.org/#osps-le-01))
- **All CI checks pass**, including lint, tests, and the coverage threshold
- **Generated types in sync** -- run `make generate` if you've updated the Gemara spec version

## Filing Issues

Use the [issue chooser](https://github.com/gemaraproj/go-gemara/issues/new/choose) to pick the right template:

- **Bug report** -- something isn't working. Include the go-gemara version, steps to reproduce, and expected vs. actual behavior. A minimal Gemara document plus the error output is ideal for parser or conversion bugs.
- **Feature request** -- new conversions, additional Gemara artifact support, CLI improvements, or other enhancements. Describe the use case and the behavior you'd like.

For questions, the chooser also links to the OpenSSF Slack `#gemara` channel.

### Security Issues

**Do not open a public issue for security vulnerabilities.** Instead, follow the [security policy](https://github.com/gemaraproj/.github/blob/main/SECURITY.md) to report privately.

## Governance

This project is part of the [Gemara project](https://github.com/gemaraproj). See the org-level [governance](https://github.com/gemaraproj/.github/blob/main/GOVERNANCE.md) and [contributor ladder](https://github.com/gemaraproj/.github/blob/main/CONTRIBUTOR_LADDER.md) for roles and decision-making processes.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

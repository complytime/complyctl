## 1. GoReleaser Configuration

- [x] 1.1 Add `darwin` to `goos` list in `.goreleaser.yaml`
- [x] 1.2 Add explicit `goarch` entries: `amd64`, `arm64`
- [x] 1.3 Update deprecated `format: tar.gz` to `formats: [tar.gz]`
- [x] 1.4 Validate with `goreleaser check` (zero warnings)

## 2. Release Workflow — Homebrew Publishing

- [x] 2.1 Add "Generate Homebrew tap token" step using `actions/create-github-app-token` with `APP_ID_HOMEBREW_FORMULA_PUBLISHER`/`PRIVATE_KEY_APP_HOMEBREW_FORMULA_PUBLISHER` secrets, scoped to `homebrew-tap`
- [x] 2.2 Add "Publish Homebrew formula" step that computes source tarball SHA256, templates the Formula, and pushes to `complytime/homebrew-tap` repo
- [x] 2.3 Verify Formula Ruby uses `std_go_args(ldflags:)` with version, buildDate, and gitTreeState=clean (no commit)
- [x] 2.4 Verify Formula includes `generate_completions_from_executable(bin/"complyctl", "completion")`
- [x] 2.5 Verify Formula includes a `test` block asserting version output

## 3. Documentation

- [x] 3.1 Add "Homebrew (macOS / Linux)" section to `docs/INSTALLATION.md` with `brew install complytime/tap/complyctl`
- [x] 3.2 Add "go install" section to `docs/INSTALLATION.md` with the `go install` command and devel-version note
- [x] 3.3 Add CHANGELOG entries under Unreleased/Added for macOS binaries, Homebrew formula, and `go install`

## 4. Pre-merge Verification

- [x] 4.1 Run `goreleaser check` — must pass clean
- [x] 4.2 Run `goreleaser release --snapshot --clean` — verify darwin archives are produced
- [ ] 4.3 Confirm GitHub App has Contents:write on `homebrew-tap` (admin check)
- [x] 4.4 Confirm `complytime/homebrew-tap` repo has `Formula/` directory or that the workflow creates it via `mkdir -p`

## Context

complyctl is a Go CLI tool built with Cobra. The existing release pipeline uses
GoReleaser v2 (invoked via GitHub Actions) to cross-compile, archive, generate
SBOMs, and sign checksums with Sigstore/cosign. Today it only builds for
linux/amd64. The `complytime/homebrew-tap` repository exists but is empty
(contains only a LICENSE file).

Homebrew Casks (pre-built binary distribution) were ruled out because:
- Casks trigger macOS Gatekeeper quarantine on unsigned binaries.
- Apple Developer signing requires a paid account and secret management that
  is not yet in place for this project.
- The deprecated GoReleaser `brews` section (which generated "hackyish"
  formulas installing pre-built binaries) is fully deprecated in v2.16.

Cross-repo authentication uses a GitHub App (secrets: `APP_ID`, `PRIVATE_KEY`)
with `actions/create-github-app-token` to mint short-lived installation tokens.

## Goals / Non-Goals

**Goals:**
- Users can install complyctl on macOS/Linux with a single `brew install` command.
- Go developers can install with `go install` (zero infrastructure).
- The Homebrew Formula is updated automatically on each release (no manual
  Formula maintenance).
- Shell completions (Bash, Zsh, Fish) are installed by the Formula.
- No Apple Developer signing or notarization is required.
- GoReleaser config passes `goreleaser check` without deprecation warnings.

**Non-Goals:**
- Homebrew-core submission (third-party tap is sufficient for now).
- Homebrew Cask support (deferred until signing infrastructure exists).
- Windows package manager support (scoop, chocolatey).
- Migrating the release workflow to org-infra reusable workflows (separate
  concern; would force all repos into Homebrew publishing).
- Source-build verification or reproducible builds.

## Decisions

### 1. Source-build Formula (not Cask, not pre-built Formula)

**Choice**: Generate a Homebrew Formula that runs `go build` on the user's machine.

**Alternatives considered**:
| Approach | Pros | Cons |
|----------|------|------|
| `homebrew_casks` (GoReleaser) | Official GoReleaser path; fast install | Requires signing/notarization or xattr workaround |
| `brews` (GoReleaser, deprecated) | Automated; pre-built binary via Formula | Deprecated in v2.16; will be removed in v3 |
| Source-build Formula (custom step) | No signing needed; no deprecation; proper Homebrew pattern | Slower install; requires Go on user's machine (auto-installed by Homebrew) |
| No Homebrew, just `go install` | Zero maintenance | Poor UX for non-Go users; no completions; no `brew upgrade` |

**Rationale**: Source-build avoids all Gatekeeper/signing concerns permanently.
Homebrew auto-installs Go as a build dependency so users don't need it
pre-installed. The install time (~30s) is acceptable for a CLI tool.

### 2. Custom workflow step (not GoReleaser pipe)

**Choice**: A shell step in `release.yml` that templates the Formula and pushes
to the tap repo.

**Rationale**: GoReleaser's native Formula generation (`brews`) is deprecated
and produces formulas that download pre-built binaries (not source-build). A
custom step gives full control over the Ruby Formula content and avoids the
deprecation.

### 3. GitHub App token (not PAT, not GITHUB_TOKEN)

**Choice**: Use `actions/create-github-app-token` with the org's existing
`APP_ID`/`PRIVATE_KEY` secrets to push to `homebrew-tap`.

**Rationale**: Short-lived tokens with minimal scope. No long-lived PATs to
rotate. Consistent with existing cross-repo patterns in the org.

### 4. ldflags: version, buildDate, and gitTreeState

**Choice**: The Formula injects `version.version`, `version.buildDate`, and
`version.gitTreeState=clean` via ldflags. It does NOT inject `commit` because
source tarballs have no `.git` directory.

**Rationale**: Homebrew downloads source archives (not git clones). The `commit`
field is unavailable at build time and will show as empty, which the version
template handles gracefully. `gitTreeState` is hardcoded to `clean` because
source tarballs are always a clean snapshot of the tagged commit — without this,
the version output shows a trailing `+` with no state value (e.g., `1.2.3+`)
which looks broken.

### 5. macOS binaries in GoReleaser (in addition to Formula)

**Choice**: Add `darwin` to `goos` and explicit `amd64`/`arm64` to `goarch` in
the GoReleaser build matrix.

**Rationale**: Users who prefer direct binary downloads (without Homebrew or Go)
can still grab platform-specific archives from GitHub Releases. Also enables
future Cask migration if signing is adopted.

## Risks / Trade-offs

- **[Slower install]** → Source builds take ~30s vs instant for pre-built. Acceptable for a CLI tool installed once.
- **[Go dependency size]** → Homebrew downloads Go toolchain (~300MB) if not already installed. One-time cost; shared across other Go formulas.
- **[Formula fragility]** → The workflow templates Ruby via heredoc. If Homebrew API changes, the Formula may need manual updates. Mitigated by using stable Homebrew DSL (`std_go_args`, `generate_completions_from_executable`).
- **[Tag-before-tarball race]** → The Formula step curls the source tarball right after GoReleaser creates the release. GitHub may have a brief propagation delay. Mitigated by the tarball being created by `git tag` (happens in preflight job, well before the Formula step runs).
- **[GitHub App access]** → If the App's installation doesn't include `homebrew-tap`, the push will fail. Pre-merge checklist item for admins.

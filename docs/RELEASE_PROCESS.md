# Release Process for complyctl

The release process values simplicity and automation in order to provide better predictability and low cost for maintainers.

## Process Description

complyctl uses the org-wide release infrastructure from [org-infra](https://github.com/complytime/org-infra). The [release workflow](https://github.com/complytime/complyctl/blob/main/.github/workflows/release.yml) delegates to two reusable workflows:

1. **Preflight** (`reusable_release_preflight.yml`) -- validates semver format, tag uniqueness (with re-run resilience), semver ordering, CI check status, and unreleased commits before creating an annotated tag.
2. **GoReleaser** (`reusable_release_goreleaser.yml`) -- builds release artifacts via [GoReleaser](https://goreleaser.com/) with cosign signatures and SBOMs.

GoReleaser is configured in [.goreleaser.yaml](https://github.com/complytime/complyctl/blob/main/.goreleaser.yaml).

3. **Homebrew Formula Publishing** -- after the release job completes, a third job downloads the source tarball for the tag, computes its SHA256, templates a source-build Homebrew Formula, and opens a PR in `complytime/homebrew-tap`. A maintainer merges the PR to publish the formula.

### Homebrew Formula Publishing

The `homebrew` job in the release workflow opens a PR with `Formula/complyctl.rb` in the [homebrew-tap](https://github.com/complytime/homebrew-tap) repository. The PR must be merged by a maintainer to publish the formula update.

**Prerequisites:**
- The GitHub App (secrets: `APP_ID_HOMEBREW_FORMULA_PUBLISHER`, `PRIVATE_KEY_APP_HOMEBREW_FORMULA_PUBLISHER`) must have `Contents:write` and `Pull requests:write` permissions on `complytime/homebrew-tap`
- If the App installation is scoped to specific repos, `homebrew-tap` must be included

**Authentication:** The publishing step exports the generated App token as
step-scoped `GH_TOKEN`. Both GitHub CLI API commands and the credential helper
spawned by `git push` must inherit it. A token assignment on an earlier `gh`
command does not reach a later Git process. `gh auth setup-git` configures the
helper; it does not persist an environment-supplied token. Keep credentials out
of remote URLs, Git configuration, and logs. No additional secret or broader
App permission is needed.

**Failure modes:**
- Tarball download failure (HTTP != 200) -- the step exits with an error
- SHA256 length mismatch -- the step exits with an error
- GitHub App token generation failure -- the step fails visibly
- Missing publishing credential or failed push -- the step fails without
  opening a PR or reporting PR creation success
- Commit failure -- the current catch-all handler reports "No changes to
  commit" and exits 0, even for errors other than an unchanged formula.
  Confirm the formula matches tap `main` before treating this as success
- PR already exists for this version (re-run before PR was merged) -- exits 0 with a notice

**Re-runs:** An existing open formula PR produces a successful no-op with a
notice. If the PR was already merged and the formula matches main, `git commit`
exits with "No changes to commit" and the job succeeds. However, retrying an old
failed Actions run uses its original workflow revision: it does not pick up a
fix merged afterward. Future releases dispatched from a revision containing
the fix use the corrected environment.

If a prior run pushed the formula branch but did not open a PR, a retry can
fail with a non-fast-forward push. Inspect and reuse the existing branch for
the manual recovery below instead of force-pushing or rerunning the release.

**Post-release step:** After the release workflow completes, merge the formula
PR in the [homebrew-tap](https://github.com/complytime/homebrew-tap/pulls)
repository. Resolve conflicts and review feedback directly on the tap feature
branch. Do not rerun the full release to repair only its formula publication.

#### Recover an already-published release

Recover formula publication through a tap PR, without recreating the release.
The v1.1.0 and v1.2.0 runs failed because their Git credential helper could not
inherit the App token. Retrying those old runs will not load the corrected
workflow. For v1.2.0, obtain separate maintainer authorization for live tap
pushes and PR operations, then:

1. Confirm that the existing v1.2.0 release and tag are intact. Check for an
   existing `formula/complyctl-1.2.0` PR before creating another one; update an
   existing PR instead if necessary. If the branch exists without an open PR,
   inspect its formula and reuse that branch rather than recreating it.
2. Download the source archive at
   <https://github.com/complytime/complyctl/archive/refs/tags/v1.2.0.tar.gz>
   and compute its SHA256 locally. Use the same archive URL in the formula.
3. Clone `complytime/homebrew-tap` using an authorized maintainer credential or
   a fresh repository-scoped App token. For an App token, export `GH_TOKEN`
   for the publishing session and configure Git with `gh auth setup-git`.
   Never copy tokens from Actions logs or embed them in URLs or Git config.
4. On `formula/complyctl-1.2.0`, update only `url` and `sha256` in
   `Formula/complyctl.rb`, preserving the source-build formula. Commit, push
   that feature branch, and open a PR against tap `main`.
5. Review and merge the formula PR through the tap's normal protected-branch
   process, then unset the session's `GH_TOKEN` if one was exported.

Do not move or recreate the v1.2.0 tag, replace published release artifacts,
bypass branch protection, or rerun the full release for this recovery. Offline
regression tests do not prove server-side App authorization; live validation
requires separate authorization. See [Tests](#tests) for the offline check.

### Previewing Release Notes

Before creating a release, preview the generated notes for committed HEAD:

```bash
make release-notes-preview
```

The command discovers the latest published stable release in
`complytime/complyctl` and compares it with the checkout's committed HEAD.
Push the candidate commit so GitHub can compare it. Uncommitted changes are
not included. GitHub must be able to resolve that commit in the canonical
comparison; a commit available only on a fork may be unavailable. The command
uses the checkout's GoReleaser changelog settings;
it does not select a new tag, build artifacts, execute release hooks, or
publish a release. It requires a canonical Git remote (preferably named
`upstream`) even when `origin` points to a fork. A detached checkout requires
its effective default remote to point to the canonical repository.

Install Git, GitHub CLI (`gh`), Python 3.9 or newer with PyYAML, and open-source
GoReleaser v2.20.0 or newer within v2. Authenticate GitHub CLI for github.com
with `gh auth login`. Run from the repository root and keep `TMPDIR` outside
the checkout. The helper does not install dependencies or accept a baseline
override. It writes the
baseline, candidate, and diagnostics to standard error, and Markdown to
standard output, which can be redirected:

```bash
make release-notes-preview > release-notes.md
```

If HEAD is already released, the command succeeds with a "No unreleased
commits" diagnostic and no Markdown. Failed authentication, unavailable or
diverged commits, missing tools, or renderer failures return a nonzero exit;
fix the diagnostic and retry. No partial Markdown is emitted on failure.

Run the offline coordinator regression suite with Git, Make, Python 3.9 or
newer, and PyYAML:

```bash
make test-release-notes-preview
```

The tests do not require GitHub access or GoReleaser. They are local-only, not
CI-gated; run them before submitting changes to the preview helper.

### Creating a Release

To create a release, a project maintainer triggers the workflow manually:

1. Navigate to **Actions** > **Release** > **Run workflow**
2. Enter the tag (e.g., `v1.2.3`) -- must be a valid semver greater than the latest existing tag
3. The preflight job validates all conditions and creates the annotated tag automatically
4. The release job builds and publishes artifacts to the [releases page](https://github.com/complytime/complyctl/releases)
5. The homebrew job opens a formula PR in [homebrew-tap](https://github.com/complytime/homebrew-tap/pulls) -- merge it to publish the Homebrew formula

> **Note:** Tags are created automatically by the preflight job. Do not create tags manually.

### Re-runs

If the release job fails after the tag is created, re-running the workflow with the same tag is safe. The preflight detects the existing tag at HEAD and skips tag creation.

For the full release process reference, see the [org-infra release process](https://github.com/complytime/org-infra/blob/main/docs/RELEASE_PROCESS.md).

## Tests

Core release tests run in CI for every PR. The
preflight job verifies that the following CI checks have passed on HEAD before
allowing a release: `buf-lint`, `api-compatibility`, `unit-test`, `e2e-test`,
and `integration-test`.

For changes to Homebrew publishing, run the isolated offline regression test:

```bash
make test-homebrew-publish
```

It requires Bash, Python 3, Git, and GitHub CLI (`gh`), but no Homebrew
installation, production credentials, or network operations. It exercises the
workflow's actual publishing script, real Git/`gh` credential resolution,
existing-PR no-op, push failure, and the original token-inheritance regression.
It does not replace the separate `make test-homebrew` installation smoke test
or validate live App permissions. This regression check runs locally only;
it is not part of PR CI. Maintainers must run it when changing Homebrew
publishing.

### Provider API Compatibility

Before releasing a change to the provider-facing API, run the local combined
comparison from the release candidate commit:

```bash
make api-compatibility
```

The command compares the `api/plugin` protobuf contract and exported
`pkg/provider` Go API with the highest stable tag in the candidate's major
version. It ignores prerelease tags, reports the chosen tag and commit, and
fails closed if baseline resolution, tool setup, package loading, or analysis
cannot complete. Run `make api-compatibility-proto` or
`make api-compatibility-go` to diagnose an individual contract.

The `api-compatibility` CI job runs the combined command for pull requests and
pushes to `main`. It uses SHA-pinned checkout, Buf, and Go setup actions; Buf
`1.50.0`; and `apidiff`
`v0.0.0-20251023183803-a4bb9ffd2546`. The job publishes the selected baseline
and complete compatibility diagnostics to its GitHub Actions job summary, even
when analysis fails.

Local protobuf analysis accepts stable Buf versions at or above `1.50.0`;
it rejects older, malformed, and prerelease versions. CI continues installing
exactly Buf `1.50.0` for reproducible release evidence, and the `apidiff` module
revision remains exact for both local and CI execution. Use the CI versions
when reproducing a release check; newer local Buf versions can differ in
analyzer behavior and do not replace required CI evidence.

The `api-compatibility` check is required merge and release evidence for
provider-facing API changes. Release preflight lists that exact job ID in its
explicit CI checks and rejects the release when the check fails. After pushing
the workflow configuration, a maintainer must confirm GitHub's emitted check
name is `api-compatibility` and that a failed run blocks preflight; this
post-push validation confirms the configured job ID matches the GitHub check.

Static analysis does not replace provider validation. `apidiff` detects
exported API changes that affect struct comparability, but it cannot detect
downstream use of unkeyed composite literals. It also cannot verify provider
builds, protocol interoperability, or runtime behavior. Coordinate downstream
provider build and integration validation through
[complytime-providers#187](https://github.com/complytime/complytime-providers/issues/187).

### Future major-version breaking changes

The current release process has no breaking-change override. An incompatible
provider API fails the `api-compatibility` check on `main`, the current
supported release line. When maintainers approve a future major migration, they
must design and review a protected authorization workflow before enabling it.
The future workflow must preserve analyzer diagnostics, bind authorization to a
reviewed commit, document consumer impact and migration steps, coordinate
downstream provider validation, and reject operational analyzer failures.

## Cadence

Releases are currently expected every three weeks. Project maintainers always discuss and agree on releases. Therefore, some releases may be triggered a bit earlier or later when necessary.

## Fedora Packages

After the repository split, complyctl and complytime-providers are independent Fedora packages with separate release cycles.

### complyctl

Once a new complyctl release is out, the [Fedora Package](https://src.fedoraproject.org/rpms/complyctl) also needs to be updated.

The process is automated by [Packit](https://packit.dev/docs/fedora-releases-guide) according to [.packit.yaml](https://github.com/complytime/complyctl/blob/main/.packit.yaml) configuration file and should only demand a PR review from a Fedora package [maintainer](https://src.fedoraproject.org/rpms/complyctl)

This automation will create PRs for the specified branches. Once the PRs are reviewed and merged:
- [Koji builds](https://koji.fedoraproject.org/koji/packageinfo?packageID=42298) will be created
- [Bodhi updates](https://bodhi.fedoraproject.org/updates/?packages=complyctl) will be submitted

### complytime-providers

The [complytime-providers](https://github.com/complytime/complytime-providers) repository has its own independent release and packaging pipeline. It produces two sub-packages:
- `complytime-providers-openscap` -- OpenSCAP scanning provider
- `complytime-providers-ampel` -- Ampel scanning provider

The process is also automated by Packit via the `.packit.yaml` in the complytime-providers repository.

> **Note:** The complytime-providers Fedora package requires a one-time [Fedora package review](https://docs.fedoraproject.org/en-US/package-maintainers/Joining_the_Package_Maintainers/) before the automation can function. Once approved, the Packit automation operates identically to complyctl.

### Preparation (only necessary for Manual Process)

To update a Fedora package, it is ultimately necessary to be a member of Fedora Packager group.
Here is the main documentation on how to become a Fedora Packager:
- [Joining the Package Maintainers](https://docs.fedoraproject.org/en-US/package-maintainers/Joining_the_Package_Maintainers/)

However, if you are not yet a Fedora Packager, it is still possible to propose a PR.
In this case, a package maintainer will review it and help on the process.

### Requirements

#### Install the required tools

```bash
sudo dnf install fedora-packager fedora-review
```
- Ensure your system user is included in the `mock` group. This is useful when testing the package changes.
```bash
sudo usermod -a -G mock $USER
```

#### Token for authenticated commands

Make sure you have a valid kerberos token. It will be necessary for commands that require authentication:
```bash
fkinit -u <your_fas_id>
```

#### Fork the repository

Create a fork from <https://src.fedoraproject.org/rpms/complyctl>

```bash
fedpkg clone --anonymous forks/<your fedora id>/rpms/complyctl
cd complyctl
```

### Update the spec file and sources

Usually it is only necessary to update the `Version:` line and include a `%changelog` entry.

`rpmdev-bumpspec` command can be used to automate this process. e.g.:
```bash
rpmdev-bumpspec -n 0.0.9 -c "Bump to upstream version v0.0.9" complyctl.spec
```

Ensure the sources are downloaded locally:
```bash
fedpkg sources
```

To ensure the `scratch build` doesn't fail due to an "Invalid Source", ensure the new sources are uploaded to the [lookaside_cache](https://docs.fedoraproject.org/en-US/package-maintainers/Package_Maintenance_Guide/#upload_new_source_files):
```bash
fedpkg new-sources
```

### Package Tests

Check if the changes work as expected before proceeding to the next step:
```bash
fedpkg diff
fedpkg lint
fedpkg mockbuild
```
> **_NOTE:_** Alternatively one can test the package build in Koji with `fekpkg scratch-build --srpm`.

### Propose the updates

After confirming that everything is fine, create a new branch to use in the Pull Request. e.g.:
```bash
git checkout -b release-0.0.9_rawhide
git status
git add -u
git commit -s
git push -u origin release-0.0.9_rawhide
```
Continue the steps via src.fedoraproject.org web UI.

Repeat this process for all other relevant branches.

```bash
fedpkg switch-branch f42
```

### Create the new Builds

Once the PRs are merged, it is time to create the new builds.

```bash
fedpkg switch-branch rawhide
fedpkg build
```
- Follow the builds status in the following links:
  - [Builds Status](https://koji.fedoraproject.org/koji/packageinfo?packageID=42298)

### Submit Fedora updates

After the build is done, an update must be submitted to [Bodhi](https://bodhi.fedoraproject.org).

Updates for `rawhide` builds are submitted automatically, but updates for any branched version needs to be submitted manually.
```bash
fedpkg update
```
Or via web interface on [Bodhi](https://bodhi.fedoraproject.org).

The new updates enter in `testing` state and are moved to stable after 7 days, or sooner if it receives 3 positive "karmas".
After moving to `stable` state, the update is signed and awaits to be pushed to the repositories by the Release Engineering Team.

Check the package update status in the following links:
- [Updates Status](https://bodhi.fedoraproject.org/updates/?packages=complyctl)
- [Package Overview](https://src.fedoraproject.org/rpms/complyctl)

#### Troubleshooting

If tests fail due to external issues, they can be restarted once the external issues are solved.
For example, if some tests in [FEDORA-2025-2b39abfa99](https://bodhi.fedoraproject.org/updates/FEDORA-2025-2b39abfa99) failed due to infrastructure issues, they could be restarted by the following command:
```bash
bodhi updates trigger-tests FEDORA-2025-2b39abfa99
```

### More information
- [Fedora Package Guidelines](https://docs.fedoraproject.org/en-US/packaging-guidelines/)
- [Package Maintenance Guide](https://docs.fedoraproject.org/en-US/package-maintainers/Package_Maintenance_Guide)
- [Package Update Guide](https://docs.fedoraproject.org/en-US/package-maintainers/Package_Update_Guide/)
- [BZ introducing the Package](https://bugzilla.redhat.com/show_bug.cgi?id=2375155)

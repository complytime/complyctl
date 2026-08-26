## 1. Spec File Modernization

> Prerequisites: Tasks in this section all modify `complyctl.spec`.
> Task 1.1 (header) MUST be applied first since it declares Source0/1/2,
> BuildRequires, and %gometa that subsequent tasks depend on. Tasks
> 1.2-1.10 can be applied in any order after 1.1. Run verification
> (`rpmspec --parse`) after all tasks in this section are complete.

- [x] 1.1 Update header: set `Version: 1.0.0`, change
  `Release: 1%{?dist}` to `Release: %autorelease` (D8), replace
  `Source0` with `%{gosource}`, add `Source1` and `Source2` declarations,
  replace `URL` with `%{gourl}`, update `License` tag to include
  vendored dependency licenses (placeholder until task 2.4), place
  `%gometa -f` after `%global goipath` and before `Name:`, add
  `BuildRequires: go-vendor-tools`, remove `%global debug_package %{nil}`;
  verify by running `rpmspec --parse complyctl.spec`
- [x] 1.2 Replace manual `go build` in `%build` section with `%gobuild`
  macro, using `GO_LDFLAGS` env var for version ldflags (D1); set
  `%global gomodulesmode GO111MODULE=on`; verify by running
  `rpmspec --parse complyctl.spec` and confirming `%gobuild` expansion
  includes `-B 0x`, `-linkmode=external`, and `-compressdwarf=false`
- [x] 1.3 Update `%prep` section: change `%goprep -k` to `%goprep -A`,
  add `%setup -q -T -D -a1` to unpack Source1 (vendor archive), and add
  `%autopatch -p1` for patch support (D3); verify by running
  `rpmspec --parse` and confirming the prep section expands correctly
- [x] 1.4 Add `%generate_buildrequires` section with
  `%go_vendor_license_buildrequires -c %{S:2}`; verify present in
  parsed spec output
- [x] 1.5 Update `%install` section: add `%go_vendor_license_install
  -c %{S:2}` and change binary install path from `bin/complyctl` to
  `%{gobuilddir}/bin/complyctl`; verify by inspecting parsed spec
- [x] 1.6 Update `%check` section: add `%bcond check 1` at the top of
  the spec, replace manual `go test` with `%go_vendor_license_check
  -c %{S:2}` and `%gocheck2` with exclusions for `tests/`,
  `cmd/test-provider/`, `cmd/mock-oci-registry/`, and
  `cmd/behavioral-report/` (D4), wrapped in `%if %{with check}`;
  verify by inspecting parsed spec
- [x] 1.7 Update `%files` section: use `-f %{go_vendor_license_filelist}`
  on the `%files` directive, keep `%license LICENSE`, update `%doc` to
  `README.md`; remove `vendor/modules.txt` from `%license` (handled by
  `%go_vendor_license_install`); verify by inspecting parsed spec
- [x] 1.8 Retain Fedora 43 Go version compatibility hack (D7): keep the
  `%if 0%{?fedora} == 43` conditional sed block in `%prep` after the
  setup macros; generalize the `vendor/modules.txt` sed to match any
  Go version above 1.25 (not just `1.26.*`); verify the sed commands
  are present in the parsed spec output and the regex patterns match
  the current `go.mod` toolchain directive (`go 1.26.5`)
- [x] 1.9 Remove CentOS Stream targets from `.packit.yaml` (D9): remove
  `centos-stream-9-x86_64` and `centos-stream-10-x86_64` from
  `copr_build` and `tests` target lists; add a YAML comment noting
  these targets require go-vendor-tools availability on CentOS Stream;
  verify by inspecting the updated `.packit.yaml`
- [x] 1.10 Add changelog entry for v1.0.0 spec modernization; verify
  by inspecting the `%changelog` section

## 2. Go Vendor Tools Configuration

> Prerequisites: `go-vendor-tools` and `golang` must be installed
> locally (`dnf install go-vendor-tools golang`). Section 1 must be
> completed first since these tasks use the updated spec file.

- [x] 2.1 Generate initial vendor archive: run `go_vendor_archive create
  --config go-vendor-tools.toml complyctl.spec` and verify
  `complyctl-1.0.0-vendor.tar.bz2` is created
- [x] 2.2 Run license detection and populate overrides: run
  `go_vendor_license --config go-vendor-tools.toml --path complyctl.spec
  report --prompt --autofill=auto` and resolve any undetected licenses;
  verify `go-vendor-tools.toml` contains `[[licensing.licenses]]` entries
  for any manually resolved licenses; run `go_vendor_license --config
  go-vendor-tools.toml --path complyctl.spec report expression` and
  confirm exit code 0 with a valid SPDX expression
- [x] 2.3 Determine the composite SPDX license expression: run
  `go_vendor_license --config go-vendor-tools.toml report expression`
  and update the `License:` tag in the spec to match; verify by re-running
  the report and confirming no expression mismatch
- [x] 2.4 Commit `go-vendor-tools.toml` to the repository; verify the
  file is tracked in git

## 3. Packit Configuration

> Prerequisites: Section 1 must be completed first (spec references
> Source1/Source2 which Packit actions will generate).

- [x] 3.1 Add `srpm_build_deps` to `.packit.yaml` with `go-vendor-tools`
  and `golang` entries; verify by inspecting the YAML structure
- [x] 3.2 Add `go-vendor-tools.toml` to `files_to_sync` list; verify
  by inspecting the YAML structure
- [x] 3.3 Add top-level `actions.post-modifications` using the concrete
  shell logic from design D5: detect `$PACKIT_DOWNSTREAM_REPO` for
  context, run `go_vendor_archive create` and `go_vendor_license report
  --verify-spec` under `sh -xeuc` (fail on error); verify by inspecting
  the YAML structure and confirming the shell script matches D5
- [x] 3.4 Add `create_sync_note: false` to avoid README.packit clutter;
  verify by inspecting the YAML structure

## 4. Local Verification

> Prerequisites: All tasks in sections 1-3 must be completed before
> running local verification.

- [x] 4.1 Run `rpmspec --parse complyctl.spec` and confirm no macro
  expansion errors; verify exit code 0 and no `error:` lines in output
- [x] 4.2 Run `rpmlint complyctl.spec` and verify zero errors; document
  any intentional suppressions
- [x] 4.3 Run `rpmbuild -bs` (or equivalent) to create SRPM and confirm
  Source0, Source1, and Source2 are all resolved; verify SRPM is produced
  without errors
- [x] 4.4 Build the RPM locally (via `rpmbuild -ba` or `mock`) and
  confirm the build succeeds, debuginfo subpackage is produced, and
  `complyctl` binary is installed to `%{_bindir}`; verify by listing
  RPM outputs and confirming `complyctl-debuginfo-*.rpm` exists;
  inspect the build log to confirm: (a) `%gocheck2` ran unit tests
  (count > 0) and all passed, (b) tests from excluded paths (`tests/`,
  `cmd/test-provider/`, `cmd/mock-oci-registry/`, `cmd/behavioral-report/`)
  do not appear in the build log, (c) the upstream `vendor/` was
  replaced by the Source1 archive
- [x] 4.5 Verify debuginfo subpackage: run `file` on the installed
  `complyctl` binary to confirm it is dynamically linked (external
  linkmode) and `eu-readelf --debug-dump=info` or equivalent to confirm
  DWARF debug info is present; verify `complyctl-debuginfo-*.rpm`
  contains `.debug` files
- [x] 4.6 Verify the built RPM contains proper license files by running
  `rpm -qL` on the built package; confirm vendor license files are
  present under `/usr/share/licenses/complyctl/`
- [x] 4.7 Build with `rpmbuild -ba --without check` and confirm the
  build succeeds without running tests or license checks; verify the
  resulting RPM still contains correct license files

## 5. Documentation

- [x] 5.1 Add CHANGELOG.md entry for spec modernization (go-vendor-tools
  adoption, debuginfo fix, `%autorelease`, Fedora packaging alignment)
- [x] 5.2 Update AGENTS.md Recent Changes with `rpm-spec-modernization`
  summary covering: go-vendor-tools adoption, debuginfo fix, vendor
  archive workflow, Packit `post-modifications` action
- [x] 5.3 Assess website gate: packaging infrastructure only, no
  user-facing workflow changes; exempt per CI-only scope

<!-- spec-review: passed -->

# Release Process for complyctl

The release process values simplicity and automation in order to provide better predictability and low cost for maintainers.

## Process Description

Release artifacts are orchestrated by [GoReleaser](https://goreleaser.com/), which is configured in [.goreleaser.yaml](https://github.com/complytime/complyctl/blob/main/.goreleaser.yaml)

There is a [Workflow](https://github.com/complytime/complyctl/blob/main/.github/workflows/release.yml) created specifically for releases. This workflow is triggered manually by a project maintainer when a new release is ready to be published.

This workflow needs to be associated with a [Tag](https://github.com/complytime/complyctl/tags) for the corresponding release. If the tag is not already available, a project maintainer can create it. Here is an example to create the tag `v0.0.8`:

```bash
git remote -v
...
upstream	https://github.com/complytime/complyctl.git (push)
```

```bash
git tag v0.0.8
git push upstream v0.0.8
```

Once the automation is finished without issues, the release is available in [releases page](https://github.com/complytime/complyctl/releases)

## Tests

Tests relevant for releases are incorporated in CI tests for every PR.

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

Create a fork from https://src.fedoraproject.org/rpms/complyctl

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

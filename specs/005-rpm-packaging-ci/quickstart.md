# Quickstart: RPM Packaging and CI for Split Repositories

**Feature**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)
**Date**: 2026-04-24

## Prerequisites

```bash
# Fedora packaging tools
sudo dnf install rpm-build mock golang go-rpm-macros

# TMT tools (optional, for local test plan validation)
sudo dnf install tmt

# Ensure mock group membership
sudo usermod -a -G mock $USER
```

## Testing complyctl RPM Locally

### Quick spec lint

```bash
rpmlint complyctl.spec
```

### Build with mock (Fedora rawhide)

```bash
# Download source tarball
spectool -g -R complyctl.spec

# Build SRPM
rpmbuild -bs complyctl.spec \
  --define "_sourcedir $(pwd)" \
  --define "_srcrpmdir $(pwd)"

# Build in mock
mock -r fedora-rawhide-x86_64 rebuild complyctl-*.src.rpm
```

### Verify the built RPM

```bash
# List files in the RPM
rpm -qlp /var/lib/mock/fedora-rawhide-x86_64/result/complyctl-*.x86_64.rpm

# Verify no provider binaries are included
rpm -qlp /var/lib/mock/fedora-rawhide-x86_64/result/complyctl-*.x86_64.rpm \
  | grep -v debuginfo | grep provider && echo "FAIL: provider found" || echo "OK"

# Verify man page is included
rpm -qlp /var/lib/mock/fedora-rawhide-x86_64/result/complyctl-*.x86_64.rpm \
  | grep complyctl.1 && echo "OK: man page found" || echo "FAIL: missing man page"

# Verify bundled provides are generated
rpm -qp --provides /var/lib/mock/fedora-rawhide-x86_64/result/complyctl-*.x86_64.rpm \
  | grep "bundled(golang" | head -5
```

### Test with Packit locally (alternative)

```bash
packit build locally
```

## Testing complytime-providers RPM Locally

Run from the `complytime-providers` repository:

```bash
# Download source tarball
spectool -g -R complytime-providers.spec

# Build SRPM
rpmbuild -bs complytime-providers.spec \
  --define "_sourcedir $(pwd)" \
  --define "_srcrpmdir $(pwd)"

# Build in mock
mock -r fedora-rawhide-x86_64 rebuild complytime-providers-*.src.rpm

# Verify two sub-packages produced (no main package)
ls /var/lib/mock/fedora-rawhide-x86_64/result/*.rpm | grep -v src | grep -v debug

# Expected output:
# complytime-providers-openscap-*.x86_64.rpm
# complytime-providers-ampel-*.x86_64.rpm
# (no complytime-providers-*.x86_64.rpm without a suffix)

# Check provider binary paths
rpm -qlp /var/lib/mock/fedora-rawhide-x86_64/result/complytime-providers-openscap-*.x86_64.rpm
rpm -qlp /var/lib/mock/fedora-rawhide-x86_64/result/complytime-providers-ampel-*.x86_64.rpm

# Verify dependency on complyctl
rpm -qp --requires /var/lib/mock/fedora-rawhide-x86_64/result/complytime-providers-openscap-*.x86_64.rpm \
  | grep complyctl
```

## Verifying TMT Plans

```bash
# Lint TMT plans (from repo root)
tmt lint

# List discovered plans
tmt plan ls

# Dry-run tests locally
tmt run --all provision --how local discover --how fmf
```

## Verifying Packit Configuration

```bash
# Validate packit config
packit validate-config

# Test COPR build locally
packit build locally
```

## Verifying GoReleaser Configuration

```bash
# Check config syntax (does not run a release)
goreleaser check

# Dry-run build (produces local artifacts without publishing)
goreleaser build --snapshot --clean
```

## Man Page Workflow

```bash
# Update the man page source (docs/man/complyctl.md)
# Then regenerate the man page:
make man

# Verify the generated man page
man -l docs/man/complyctl.1
```

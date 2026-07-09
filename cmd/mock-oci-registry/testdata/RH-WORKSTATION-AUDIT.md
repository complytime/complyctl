# Red Hat Workstation Security Audit Policy

Automated compliance policy for Red Hat workstation security baseline verification, replacing manual screenshot evidence collection.

## Purpose

Automates the Red Hat external audit requirement to provide evidence of:
1. SELinux enforcement mode (`sestatus`)
2. Supported OS version (`cat /etc/redhat-release`)

## Gemara Layers

### Layer 1: Guidance (`rh-workstation-audit-guidance.yaml`)
- **Framework**: CIS Controls v8
- **Controls**:
  - Control 4: Secure Configuration of Enterprise Assets
  - Control 7: Continuous Vulnerability Management

### Layer 2: Control Catalog (`rh-workstation-audit-catalog.yaml`)
- **RH-AUDIT-1**: SELinux Enforcement - Verify SELinux is enabled and in enforcing mode
- **RH-AUDIT-2**: Supported OS Version - Verify running Fedora 43+ or RHEL 8+

### Layer 3: Policy/Assessment (`rh-workstation-audit-policy.yaml`)
- **Assessment Plan 1**: `selinux_state` - Uses existing SSG rule
- **Assessment Plan 2**: `fedora_supported_version` - Uses custom OVAL check

## Custom OVAL Check

**File**: `fedora-version-check.oval.xml`

Checks `/etc/os-release` for `VERSION_ID >= 43` (Fedora minimum version for testing).

## Usage

### 1. Start Mock Registry
```bash
make mock-registry
```

### 2. Copy Sample Config
```bash
cp cmd/mock-oci-registry/testdata/rh-workstation-complytime.yaml ./complytime.yaml
```

### 3. Fetch Policy
```bash
bin/complyctl get
```

### 4. Generate Tailoring
```bash
bin/complyctl generate --policy-id policies/rh-workstation-audit
```

### 5. Run Scan (requires sudo)
```bash
sudo bin/complyctl scan --policy-id policies/rh-workstation-audit
```

### 6. View Results
```bash
cat ./.complytime/scan/evaluation-log-*.json
# Or generate markdown report
sudo bin/complyctl scan --policy-id policies/rh-workstation-audit --format pretty
```

## Output

Instead of manual screenshots, you upload the formal compliance scan report showing:
- ✓ SELinux Enforcement: PASS (enforcing mode active)
- ✓ Supported OS Version: PASS (Fedora 43 >= minimum)

## Files Created

- `rh-workstation-audit-guidance.yaml` - CIS Controls v8 guidance layer
- `rh-workstation-audit-catalog.yaml` - Control catalog with 2 controls
- `rh-workstation-audit-policy.yaml` - OpenSCAP assessment policy
- `fedora-version-check.oval.xml` - Custom OVAL definition for OS version check
- `rh-workstation-complytime.yaml` - Sample workspace configuration
- Updated `main.go` - Mock registry serves the new policy

## Next Steps

1. Test the full workflow with `make mock-registry` and complyctl commands
2. Validate Gemara layers using gemara-mcp server (requires restart)
3. Extend to support RHEL version checks (modify OVAL for RHEL_VERSION)
4. Add additional workstation security controls as needed

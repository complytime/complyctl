# Installation

## Binary

The latest binary release can be downloaded from <https://github.com/complytime/complyctl/releases/latest>.

Verify the release signature:

```bash
cosign verify-blob \
  --certificate complyctl_*_checksums.txt.pem \
  --signature complyctl_*_checksums.txt.sig \
  complyctl_*_checksums.txt \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  --certificate-identity=https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/heads/main
```

## From Source

### Prerequisites

- **Go** 1.24+
- **Make**
- **buf** CLI (optional, for protobuf regeneration)

### Clone and build

```bash
git clone https://github.com/complytime/complyctl.git
cd complyctl
make build
```

Binaries are placed in `bin/`. Add it to your `PATH`:

```bash
export PATH="$PWD/bin:$PATH"
```

### Build the test provider (optional)

```bash
make build-test-provider
```

Produces `bin/complyctl-provider-test` for use in E2E testing. See [E2E_INTEGRATION.md](E2E_INTEGRATION.md).

# Installation

## Homebrew (macOS / Linux)

```bash
brew install complytime/tap/complyctl
```

To upgrade:

```bash
brew upgrade complytime/tap/complyctl
```

Shell completions for Bash, Zsh, and Fish are installed automatically.

### Verifying the Homebrew installation

After installing, confirm the binary is linked and functional:

```bash
# Check version and build metadata
complyctl version

# Run the formula's built-in test (version output match)
brew test complyctl

# Verify shell completions were generated
ls "$(brew --prefix)/share/zsh/site-functions/_complyctl" 2>/dev/null && echo "zsh completions OK"
ls "$(brew --prefix)/etc/bash_completion.d/complyctl" 2>/dev/null && echo "bash completions OK"
ls "$(brew --prefix)/share/fish/vendor_completions.d/complyctl.fish" 2>/dev/null && echo "fish completions OK"
```

For contributors testing a formula from a feature branch before release:

```bash
make test-homebrew
```

This installs from the current branch via `brew install --HEAD`,
runs `brew test`, and cleans up automatically. See
`tests/homebrew_test.sh` for details.

## go install

If you already have a Go toolchain:

```bash
go install github.com/complytime/complyctl/cmd/complyctl@latest
```

This builds from source on your machine. Version information will show
`(devel)` unless you pass `-ldflags` manually.

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

- **Go** 1.26+
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

## Next steps

complyctl requires scanning providers to run compliance scans. After installing complyctl, see the [Quick Start](QUICK_START.md) to set up a provider and run your first scan.

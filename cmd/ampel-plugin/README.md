# ampel-plugin

## Overview

NOTE: The development of this plugin is in progress and therefore it should only be used for testing purposes at this point.

**ampel-plugin** is a plugin which extends the complyctl capabilities to verify branch protection settings on GitHub repositories using [AMPEL](https://github.com/carabiner-dev/ampel) and [snappy](https://github.com/carabiner-dev/snappy). The plugin communicates with complyctl via gRPC, providing a standard and consistent communication mechanism that gives independence for plugin developers to choose their preferred languages. This plugin is structured to allow modular development, ease of packaging, and maintainability.

For now, this plugin is developed together with complyctl for better collaboration during this phase of the project. In the future, this plugin may be decoupled into its own repository.

## Plugin Structure

```
ampel-plugin/
├── config/               # Package for plugin configuration
│ ├── config_test.go      # Tests for functions in config.go
│ └── config.go           # Main code used to process plugin configuration
├── convert/              # Package to convert OSCAL rules to AMPEL policies
│ ├── convert_test.go     # Tests for functions in convert.go
│ ├── convert.go          # Main code used to match and merge AMPEL policies
│ └── types.go            # AMPEL policy type definitions
├── docs/                 # Documentation and sample files
│ └── samples/            # Sample configuration files
├── results/              # Package to parse AMPEL results and map to OSCAL
│ ├── results_test.go     # Tests for functions in results.go
│ └── results.go          # Main code used to parse AMPEL output and produce OSCAL observations
├── scan/                 # Package to execute snappy and ampel commands
│ ├── scan_test.go        # Tests for functions in scan.go
│ ├── scan.go             # Main code used to orchestrate repository scanning
│ └── specs/              # Embedded spec files for snappy
├── server/               # Package to process server functions. Here is where the plugin communicates with complyctl CLI
│ ├── server_test.go      # Tests for functions in server.go
│ └── server.go           # Main code used to process server functions
├── targets/              # Package to load and validate target repository configuration
│ ├── targets_test.go     # Tests for functions in targets.go
│ └── targets.go          # Main code used to load target YAML configuration
├── toolcheck/            # Package to verify required external tools are available
│ ├── toolcheck_test.go   # Tests for functions in toolcheck.go
│ └── toolcheck.go        # Main code used to check snappy and ampel availability
├── main.go               # Plugin entry point
└── README.md             # This file
```

## Features

### Configuration

The plugin has some parameters that can be configured via the manifest file. Check the quick start [guide](../../docs/QUICK_START.md) to see an example.
Complyctl processes the manifest file and sends the configuration values to the plugin.

These are the configuration values used by ampel-plugin:
- **workspace**: Directory used to store plugin artifacts (policies, results, specs). This configuration can also be set by complyctl. Within this directory, the plugin creates an `ampel/` subdirectory for its files.
- **profile**: Is the FrameworkID informed by complyctl. This FrameworkID corresponds to the compliance profile used for branch protection checks.
- **policy_dir**: Directory containing granular AMPEL policy files (JSON). Default: `policy` (resolved relative to `{workspace}/ampel/`).
- **results_dir**: Directory for per-repository scan result files. Default: `results` (resolved relative to `{workspace}/ampel/`).
- **targets_file**: Path to the YAML file defining which repositories and branches to scan. Default: `ampel-targets.yaml` (resolved relative to `{workspace}/ampel/`).

### Target Configuration

The plugin requires a targets file that defines which repositories and branches to scan. The file uses YAML format:

```yaml
repositories:
  - url: https://github.com/myorg/myrepo
    branches:
      - main
      - release
    specs:
      - builtin:github/branch-rules.yaml
  - url: https://github.com/myorg/another-repo
    branches:
      - main
    specs:
      - builtin:github/branch-rules.yaml
```

Each repository entry supports:
- **url** (required): HTTPS URL to a GitHub repository.
- **branches** (required): List of branch names to scan.
- **specs** (required): List of snappy spec file references. Use the `builtin:` prefix for embedded specs (e.g., `builtin:github/branch-rules.yaml`) or absolute paths for custom specs.

A sample targets file is available at `docs/samples/ampel-targets.yaml`.

### AMPEL Policies

The plugin uses granular AMPEL policy files (one JSON file per control) stored in the `policy_dir` directory. During the `generate` phase, the plugin matches OSCAL assessment plan rules to these policies and merges the matched policies into a single bundle used for verification.

Sample policy files are available in the [complytime-demos](https://github.com/complytime/complytime-demos) repository under `base_ansible_env/files/ampel-policies/`.

### Generate

When the plugin receives the `generate` command from complyctl, it will:
* Load granular AMPEL policy files from the configured `policy_dir`
* Match OSCAL assessment plan rules to available AMPEL policies by rule ID
* Merge matched policies into a single policy bundle
* Write the bundle to `{workspace}/ampel/policy/complytime-ampel-policy.json`

### Scan

When the plugin receives the `scan` command from complyctl, it will:
* Validate that `snappy` and `ampel` CLI tools are available on the system PATH
* Load the target repository configuration from the targets file
* For each repository, branch, and spec combination:
  * Run `snappy snap` to collect branch protection data from the GitHub API as an in-toto attestation
  * Extract the subject hash from the snappy attestation
  * Run `ampel verify` to evaluate the attestation against the generated policy bundle
  * Parse the AMPEL verification results (supporting both raw and DSSE-wrapped attestations)
* Write per-repository result files to the configured `results_dir`
* Return OSCAL observations to complyctl so an `assessment-results.json` file can be created

## Installation

### Prerequisites

- **Go** version 1.22 or higher
- **Make** (optional, for using the Makefile)
- **snappy** CLI tool
- **ampel** CLI tool
- A **GitHub personal access token** with repository read permissions

### Installing snappy and ampel

Since snappy and ampel are not available as RPM packages, install them using `go install`:

```bash
go install github.com/carabiner-dev/snappy@latest
go install github.com/carabiner-dev/ampel/cmd/ampel@latest
```

Ensure the Go binary directory is in your PATH:

```bash
export PATH=$PATH:$HOME/go/bin
```

You can add this line to your `~/.bashrc` or `~/.zshrc` to make it permanent.

Verify the installation:

```bash
snappy --help
ampel --help
```

### GitHub Token

The `snappy` tool requires a valid GitHub personal access token to access the GitHub API for reading branch protection settings. Set the `GITHUB_TOKEN` environment variable before running a scan:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

The token needs at minimum read access to the repositories being scanned.

### Clone the repository

```bash
git clone https://github.com/complytime/complyctl.git
cd complyctl
```

## Build Instructions

To compile complyctl and ampel-plugin:

```bash
make build
```

### Plugin Registration

After building, register the plugin with complyctl by placing the manifest and binary in the plugins directory:

```bash
mkdir -p ~/.local/share/complytime/plugins

cp bin/ampel-plugin ~/.local/share/complytime/plugins/
cp cmd/ampel-plugin/docs/samples/c2p-ampel-manifest.json ~/.local/share/complytime/plugins/
```

Update the `sha256` field in the manifest file with the checksum of the binary:

```bash
sha256sum ~/.local/share/complytime/plugins/ampel-plugin
```

Edit `~/.local/share/complytime/plugins/c2p-ampel-manifest.json` and set the `sha256` field to the computed checksum.

### Running

To use the plugin with `complyctl`, see the quick start [guide](../../docs/QUICK_START.md).

### Using complytime-demos with a Fedora 43 VM

The [complytime-demos](https://github.com/complytime/complytime-demos) repository provides an automated way to set up a complete environment with complyctl, the ampel-plugin, and all required tools inside a Fedora 43 VM using Vagrant and Ansible.

**Prerequisites:** Vagrant with the libvirt provider and Ansible installed on the host.

1. Clone both repositories on the host machine:

```bash
git clone https://github.com/complytime/complytime-demos.git
git clone https://github.com/complytime/complyctl.git
```

2. Provision the Fedora 43 VM:

```bash
cd complytime-demos/base_vms/fedora
vagrant up
```

3. Deploy complyctl binaries, the ampel-plugin, AMPEL tools (snappy and ampel), policies, and targets to the VM:

```bash
cd ../../base_ansible_env
ansible-playbook populate_complyctl_dev_binaries.yml
```

This playbook builds complyctl from source, copies the binaries and plugin manifests, installs snappy and ampel via `go install`, and deploys the AMPEL policy files and targets configuration to the VM.

4. Deploy the AMPEL OSCAL content (catalog, profile, and component definition):

```bash
ansible-playbook populate_complyctl_dev_content_ampel.yml
```

5. SSH into the VM and run a scan:

```bash
vagrant ssh
# or: ssh ansible@<VM_IP>

export GITHUB_TOKEN=ghp_your_token_here
complyctl plan ampel_bp
complyctl generate
complyctl scan -d
```

Note: Update the `complyctl_repo_dest` variable in the playbook if your local complyctl clone is not at the default path. See the complytime-demos README for additional configuration options.

### Testing

Tests are organized within each package. Whenever possible a unit test is created for every function.

Run tests using:

```bash
make test-units
```

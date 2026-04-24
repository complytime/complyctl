# Data Model: RPM Packaging and CI for Split Repositories

**Feature**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)
**Date**: 2026-04-24

## RPM Package Entities

### complyctl (source RPM → binary RPM)

```text
complyctl (source RPM)
└── complyctl (binary RPM)
    ├── /usr/bin/complyctl                                  # CLI binary
    ├── /usr/share/man/man1/complyctl.1.gz                  # Man page
    ├── /usr/share/licenses/complyctl/LICENSE                # License
    ├── /usr/share/licenses/complyctl/modules.txt            # Auto-generates bundled provides
    ├── /usr/libexec/complytime/                             # Owned directory
    └── /usr/libexec/complytime/providers/                   # Owned directory (providers install here)
```

- **Source**: `github.com/complytime/complyctl`
- **Module path**: `github.com/complytime/complyctl`
- **Build target**: `./cmd/complyctl/`
- **Version injection**: 4 linker flags into `internal/version.*`
- **Dependency direction**: None (standalone, no provider requirements)

### complytime-providers (source RPM → two sub-packages)

```text
complytime-providers (source RPM)
├── complytime-providers-openscap (binary sub-package)
│   ├── /usr/libexec/complytime/providers/complyctl-provider-openscap
│   └── /usr/share/licenses/complytime-providers-openscap/LICENSE
│   Requires: complyctl >= X.Y.Z
│   Requires: scap-security-guide
│
└── complytime-providers-ampel (binary sub-package)
    ├── /usr/libexec/complytime/providers/complyctl-provider-ampel
    └── /usr/share/licenses/complytime-providers-ampel/LICENSE
    Requires: complyctl >= X.Y.Z
```

- **Source**: `github.com/complytime/complytime-providers`
- **Module path**: `github.com/complytime/complytime-providers`
- **Build targets**: `./cmd/openscap-provider/` and `./cmd/ampel-provider/`
- **No main binary RPM** is produced (no `%files` for main package)
- **No version injection** (RPM version suffices for providers)

## Dependency Graph

```text
                    ┌──────────────────────────────────────┐
                    │  complytime-providers-openscap        │
                    │  (complyctl-provider-openscap binary) │
                    └──────────┬──────────┬────────────────┘
                               │          │
                   Requires >= │          │ Requires
                               │          │
                    ┌──────────▼──┐   ┌───▼──────────────────┐
                    │  complyctl   │   │  scap-security-guide  │
                    │  (CLI binary)│   │  (SCAP content)       │
                    └──────────▲──┘   └────────────────────────┘
                               │
                   Requires >= │
                               │
                    ┌──────────┴──────────────────────────┐
                    │  complytime-providers-ampel           │
                    │  (complyctl-provider-ampel binary)    │
                    └──────────────────────────────────────┘
```

## Directory Ownership

| Path                                  | Owned By       | Notes                             |
|---------------------------------------|----------------|-----------------------------------|
| `/usr/bin/complyctl`                  | complyctl      | CLI entry point                   |
| `/usr/share/man/man1/complyctl.1.gz`  | complyctl      | Man page                          |
| `/usr/libexec/complytime/`           | complyctl      | Parent directory                  |
| `/usr/libexec/complytime/providers/` | complyctl      | Provider discovery directory      |
| `.../providers/complyctl-provider-openscap` | complytime-providers-openscap | Installed into complyctl-owned dir |
| `.../providers/complyctl-provider-ampel`    | complytime-providers-ampel    | Installed into complyctl-owned dir |

## Packit Pipeline Stages

```text
┌─ Per-PR Pipeline ─────────────────────────────────────────────────────┐
│                                                                       │
│  PR opened → copr_build → tests (Testing Farm/TMT) → PR status check │
│                                                                       │
└───────────────────────────────────────────────────────────────────────┘

┌─ Release Pipeline ────────────────────────────────────────────────────┐
│                                                                       │
│  Git tag → GoReleaser → GitHub Release                                │
│       └─► Packit propose_downstream → dist-git PR                     │
│              └─► (merge) → koji_build → bodhi_update                  │
│                                                                       │
└───────────────────────────────────────────────────────────────────────┘
```

Both pipelines run independently for each repository. The complyctl pipeline
produces a single binary RPM. The complytime-providers pipeline produces two
sub-package RPMs.

## CI Target Matrix

| Target                    | COPR Build | Testing Farm | Propose Downstream | Koji Build | Bodhi Update |
|---------------------------|:---:|:---:|:---:|:---:|:---:|
| fedora-rawhide-x86_64     | PR  | PR  | release | commit | --     |
| fedora-43-x86_64          | PR  | PR  | release | commit | commit |
| fedora-42-x86_64          | PR  | PR  | release | commit | commit |
| centos-stream-10-x86_64   | PR  | PR  | --      | --     | --     |
| centos-stream-9-x86_64    | PR  | PR  | --      | --     | --     |

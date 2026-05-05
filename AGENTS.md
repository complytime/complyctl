# complyctl Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-21

## Active Technologies
- Go 1.25 + go-rpm-macros, Packit, Testing Farm (TMT/FMF) (005-rpm-packaging-ci)

- Go 1.25 (complyctl root `go.mod`) (004-providers-repository-split)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.25 (complyctl root `go.mod`)

## Code Style

Go 1.25 (complyctl root `go.mod`): Follow standard conventions

## Recent Changes
- 005-rpm-packaging-ci: Added Go 1.25 + go-rpm-macros, Packit, Testing Farm (TMT/FMF)

- 004-providers-repository-split: Providers (openscap, ampel) migrated to `complytime-providers`; `pkg/plugin/` renamed to `pkg/provider/`; all "plugin" terminology updated to "provider"

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

## Convention Packs

This repository uses convention packs scaffolded by
unbound-force. Agents MUST read the applicable pack(s)
before writing or reviewing code.

- `.opencode/uf/packs/default.md`
- `.opencode/uf/packs/default-custom.md`
- `.opencode/uf/packs/severity.md`
- `.opencode/uf/packs/content.md`
- `.opencode/uf/packs/content-custom.md`
- `.opencode/uf/packs/go.md`
- `.opencode/uf/packs/go-custom.md`

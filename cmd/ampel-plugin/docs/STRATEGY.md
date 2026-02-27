# ampel-plugin Strategy

## Introduction

The ampel-plugin extends complyctl to verify branch protection settings on GitHub repositories using [AMPEL](https://github.com/carabiner-dev/ampel) and [snappy](https://github.com/carabiner-dev/snappy). This document describes the key design decisions, the value proposition of this approach, and the next steps for the plugin.

## Granular AMPEL Policies

A central design choice in the ampel-plugin is the use of **granular AMPEL policies** — one JSON file per control — rather than a single monolithic policy bundle.

Each policy file is self-contained and independently authored. For example, `SC-CODE-01.01-require-pull-request.json` defines a single control with its own CEL verification logic, attestation type references, assessment messages, and remediation guidance. This makes policies:

- **Easy to author and review**: Each file covers one control with a clear purpose.
- **Independently testable**: A single policy can be validated with `ampel verify` in isolation.
- **Composable**: The plugin dynamically selects and merges only the policies that match the active assessment plan, producing a single policy bundle at scan time.

During the `generate` phase, the plugin reads the OSCAL assessment plan, matches its rule IDs against the available granular policies, and merges the matching policies into a combined bundle written to `{workspace}/ampel/policy/complytime-ampel-policy.json`. Policies that are not referenced by the assessment plan are simply excluded — no manual bundle maintenance is required.

This granular approach is also aligned with how the [Gemara2Ampel](https://github.com/complytime/complytime-demos/tree/main/tools/gemara2ampel) tool works: Gemara Layer 3 policies map naturally to individual AMPEL policy files, and the workspace mode (`-w`) of Gemara2Ampel already produces one file per policy. When complyctl adopts Gemara as its policy source, this granular structure will allow the plugin to consume Gemara-generated policies without changes to the matching and merging logic.

## Multi-Target Scanning

The ampel-plugin introduces multi-target scanning to complyctl. Unlike the existing openscap-plugin, which scans the local system it runs on, the ampel-plugin scans remote repositories defined in a targets configuration file.

A single scan run can evaluate multiple repositories, branches, and spec files. The results from all targets are aggregated into the OSCAL assessment results, with each check (ObservationByCheck) containing one subject per scanned repository. This allows a single `complyctl scan` invocation to produce a unified compliance view across an entire set of repositories.

## Value of Using complyctl with ampel-plugin

Using AMPEL directly (snappy + ampel verify) is sufficient for ad-hoc verification of individual repositories. The value of integrating AMPEL through complyctl comes from two capabilities that AMPEL alone does not provide:

**Dynamic policy generation from organizational policy**: The plugin translates high-level compliance requirements (currently from OSCAL assessment plans, and in the future from Gemara policies) into concrete AMPEL verification policies. This means the set of checks executed during a scan is derived from the organizational policy rather than manually assembled. When the policy changes, the generated AMPEL bundle changes accordingly — without editing AMPEL files by hand.

**Standardized OSCAL results**: AMPEL produces technology-specific in-toto attestations with CEL evaluation outcomes. The plugin transforms these results into OSCAL Assessment Results (assessment-results.json), a standard format that is independent of AMPEL, snappy, or any other verification technology. This allows complyctl to combine results from multiple plugins (e.g., openscap-plugin for system hardening, ampel-plugin for branch protection) into a single, unified compliance report.

Together, these capabilities position complyctl as the layer that connects organizational governance policy to technology-specific verification tools, while maintaining a standard interface for both policy input and compliance output.

## Next Actions

1. **Review and update the Gemara2Ampel tool to work with granular AMPEL policies**
   The Gemara2Ampel converter currently supports workspace mode for generating individual policy files. It should be reviewed to ensure its output aligns with the granular policy format expected by the ampel-plugin (policy ID, meta.controls, tenets structure), so that Gemara-generated policies can be used directly as input to the plugin without manual adjustments.

2. **Update the plugin API once complyctl supports Gemara**
   When complyctl transitions from OSCAL assessment plans to Gemara as its policy source, the plugin's `Generate` phase will need to match Gemara policy identifiers (instead of OSCAL rule IDs) against the granular AMPEL policies. The matching and merging logic in the `convert` package is isolated for this purpose. The `server.Generate` method should be updated to accept the new policy format from complyctl once the API is ready.

3. **Implement Gemara results for each target**
   Extend the plugin to produce Gemara-formatted results per target repository, in addition to the current OSCAL observations. This will allow downstream consumers that work with Gemara to receive structured compliance data directly from the scan, with per-repository granularity matching the multi-target scanning model.

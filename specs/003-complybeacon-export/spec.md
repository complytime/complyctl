# Feature Specification: ComplyBeacon Evidence Export

**Feature Branch**: `003-complybeacon-export`
**Created**: 2026-03-10
**Status**: Draft
**Input**: Jira ticket: "Improve Integration with ComplyBeacon. Plugins leverage Proofwatch and send results directly to the collector on a plugin by plugin basis."

## Clarifications

### Session 2026-03-10

- Q: Should evidence export be a new command or part of the
  existing scan command? → A: Part of the existing scan
  command via `--format otel`. The scan command already
  supports output formats (`oscal`, `pretty`, `sarif`).
  Adding `otel` as a format keeps the UX consistent —
  the user chooses where results go at scan time. This
  avoids a two-step workflow and makes export a
  lightweight addition to the existing scan flow rather
  than a separate lifecycle concept.

- Q: Is the enrichment step still needed in complyctl? → A:
  No. Enrichment is handled by the TruthBeam processor
  inside the Beacon collector. complyctl and plugins send
  evidence with Gemara attributes; TruthBeam enriches it
  in-pipeline with compliance context (control mappings,
  frameworks, risk levels). complyctl does not need to
  understand or implement enrichment logic.

- Q: What are we exporting — raw plugin evidence or Gemara
  results? → A: Gemara results. ProofWatch already provides
  a `GemaraEvidence` type that wraps `layer4.Metadata` and
  `layer4.AssessmentLog` from go-gemara. complyctl's scan
  already produces `AssessmentLog` entries per requirement.
  Plugins convert their scan results to `GemaraEvidence`
  and emit via ProofWatch. This preserves the Gemara data
  model end-to-end and lets TruthBeam enrich using Gemara
  attribute keys (`policy.rule.id`, `policy.engine.name`,
  `policy.evaluation.result`).

- Q: Should each plugin implement its own export, or should
  complyctl handle exporting centrally? → A: Plugins export
  directly. Each plugin uses the ProofWatch library to emit
  evidence as OTLP log records to the configured collector
  endpoint. complyctl orchestrates by calling a new `Export`
  RPC on each plugin that supports it, passing the collector
  configuration. Plugins that do not support export are
  skipped. This design avoids funneling potentially remote
  or large evidence through complyctl's gRPC pipe — the
  plugin sends evidence directly from where it lives to the
  collector.

- Q: How does complyctl know which plugins support export?
  → A: The `Describe` RPC response gains a capabilities
  field. Plugins that implement the `Export` RPC declare
  `supports_export: true`. complyctl skips plugins that do
  not declare this capability. Plugins that declare support
  but fail during export return an error status without
  affecting other plugins.

- Q: Where does collector endpoint and authentication
  configuration live? → A: `complytime.yaml` top-level
  `collector` section with `endpoint` and `auth` fields.
  complyctl reads this configuration when `--format otel`
  is used and passes it to plugins via the `Export` RPC
  request. This keeps all workspace configuration in one
  place, consistent with how policies, targets, and
  variables are already configured.

- Q: How does authentication work? → A: OIDC client
  credentials flow. The user configures `client-id`,
  `client-secret`, and `token-endpoint` in
  `complytime.yaml`. When `--format otel` is used,
  complyctl performs the OAuth2 client credentials
  handshake with the token endpoint and obtains a
  short-lived access token. complyctl passes this resolved
  token to plugins via the `Export` RPC. Plugins never
  handle client secrets or token exchange — they always
  receive a ready-to-use bearer token. This centralizes
  auth complexity in complyctl and keeps plugins simple.
  Example:
  ```yaml
  collector:
    endpoint: "collector.example.com:4317"
    auth:
      client-id: "${BEACON_CLIENT_ID}"
      client-secret: "${BEACON_CLIENT_SECRET}"
      token-endpoint: "https://sso.example.com/realms/comply/protocol/openid-connect/token"
  ```

- Q: What transport does ProofWatch use to reach the
  collector? → A: OTLP (OpenTelemetry Protocol). The Beacon
  collector accepts OTLP over gRPC (port 4317) or HTTP
  (port 4318), and also offers a webhook receiver
  (`POST /eventsource/receiver` on port 8088). ProofWatch
  uses the OTEL SDK's log exporter, so OTLP gRPC or HTTP
  is the natural fit. The collector endpoint URL determines
  the transport.

- Q: What attributes does the evidence carry? → A: The
  `GemaraEvidence` type maps Gemara assessment fields to
  OTEL attributes: `policy.engine.name` (evaluator/author),
  `compliance.control.id` (requirement entry ID),
  `compliance.control.catalog.id` (catalog reference),
  `policy.evaluation.result` (Passed/Failed/Skipped/Error),
  `policy.rule.id` (assessment plan entry ID),
  `compliance.assessment.id` (assessment log ID), plus
  optional `policy.evaluation.message` and
  `compliance.remediation.description`. These are the
  minimum attributes TruthBeam needs for enrichment.

- Q: What are the RHEL 11 requirements? → A: Out of scope
  for this spec. To be discussed separately with the RHEL
  team. No known blockers at this time.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Export Scan Results to Beacon Collector (Priority: P1)

A compliance administrator runs `complyctl scan --format otel`
to evaluate their systems and ship the assessment results
to the Beacon collector in a single step. The scan command
discovers which plugins support evidence export, runs the
scan as normal, then passes the collector configuration to
each capable plugin. The plugins use ProofWatch to emit
Gemara evidence as OTLP log records directly to the
collector. The administrator sees the normal scan summary
followed by an export summary showing what was shipped and
any errors.

**Why this priority**: This is the core value proposition of
the feature. Without export, scan results remain local and
must be manually transferred to the collector via the
existing CI pipeline workaround.

**Independent Test**: Run `complyctl scan --format otel` with
a running Beacon collector. Verify that OTLP log records
appear in the collector's debug exporter output with the
expected Gemara attributes.

**Acceptance Scenarios**:

1. **Given** a workspace with a configured collector and
   plugins that support export,
   **When** the user runs
   `complyctl scan --policy-id <ID> --format otel`,
   **Then** the scan executes normally, evidence records
   are sent to the configured collector endpoint, and
   the terminal displays the scan summary followed by
   an export summary showing the plugin name, number of
   records exported, and success status.

2. **Given** a scan with multiple plugins where only some
   support export,
   **When** the user runs
   `complyctl scan --policy-id <ID> --format otel`,
   **Then** all plugins scan as normal, plugins that
   support export send their evidence, plugins that do
   not are skipped with an informational message, and
   the export summary reflects both.

3. **Given** a `complytime.yaml` without a `collector`
   section,
   **When** the user runs
   `complyctl scan --policy-id <ID> --format otel`,
   **Then** the command exits with a clear error indicating
   no collector is configured and pointing to documentation.

4. **Given** a configured collector endpoint that is
   unreachable,
   **When** the user runs
   `complyctl scan --policy-id <ID> --format otel`,
   **Then** the scan completes normally, the scan summary
   is displayed, and the export summary reports the
   connection failure per plugin with an actionable error
   message. The command exits with a non-zero status code.

---

### User Story 2 - Configure Collector Endpoint (Priority: P1)

A compliance administrator configures their workspace to
point to the organization's Beacon collector. The collector
endpoint and any required authentication are specified in
the workspace configuration. The `doctor` command validates
the collector configuration.

**Why this priority**: Without collector configuration,
`--format otel` has no destination. This is a prerequisite
for US1.

**Independent Test**: Add a `collector` section to
`complytime.yaml`, run `complyctl doctor`, verify it reports
collector reachability status.

**Acceptance Scenarios**:

1. **Given** a `complytime.yaml` with a valid `collector`
   section,
   **When** the user runs `complyctl doctor`,
   **Then** the doctor output includes a collector
   reachability check showing pass/fail status.

2. **Given** a `complytime.yaml` without a `collector`
   section,
   **When** the user runs
   `complyctl scan --policy-id <ID> --format otel`,
   **Then** the command exits with a clear error indicating
   no collector is configured and pointing to documentation.

3. **Given** a `complytime.yaml` with a collector endpoint
   that requires authentication,
   **When** the auth credentials (`client-id`, `client-secret`,
   or `token-endpoint`) are missing or incomplete,
   **Then** `complyctl scan --format otel` reports a
   validation error with guidance on how to configure
   credentials.

4. **Given** a `complytime.yaml` with valid OIDC client
   credentials,
   **When** the user runs
   `complyctl scan --policy-id <ID> --format otel`,
   **Then** complyctl performs the OAuth2 client credentials
   handshake with the token endpoint, obtains a short-lived
   access token, and passes it to plugins via the Export RPC.

5. **Given** a `complytime.yaml` with OIDC client credentials
   where the token endpoint is unreachable or returns an error,
   **When** the user runs
   `complyctl scan --policy-id <ID> --format otel`,
   **Then** the command reports the token exchange failure
   with an actionable error before attempting any plugin
   exports.

---

### User Story 3 - Plugin Implements Export RPC (Priority: P1)

A plugin author implements the optional `Export` RPC in their
complyctl scanning provider. When `--format otel` is used,
complyctl calls the Export RPC after the scan completes. The
plugin receives collector configuration from complyctl,
creates `GemaraEvidence` objects from its scan results, and
uses ProofWatch to emit them as OTLP log records to the
collector. The plugin reports back the number of records
exported and any errors.

**Why this priority**: At least one plugin must implement
export for the feature to be usable. The OpenSCAP and AMPEL
plugins are the initial targets.

**Independent Test**: Build a test plugin that implements
Export, run `complyctl scan --format otel`, verify evidence
records arrive at the collector with correct Gemara
attributes.

**Acceptance Scenarios**:

1. **Given** a plugin that implements the Export RPC,
   **When** complyctl calls Export with collector config
   after a successful scan,
   **Then** the plugin emits one `GemaraEvidence` OTLP log
   record per assessment result and returns a success
   response with the count.

2. **Given** a plugin that does NOT implement the Export RPC,
   **When** complyctl calls Describe and sees
   `supports_export: false`,
   **Then** complyctl skips calling Export on that plugin
   and reports it as skipped in the export summary.

3. **Given** a plugin that implements Export but encounters
   a partial failure (e.g., 8 of 10 records sent, 2 failed),
   **When** the Export RPC returns,
   **Then** the response includes the count of successful
   and failed records, and complyctl surfaces this in the
   export summary.

---

### User Story 4 - Auditor Verifies Evidence Chain (Priority: P2)

An auditor needs to verify that compliance evidence in the
downstream reporting system traces back to the original
scan. The evidence exported by complyctl via ProofWatch
carries integrity attributes (assessment IDs, timestamps,
evaluator identity) that propagate through the Beacon
collector to storage backends and reporting tools. The
auditor can correlate evidence records across systems using
these attributes.

**Why this priority**: Audit traceability is a key
requirement but depends on US1-US3 being complete. The
integrity chain (scan → OTLP → collector → storage →
reporting) is mostly handled by existing ComplyBeacon
infrastructure; complyctl's responsibility is emitting the
right attributes.

**Independent Test**: Export evidence, query the storage
backend for the `compliance.assessment.id` attribute, verify
it matches the EvaluationLog YAML produced by scan.

**Acceptance Scenarios**:

1. **Given** evidence exported from a complyctl scan,
   **When** an auditor queries the collector backend
   (Loki or S3) by `compliance.assessment.id`,
   **Then** the returned record contains all Gemara
   attributes (`policy.rule.id`, `policy.engine.name`,
   `policy.evaluation.result`, `compliance.control.id`)
   matching the original scan output.

2. **Given** evidence exported from multiple scan runs,
   **When** an auditor queries by `policy.engine.name`
   and time range,
   **Then** results are distinguishable by timestamp and
   assessment ID, enabling temporal audit trails.

---

### Edge Cases

- When a plugin's evidence source is remote (e.g., CI
  artifact on GitHub Actions), the plugin is responsible
  for retrieving and converting it before emitting via
  ProofWatch. complyctl does not manage remote evidence
  retrieval.
- When the collector endpoint uses TLS with a custom CA,
  the configuration must support specifying a CA certificate
  path (standard OTEL SDK TLS configuration).
- When `--format otel` is used on repeated scans, evidence
  records are re-sent each time. Deduplication is the
  collector's responsibility, not complyctl's.
- When a scan produces zero results for a given plugin,
  export for that plugin emits zero records and reports
  success (not an error).
- When the OTEL SDK encounters a transient network error,
  retry behavior follows the SDK's default retry policy.
  complyctl does not implement custom retry logic.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `scan` command MUST accept `otel` as a
  value for the existing `--format` flag. When
  `--format otel` is used, the scan executes normally and
  then orchestrates evidence export to the configured
  Beacon collector.

- **FR-002**: When `--format otel` is active, complyctl
  MUST discover plugins that support export by checking
  the `supports_export` capability in the `Describe` RPC
  response.

- **FR-003**: When `--format otel` is active, complyctl
  MUST pass collector configuration (endpoint URL, resolved
  access token) to each capable plugin via the `Export` RPC
  request. When OIDC client credentials are configured,
  complyctl MUST perform the token exchange and pass the
  resulting access token — plugins MUST NOT receive client
  secrets.

- **FR-004**: The Plugin gRPC service MUST be extended with
  an `Export` RPC method. The method receives collector
  configuration and returns an export status (success/fail,
  record count, error details).

- **FR-005**: The `DescribeResponse` message MUST be extended
  with a `supports_export` boolean field so complyctl can
  determine plugin capability without attempting the RPC.

- **FR-006**: Plugins implementing Export MUST use the
  ProofWatch library (`github.com/complytime/complytime-collector-components/proofwatch`)
  to emit evidence as OTLP log records directly to the
  collector endpoint.

- **FR-007**: Plugins MUST emit evidence using the
  `GemaraEvidence` type, which maps Gemara `layer4.Metadata`
  and `layer4.AssessmentLog` fields to OTEL attributes:
  `policy.engine.name`, `compliance.control.id`,
  `compliance.control.catalog.id`, `policy.evaluation.result`,
  `policy.rule.id`, `compliance.assessment.id`.

- **FR-008**: When `--format otel` is active, complyctl MUST
  display an export summary after the scan summary, showing
  per-plugin export status (exported count, skipped, errors)
  in the same plain-text format as other complyctl output.
  When a plugin reports failures, the summary MUST include
  the plugin's error message below the table so the user
  can diagnose the issue without checking logs.

- **FR-009**: The workspace configuration (`complytime.yaml`)
  MUST support a `collector` section for specifying the
  Beacon collector endpoint and OIDC client credentials
  (`client-id`, `client-secret`, `token-endpoint`).
  complyctl MUST perform the OAuth2 client credentials
  token exchange and pass the resolved access token to
  plugins.

- **FR-010**: The `doctor` command MUST validate collector
  configuration when present — checking endpoint format
  and optionally verifying reachability (non-blocking
  warning if unreachable, consistent with registry check
  behavior).

- **FR-011**: When `--format otel` is used without a
  `collector` section in `complytime.yaml`, complyctl
  MUST exit with a clear error indicating no collector
  is configured and pointing to documentation.

- **FR-012**: When `--format otel` is NOT used, the scan
  command's behavior, output, and exit codes MUST remain
  unchanged. Export is strictly opt-in via the format flag.

- **FR-013**: Plugins that do not implement Export MUST NOT
  be required to change. The `supports_export` field
  defaults to `false` in the proto definition, ensuring
  backward compatibility with existing plugins.

### Key Entities

- **Beacon Collector**: A custom OpenTelemetry Collector
  distribution (`otelcol-beacon`) that receives OTLP logs,
  enriches them via TruthBeam, and exports to storage
  backends (Loki, S3) and reporting tools (Hyperproof).

- **ProofWatch**: A Go instrumentation library
  (`github.com/complytime/complytime-collector-components/proofwatch`) that
  emits compliance evidence as OTLP log records using the
  OpenTelemetry SDK. Implements the `Evidence` interface
  with `ToJSON()`, `Attributes()`, and `Timestamp()`.

- **GemaraEvidence**: A ProofWatch evidence type that wraps
  `layer4.Metadata` and `layer4.AssessmentLog` from
  go-gemara. Maps Gemara assessment fields to OTEL
  attributes for downstream enrichment by TruthBeam.

- **TruthBeam**: A custom OTEL Collector processor that
  enriches compliance evidence log records with additional
  context (control mappings, frameworks, risk levels)
  using Gemara content. Enrichment is transparent to
  complyctl — plugins emit evidence with the required
  attributes and TruthBeam handles the rest.

- **Export RPC**: A new gRPC method on the Plugin service
  that receives collector configuration and triggers
  evidence emission via ProofWatch. Called by the scan
  command when `--format otel` is used. Optional — plugins
  declare support via `Describe`.

### Assumptions

- The Beacon collector is deployed and accessible from the
  system where complyctl runs. Network connectivity between
  the plugin process and the collector is the user's
  responsibility.

- ProofWatch and the OTEL SDK handle transport (gRPC/HTTP),
  serialization (OTLP protobuf), and retry logic. Plugins
  do not implement custom transport.

- Enrichment is performed by TruthBeam in the collector
  pipeline. Plugins and complyctl are not responsible for
  enrichment — they emit evidence with the required Gemara
  attributes and TruthBeam adds compliance context
  downstream.

- The `GemaraEvidence` type in ProofWatch is sufficient for
  all current plugins. If a plugin needs to emit additional
  evidence formats (e.g., raw in-toto attestations alongside
  Gemara evidence), this can be addressed as a future
  extension.

- The `Export` RPC is called after `Scan` has completed
  within the same `complyctl scan --format otel` invocation.
  Plugins access their scan results from their in-memory
  state or workspace state.

- Authentication to the collector uses OIDC client
  credentials. complyctl owns the token exchange — it
  performs the OAuth2 client credentials handshake and
  passes the resulting access token to plugins. Plugins
  always receive a resolved bearer token, never raw
  client credentials. The Beacon distribution includes
  `oidcauthextension` for server-side token validation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can export scan results to a Beacon
  collector using
  `complyctl scan --policy-id <ID> --format otel` with no
  plugin-specific knowledge required. The flow works
  identically regardless of which plugin produced the
  results.

- **SC-002**: Evidence records arriving at the collector
  contain all six Gemara OTEL attributes
  (`policy.engine.name`, `compliance.control.id`,
  `compliance.control.catalog.id`, `policy.evaluation.result`,
  `policy.rule.id`, `compliance.assessment.id`), enabling
  TruthBeam enrichment without modification.

- **SC-003**: Existing plugins (OpenSCAP, AMPEL) that do not
  yet implement Export continue to function without any code
  changes. The `supports_export` field defaults to `false`.

- **SC-004**: The export summary appended after the scan
  summary provides per-plugin status (count exported,
  skipped, errors) in a single glanceable output,
  consistent with the scan summary format.

- **SC-005**: End-to-end evidence chain is verifiable: an
  auditor can match `compliance.assessment.id` in the
  collector storage backend (Loki/S3) to the `id` field in
  the complyctl-produced `evaluation-log-*.yaml` file.

- **SC-006**: Adding export support to a new plugin requires
  implementing only the `Export` RPC and adding ProofWatch
  as a dependency — no changes to complyctl core or other
  plugins.

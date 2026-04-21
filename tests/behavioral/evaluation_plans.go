// SPDX-License-Identifier: Apache-2.0

package behavioral

import (
	"strings"

	"github.com/gemaraproj/go-gemara"
)

// Plans maps each assessment requirement ID to its ordered step sequence.
// Reusable precondition steps (WriteConfig, SyncPolicy, InstallTestPlugin)
// prepare the shared BehavioralContext before the family-specific evaluator.
// InstallTestPlugin installs the test provider binary (see reusable_steps.go).
var Plans = map[string][]gemara.AssessmentStep{
	// CTRL01: Policy Content Signature Validation
	"CT.COMPLYCTL.CTRL01.AR01": {
		WriteConfig,
		SyncPolicy,
		SignatureVerified,
	},

	// CTRL02: OCI Artifact Digest Tracking
	"CT.COMPLYCTL.CTRL02.AR01": {
		WriteConfig,
		SyncPolicy,
		DigestRecordedInState,
	},
	"CT.COMPLYCTL.CTRL02.AR02": {
		WriteConfig,
		SyncPolicy,
		OCILayoutExists,
	},

	// CTRL03: Environment Variable Resolution Safety
	"CT.COMPLYCTL.CTRL03.AR01": {
		UnsetEnvVarFails,
	},
	"CT.COMPLYCTL.CTRL03.AR02": {
		EnvVarResolution,
	},

	// CTRL04: Provider Routing Isolation
	"CT.COMPLYCTL.CTRL04.AR01": {
		WriteConfig,
		InstallTestPlugin,
		SyncPolicy,
		MatchedEvaluatorRouting,
	},
	"CT.COMPLYCTL.CTRL04.AR02": {
		WriteConfig,
		InstallTestPlugin,
		SyncPolicy,
		EvaluatorMismatchRejected,
	},

	// CTRL05: Registry Transport Validation
	"CT.COMPLYCTL.CTRL05.AR01": {
		HTTPSchemeRejected,
	},
	"CT.COMPLYCTL.CTRL05.AR02": {
		HTTPSSchemeNoPlainHTTP,
	},

	// CTRL07: Provider Binary Integrity Verification (expected fail — not implemented)
	"CT.COMPLYCTL.CTRL07.AR01": {
		WriteConfig,
		InstallTestPlugin,
		SyncPolicy,
		PluginBinaryIntegrityCheck,
	},

	// CTRL08: Provider Subprocess Isolation (expected fail — not implemented)
	"CT.COMPLYCTL.CTRL08.AR01": {
		WriteConfig,
		InstallTestPlugin,
		PluginSubprocessIsolation,
	},

	// CTRL09: Log File Credential Redaction
	"CT.COMPLYCTL.CTRL09.AR01": {
		LogCredentialRedaction,
	},

	// CTRL06: Assessment Result Audit Trail
	"CT.COMPLYCTL.CTRL06.AR01": {
		WriteConfig,
		InstallTestPlugin,
		SyncPolicy,
		EvaluationLogProduced,
	},
	"CT.COMPLYCTL.CTRL06.AR02": {
		WriteConfig,
		InstallTestPlugin,
		SyncPolicy,
		OSCALResultProduced,
	},
}

// ControlForRequirement extracts the parent control ID from a requirement ID.
// "CT.COMPLYCTL.CTRL01.AR01" -> "CT.COMPLYCTL.CTRL01"
func ControlForRequirement(requirementID string) string {
	parts := strings.Split(requirementID, ".")
	if len(parts) < 4 {
		return requirementID
	}
	return strings.Join(parts[:3], ".")
}

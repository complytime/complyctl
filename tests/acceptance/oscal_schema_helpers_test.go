// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	oscalValidation "github.com/defenseunicorns/go-oscal/src/pkg/validation"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
)

// checkOSCALSchema validates every result and the document envelope independently
// of conversion. Evidence presence and linkage still need checkOSCALEvidence.
func checkOSCALSchema(path string) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("read OSCAL document %q: %w", path, err)
	}
	var document oscalTypes.OscalModels
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode OSCAL document %q: %w", path, err)
	}
	if document.AssessmentResults == nil {
		return fmt.Errorf("OSCAL document %q: assessment-results must be present", path)
	}
	// Validate the original JSON rather than reserializing the typed model:
	// isolation from the converter must also preserve unknown fields and nulls.
	validator, err := oscalValidation.NewValidatorDesiredVersion(data, oscalTypes.Version)
	if err != nil {
		return fmt.Errorf("create OSCAL %s schema validator for %q: %w",
			oscalTypes.Version, path, err)
	}
	if err := validator.Validate(); err != nil {
		return fmt.Errorf("validate OSCAL %s document %q: %w", oscalTypes.Version, path, err)
	}
	return nil
}

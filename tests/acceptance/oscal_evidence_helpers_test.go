// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

import "fmt"

const gemaraOSCALNamespace = "https://github.com/gemaraproj/go-gemara/ns/oscal"

type oscalEvidenceExpectation struct {
	ControlID     string
	RequirementID string
	Description   string
	ReferenceID   string
	Coordinate    string
	Digest        string
}

// checkOSCALEvidence inspects a decoded result, not schema validity. Optional
// OSCAL evidence can disappear while the document remains schema-valid.
func checkOSCALEvidence(result map[string]interface{}, expected oscalEvidenceExpectation) error {
	observation, err := linkedOSCALObservation(result, expected.ControlID, expected.RequirementID)
	if err != nil {
		return err
	}
	evidence, err := oscalObjects(observation, "relevant-evidence")
	if err != nil {
		return fmt.Errorf("requirement %q: %w", expected.RequirementID, err)
	}
	if len(evidence) == 0 {
		return fmt.Errorf("requirement %q: relevant-evidence must not be empty", expected.RequirementID)
	}
	// Match all fields on one item; combining fields from separate evidence
	// entries could hide provenance loss (Single Responsibility Principle).
	var mismatch error
	for _, item := range evidence {
		mismatch = checkOSCALEvidenceItem(item, expected)
		if mismatch == nil {
			return nil
		}
	}
	return fmt.Errorf("requirement %q: no matching relevant-evidence: %w",
		expected.RequirementID, mismatch)
}

func linkedOSCALObservation(result map[string]interface{}, controlID, requirementID string) (map[string]interface{}, error) {
	observations, err := oscalObjects(result, "observations")
	if err != nil {
		return nil, err
	}
	byUUID := make(map[string]map[string]interface{}, len(observations))
	for _, observation := range observations {
		uuid, _ := observation["uuid"].(string)
		if uuid == "" || byUUID[uuid] != nil {
			return nil, fmt.Errorf("observation UUID %q is empty or duplicated", uuid)
		}
		byUUID[uuid] = observation
	}
	findings, err := oscalObjects(result, "findings")
	if err != nil {
		return nil, err
	}
	for _, finding := range findings {
		target, _ := finding["target"].(map[string]interface{})
		if target["target-id"] != controlID {
			continue
		}
		links, err := oscalObjects(finding, "related-observations")
		if err != nil {
			return nil, fmt.Errorf("control %q: %w", controlID, err)
		}
		for _, link := range links {
			uuid, _ := link["observation-uuid"].(string)
			observation := byUUID[uuid]
			if observation == nil {
				return nil, fmt.Errorf("control %q: unresolved observation UUID %q", controlID, uuid)
			}
			if hasOSCALProperty(observation, "requirement-id", requirementID) {
				return observation, nil
			}
		}
		return nil, fmt.Errorf("control %q: no related observation for requirement %q",
			controlID, requirementID)
	}
	return nil, fmt.Errorf("finding for control %q not found", controlID)
}

func checkOSCALEvidenceItem(item map[string]interface{}, expected oscalEvidenceExpectation) error {
	if item["description"] != expected.Description {
		return fmt.Errorf("evidence description = %v, want %q", item["description"], expected.Description)
	}
	properties := []struct{ name, value string }{
		{"source-reference-id", expected.ReferenceID},
		{"source-coordinate", expected.Coordinate},
		{"source-digest", expected.Digest},
	}
	for _, property := range properties {
		if !hasOSCALProperty(item, property.name, property.value) {
			return fmt.Errorf("evidence property %q missing or mismatched, want %q",
				property.name, property.value)
		}
	}
	return nil
}

func hasOSCALProperty(object map[string]interface{}, name, value string) bool {
	properties, err := oscalObjects(object, "props")
	if err != nil {
		return false
	}
	for _, property := range properties {
		if property["ns"] == gemaraOSCALNamespace &&
			property["name"] == name && property["value"] == value {
			return true
		}
	}
	return false
}

func oscalObjects(object map[string]interface{}, field string) ([]map[string]interface{}, error) {
	items, ok := object[field].([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array", field)
	}
	objects := make([]map[string]interface{}, 0, len(items))
	for i, item := range items {
		object, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", field, i)
		}
		objects = append(objects, object)
	}
	return objects, nil
}

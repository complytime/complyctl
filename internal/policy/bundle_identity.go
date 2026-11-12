// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"

	"github.com/gemaraproj/go-gemara"
)

// Downstream provider/report maps use unqualified IDs. Reject ambiguous bundles
// at resolution rather than allowing input order to choose their meaning.
type catalogIdentities struct {
	catalogs     map[string]string
	controls     map[string]string
	requirements map[string]string
}

func newCatalogIdentities() *catalogIdentities {
	return &catalogIdentities{
		catalogs: make(map[string]string), controls: make(map[string]string),
		requirements: make(map[string]string),
	}
}

func registerBundleIdentity(seen map[string]string, kind, id, declaration string) error {
	if previous, ok := seen[id]; ok {
		return fmt.Errorf("ambiguous %s ID %q declared in %s and %s", kind, id, previous, declaration)
	}
	seen[id] = declaration
	return nil
}

func (ids *catalogIdentities) registerControlCatalog(id string, catalog *gemara.ControlCatalog, file string) error {
	if err := registerBundleIdentity(ids.catalogs, "catalog", id, file); err != nil {
		return err
	}
	for i, control := range catalog.Controls {
		location := fmt.Sprintf("%q catalog %q control[%d] %q", file, id, i, control.Id)
		if err := registerBundleIdentity(ids.controls, "control", control.Id, location); err != nil {
			return err
		}
		for j, requirement := range control.AssessmentRequirements {
			declaration := fmt.Sprintf("%s requirement[%d]", location, j)
			if err := registerBundleIdentity(ids.requirements, "requirement", requirement.Id, declaration); err != nil {
				return err
			}
		}
	}
	return nil
}

type mappingReferenceDeclaration struct {
	reference gemara.MappingReference
	artifact  string
}

func appendMappingReferences(graph *DependencyGraph, seen map[string]mappingReferenceDeclaration,
	references []gemara.MappingReference, artifact string,
) error {
	for _, ref := range references {
		if previous, ok := seen[ref.Id]; ok {
			if previous.reference != ref {
				return fmt.Errorf("conflicting mapping reference ID %q declared in %q and %q",
					ref.Id, previous.artifact, artifact)
			}
			continue
		}
		seen[ref.Id] = mappingReferenceDeclaration{reference: ref, artifact: artifact}
		graph.MappingReferences = append(graph.MappingReferences, ref)
	}
	return nil
}

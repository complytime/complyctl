package gemaraconv

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/gemaraproj/go-gemara"
)

// CatalogToOSCAL converts a Gemara ControlCatalog to OSCAL Catalog format.
func CatalogToOSCAL(catalog *gemara.ControlCatalog, opts ...GenerateOption) (oscal.Catalog, error) {
	options := generateOpts{}
	for _, opt := range opts {
		opt(&options)
	}
	options.completeFromCatalog(catalog)

	metadata, err := createMetadataFromCatalog(catalog, options)
	if err != nil {
		return oscal.Catalog{}, fmt.Errorf("error creating catalog metadata: %w", err)
	}

	// Determine control HREF format for control links
	controlHREF := options.controlHREF
	if controlHREF == "" {
		controlHREF = options.canonicalHref
	}

	oscalCatalog := oscal.Catalog{
		UUID:     uuid.NewUUID(),
		Groups:   nil,
		Metadata: metadata,
	}

	familyMap := make(map[string]gemara.Group)
	for _, family := range catalog.Families {
		familyMap[family.Id] = family
	}

	controlsByFamily := make(map[string][]gemara.Control)
	for _, control := range catalog.Controls {
		controlsByFamily[control.Family] = append(controlsByFamily[control.Family], control)
	}

	catalogGroups := []oscal.Group{}

	for _, family := range catalog.Families {
		controls := controlsByFamily[family.Id]
		if len(controls) == 0 {
			continue
		}

		group := oscal.Group{
			Class:    "family",
			Controls: nil,
			ID:       family.Id,
			Title:    strings.ReplaceAll(family.Description, "\n", "\\n"),
		}

		oscalControls := []oscal.Control{}
		for _, control := range controls {
			controlTitle := strings.TrimSpace(control.Title)

			newCtl := oscal.Control{
				Class: family.Id,
				ID:    control.Id,
				Title: strings.ReplaceAll(controlTitle, "\n", "\\n"),
				Parts: &[]oscal.Part{
					{
						Name:  "statement",
						ID:    fmt.Sprintf("%s_smt", control.Id),
						Prose: control.Objective,
					},
				},
				Links: func() *[]oscal.Link {
					if controlHREF != "" {
						return &[]oscal.Link{
							{
								Href: fmt.Sprintf(controlHREF, options.version, strings.ToLower(control.Id)),
								Rel:  "canonical",
							},
						}
					}
					return nil
				}(),
			}

			var subControls []oscal.Control
			for _, ar := range control.AssessmentRequirements {
				subControl := oscal.Control{
					ID:    ar.Id,
					Title: ar.Id,
					Parts: &[]oscal.Part{
						{
							Name:  "statement",
							ID:    fmt.Sprintf("%s_smt", ar.Id),
							Prose: ar.Text,
						},
					},
				}

				if ar.Recommendation != "" {
					*subControl.Parts = append(*subControl.Parts, oscal.Part{
						Name:  "guidance",
						ID:    fmt.Sprintf("%s_gdn", ar.Id),
						Prose: ar.Recommendation,
					})
				}

				*subControl.Parts = append(*subControl.Parts, oscal.Part{
					Name: "assessment-objective",
					ID:   fmt.Sprintf("%s_obj", ar.Id),
					Links: &[]oscal.Link{
						{
							Href: fmt.Sprintf("#%s_smt", ar.Id),
							Rel:  "assessment-for",
						},
					},
				})

				subControls = append(subControls, subControl)
			}

			if len(subControls) > 0 {
				newCtl.Controls = &subControls
			}
			oscalControls = append(oscalControls, newCtl)
		}

		group.Controls = &oscalControls
		catalogGroups = append(catalogGroups, group)
	}
	oscalCatalog.Groups = &catalogGroups

	return oscalCatalog, nil
}

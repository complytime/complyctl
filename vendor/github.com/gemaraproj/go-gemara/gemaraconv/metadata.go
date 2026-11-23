package gemaraconv

import (
	"fmt"
	"net/url"
	"time"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/gemaraproj/go-gemara"
	oscalUtils "github.com/gemaraproj/go-gemara/internal/oscal"
)

// partyUUIDSource returns the deterministic UUID source for an author.
// Actor.Id is the identity key: a renamed actor keeps its UUID, and two
// actors sharing a name do not collide. Documents that omit the id (the
// schema requires it, but nothing enforces it) fall back to name and type.
func partyUUIDSource(author gemara.Actor) string {
	if author.Id != "" {
		return fmt.Sprintf("gemara:%s/party", url.PathEscape(author.Id))
	}
	return fmt.Sprintf("gemara:%s/%s/party", url.PathEscape(author.Name), url.PathEscape(author.Type.String()))
}

// createMetadata creates OSCAL metadata with common fields and optional author information
func createMetadata(title string, version string, published *time.Time, canonicalHref string, author gemara.Actor) oscal.Metadata {
	now := time.Now()
	version = defaultVersion(version)
	metadata := oscal.Metadata{
		Title:        title,
		OscalVersion: oscal.Version,
		Version:      version,
		Published:    published,
		LastModified: now,
	}

	if canonicalHref != "" {
		metadata.Links = &[]oscal.Link{
			{
				Href: canonicalHref,
				Rel:  "canonical",
			},
		}
	}

	if author.Name != "" {
		authorRole := oscal.Role{
			ID:          "author",
			Description: "Author and owner of the document",
			Title:       "Author",
		}

		party := oscal.Party{
			UUID: uuid.NewUUIDWithSource(partyUUIDSource(author)),
			Type: mapPartyType(author.Type),
			Name: author.Name,
		}

		responsibleParty := oscal.ResponsibleParty{
			PartyUuids: []string{party.UUID},
			RoleId:     authorRole.ID,
		}

		metadata.Parties = &[]oscal.Party{party}
		metadata.Roles = &[]oscal.Role{authorRole}
		metadata.ResponsibleParties = &[]oscal.ResponsibleParty{responsibleParty}
	}

	return metadata
}

// createMetadataFromGuidance creates OSCAL metadata from a GuidanceCatalog
func createMetadataFromGuidance(guidance gemara.GuidanceCatalog, opts generateOpts) (oscal.Metadata, error) {
	canonicalHref := ""
	if opts.canonicalHref != "" {
		canonicalHref = fmt.Sprintf(opts.canonicalHref, opts.version)
	}

	published := oscalUtils.GetTime(string(guidance.Metadata.Date))
	metadata := createMetadata(
		guidance.Title,
		opts.version,
		published,
		canonicalHref,
		guidance.Metadata.Author,
	)

	return metadata, nil
}

// createMetadataFromCatalog creates OSCAL metadata from a ControlCatalog
func createMetadataFromCatalog(catalog gemara.ControlCatalog, opts generateOpts) (oscal.Metadata, error) {
	// Handle canonical HREF - prefer controlHREF for Catalog, fallback to canonicalHref
	var canonicalHref string
	if opts.controlHREF != "" {
		// controlHREF format: "https://example.com/versions/%s#%s" (version, controlID)
		canonicalHref = fmt.Sprintf(opts.controlHREF, opts.version, "")
	} else if opts.canonicalHref != "" {
		// canonicalHref format: "https://example.com/versions/%s" (version only)
		canonicalHref = fmt.Sprintf(opts.canonicalHref, opts.version)
	}

	published := oscalUtils.GetTime(string(catalog.Metadata.Date))
	metadata := createMetadata(
		catalog.Title,
		opts.version,
		published,
		canonicalHref,
		catalog.Metadata.Author,
	)

	return metadata, nil
}

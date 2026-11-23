// SPDX-License-Identifier: Apache-2.0

package gemaraconv

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/gemaraproj/go-gemara"
	oscalUtils "github.com/gemaraproj/go-gemara/internal/oscal"
)

// EvaluationLogToOSCALAssessmentResults converts a Gemara EvaluationLog into an
// OSCAL Assessment Results document.
//
// Use WithImportApHref to set the assessment plan reference (defaults to "#").
// Use WithCatalog to enrich findings with control titles and requirement text.
//
// Returns an error when the log's metadata id or target id is empty, when two
// control evaluations share the same control mapping (reference id and entry
// id), or when mapping references conflict on title or URL for the same ID:
// each would break the deterministic UUID scheme or the back-matter it
// generates.
func EvaluationLogToOSCALAssessmentResults(log gemara.EvaluationLog, opts ...EvalOption) (oscal.AssessmentResults, error) {
	options := defaultEvalOpts()
	for _, opt := range opts {
		opt(&options)
	}

	// The metadata id and target id key identity-bearing assembly UUIDs;
	// logs that omit them would silently share finding and inventory-item
	// UUIDs across unrelated documents, the same authoring-mistake class
	// as a duplicate control evaluation.
	if log.Metadata.Id == "" {
		return oscal.AssessmentResults{}, fmt.Errorf("evaluation log for target %s: metadata id is required", log.Target.Id)
	}
	if log.Target.Id == "" {
		return oscal.AssessmentResults{}, fmt.Errorf("evaluation log %s: target id is required", log.Metadata.Id)
	}

	seenControls := make(map[controlIdentity]struct{}, len(log.Evaluations))
	for _, eval := range log.Evaluations {
		if eval == nil {
			continue
		}
		id := controlIdentity{referenceId: eval.Control.ReferenceId, entryId: eval.Control.EntryId}
		if _, exists := seenControls[id]; exists {
			name := eval.Control.EntryId
			if eval.Control.ReferenceId != "" {
				name = eval.Control.ReferenceId + "/" + eval.Control.EntryId
			}
			return oscal.AssessmentResults{}, fmt.Errorf("evaluation log %s: duplicate control evaluation for control %s", log.Metadata.Id, name)
		}
		seenControls[id] = struct{}{}
	}

	// The metadata party, the origin actors on findings and observations,
	// and the logged-by entry all gate on a non-empty author name — the
	// same gate createAssessmentResultsMetadata and createMetadata use to
	// emit the party. An author with no name (with or without an id) has no
	// party in the document, so no reference is generated either: deriving
	// a UUID anyway would leave origins and logged-by entries pointing at
	// a party that does not exist, and for anonymous authors it would
	// collapse every actor of the same entity type onto one shared party.
	authorPartyUUID := ""
	if log.Metadata.Author.Name != "" {
		authorPartyUUID = uuid.NewUUIDWithSource(partyUUIDSource(log.Metadata.Author))
	}
	version := defaultVersion(log.Metadata.Version)
	// A single now feeds the date segment, the Result start, and
	// last-modified, so a conversion that straddles UTC midnight cannot
	// emit a document whose UUIDs and timestamps disagree. nowFunc (not
	// time.Now directly) lets tests pin the clock: determinism tests
	// convert the same log twice and must agree on the date segment.
	now := nowFunc()
	metadata := createAssessmentResultsMetadata(log, authorPartyUUID, version, now)

	backMatter, mappingResourceUUIDs, err := mappingToBackMatterWithUUIDs(log.Metadata.MappingReferences)
	if err != nil {
		return oscal.AssessmentResults{}, fmt.Errorf("evaluation log %s: %w", log.Metadata.Id, err)
	}
	dateSegment := assessmentDateSegment(string(log.Metadata.Date), now)
	result := evaluationLogToResult(log, options.catalog, authorPartyUUID, mappingResourceUUIDs, log.Metadata.Id, dateSegment, now)

	return oscal.AssessmentResults{
		UUID:       uuid.NewUUIDWithSource(fmt.Sprintf("gemara:%s/%s/%s/assessment-results", url.PathEscape(log.Metadata.Id), url.PathEscape(version), url.PathEscape(dateSegment))),
		Metadata:   metadata,
		ImportAp:   oscal.ImportAp{Href: options.importApHref},
		Results:    []oscal.Result{result},
		BackMatter: backMatter,
	}, nil
}

// controlIdentity identifies a control evaluation the way the schema does:
// by its mapping's reference id and entry id together. Two catalogs can map
// the same entry id, so the entry id alone is not unique.
type controlIdentity struct {
	referenceId string
	entryId     string
}

// nowFunc is the conversion clock, kept as a variable so tests can pin it
// without changing the exported API: determinism tests convert the same log
// twice and must see the same date segment even if the run straddles UTC
// midnight.
var nowFunc = time.Now

// mappingSource renders a mapping's identity with a fixed marker per field.
// The ref= and entry= markers are always emitted, empty when unset, so an
// optional field can never shift the segment boundaries of a UUID source
// string: a control with no reference and a control whose reference holds
// what the first one's entry holds cannot render alike. Values are escaped,
// so a "/" inside an id cannot be read as a separator either.
func mappingSource(c gemara.EntryMapping) string {
	return "ref=" + url.PathEscape(c.ReferenceId) + "/entry=" + url.PathEscape(c.EntryId)
}

// planSource renders an assessment log's plan mapping for its UUID source
// segment and occurrence key: empty when no plan is recorded, the mapping's
// marked identity otherwise. A recorded plan distinguishes logs that
// evaluate the same requirement under different plans, leaving the
// occurrence counter to break only the remaining ties.
func planSource(p *gemara.EntryMapping) string {
	if p == nil {
		return ""
	}
	return mappingSource(*p)
}

// assessmentKey identifies what an assessment log evaluates: the control
// (mapping reference plus entry id), the requirement, and, when recorded,
// the plan it executed. Assessment logs that share a key are distinguished
// by an occurrence counter rather than identity fields alone.
type assessmentKey struct {
	controlId     string
	requirementId string
	plan          string
}

// assessmentDateSegment returns the date component of the AssessmentResults
// root and Result UUID sources. An explicit metadata.Date is used verbatim;
// an empty one falls back to the conversion date so the per-run tier still
// rotates across days instead of freezing every empty-date log onto the same
// shared root and Result UUIDs.
func assessmentDateSegment(date string, now time.Time) string {
	if date != "" {
		return date
	}
	return now.UTC().Format("2006-01-02")
}

func evaluationLogToResult(log gemara.EvaluationLog, catalog *gemara.ControlCatalog, authorPartyUUID string, mappingResourceUUIDs map[string]string, logId string, dateSegment string, now time.Time) oscal.Result {
	start := oscalUtils.GetTimeWithFallback(string(log.Metadata.Date), now)

	origins := buildOrigins(log.Metadata.Author, authorPartyUUID)

	controlIds := collectControlIds(log)
	reviewedControls := buildReviewedControls(controlIds)

	var observations []oscal.Observation
	var findings []oscal.Finding
	var logEntries []oscal.AssessmentLogEntry

	// Occurrence counter for assessment logs sharing the same
	// (control, requirement, plan) key. OSCAL requires per-document UUID
	// uniqueness, and a requirement can be evaluated in more than one way.
	occurrences := make(map[assessmentKey]int)

	for _, eval := range log.Evaluations {
		if eval == nil {
			continue
		}

		finding := buildFinding(eval, catalog, origins, logId)
		var relatedObs []oscal.RelatedObservation

		for _, alog := range eval.AssessmentLogs {
			if alog == nil {
				continue
			}

			key := assessmentKey{controlId: mappingSource(eval.Control), requirementId: alog.Requirement.EntryId, plan: planSource(alog.Plan)}
			occurrences[key]++

			obs := buildObservation(alog, eval, origins, start, mappingResourceUUIDs, logId, occurrences[key])
			observations = append(observations, obs)
			relatedObs = append(relatedObs, oscal.RelatedObservation{ObservationUuid: obs.UUID})

			entry := buildLogEntry(alog, eval, authorPartyUUID, start, logId, occurrences[key])
			logEntries = append(logEntries, entry)
		}

		if len(relatedObs) > 0 {
			finding.RelatedObservations = &relatedObs
		}
		findings = append(findings, finding)
	}

	title := fmt.Sprintf("Evaluation: %s", log.Metadata.Id)

	targetItem := buildTargetInventoryItem(log.Target)
	localDefs := oscal.LocalDefinitions{
		InventoryItems: &[]oscal.InventoryItem{targetItem},
	}

	result := oscal.Result{
		UUID:             uuid.NewUUIDWithSource(fmt.Sprintf("gemara:%s/%s/result", url.PathEscape(logId), url.PathEscape(dateSegment))),
		Title:            title,
		Description:      fmt.Sprintf("Results from Gemara EvaluationLog %s", log.Metadata.Id),
		Start:            start,
		ReviewedControls: reviewedControls,
		Observations:     oscalUtils.NilIfEmpty(observations),
		Findings:         oscalUtils.NilIfEmpty(findings),
		LocalDefinitions: &localDefs,
		Props: &[]oscal.Property{
			{
				Name:  "aggregate-result",
				Value: log.Result.String(),
				Ns:    oscalUtils.GemaraNamespace,
			},
		},
	}

	if len(logEntries) > 0 {
		result.AssessmentLog = &oscal.AssessmentLog{Entries: logEntries}
	}

	return result
}

func createAssessmentResultsMetadata(log gemara.EvaluationLog, authorPartyUUID string, version string, now time.Time) oscal.Metadata {
	published := oscalUtils.GetTime(string(log.Metadata.Date))

	metadata := oscal.Metadata{
		Title:        fmt.Sprintf("Assessment Results: %s", log.Metadata.Id),
		OscalVersion: oscal.Version,
		Version:      version,
		Published:    published,
		LastModified: now,
	}

	if log.Metadata.Author.Name != "" {
		authorRole := oscal.Role{
			ID:    "assessor",
			Title: "Assessor",
		}
		party := oscal.Party{
			UUID: authorPartyUUID,
			Type: mapPartyType(log.Metadata.Author.Type),
			Name: log.Metadata.Author.Name,
		}
		metadata.Roles = &[]oscal.Role{authorRole}
		metadata.Parties = &[]oscal.Party{party}
		metadata.ResponsibleParties = &[]oscal.ResponsibleParty{
			{RoleId: authorRole.ID, PartyUuids: []string{party.UUID}},
		}
	}

	return metadata
}

// buildOrigins returns the origin list for the log's author, or nil when the
// author has no name: the party is emitted only for named authors, and OSCAL
// requires an origin to carry at least one actor referencing a party UUID,
// so a name-less author (with or without an id) has no party to reference.
func buildOrigins(author gemara.Actor, partyUUID string) *[]oscal.Origin {
	if partyUUID == "" {
		return nil
	}
	return &[]oscal.Origin{
		{
			Actors: []oscal.OriginActor{
				{
					ActorUuid: partyUUID,
					Type:      mapActorType(author.Type),
					RoleId:    "assessor",
				},
			},
		},
	}
}

func buildReviewedControls(controlIds []string) oscal.ReviewedControls {
	selectors := make([]oscal.AssessedControlsSelectControlById, 0, len(controlIds))
	for _, id := range controlIds {
		selectors = append(selectors, oscal.AssessedControlsSelectControlById{ControlId: id})
	}
	return oscal.ReviewedControls{
		ControlSelections: []oscal.AssessedControls{
			{IncludeControls: oscalUtils.NilIfEmpty(selectors)},
		},
	}
}

func buildFinding(eval *gemara.ControlEvaluation, catalog *gemara.ControlCatalog, origins *[]oscal.Origin, logId string) oscal.Finding {
	controlId := eval.Control.EntryId
	status := mapResultToObjectiveStatus(eval.Result)

	description := eval.Message
	if description == "" {
		description = fmt.Sprintf("Evaluation of control %s", controlId)
	}

	title := eval.Name
	if catalog != nil {
		control, _ := findControlAndRequirement(catalog, controlId, "")
		if control != nil && control.Title != "" {
			title = control.Title
		}
	}
	if title == "" {
		title = controlId
	}
	title = sanitizeTitle(title)

	return oscal.Finding{
		UUID:        uuid.NewUUIDWithSource(fmt.Sprintf("gemara:%s/%s/finding", url.PathEscape(logId), mappingSource(eval.Control))),
		Title:       title,
		Description: description,
		Origins:     origins,
		Target: oscal.FindingTarget{
			Type:     "objective-id",
			TargetId: controlId,
			Status:   status,
		},
	}
}

func buildObservation(alog *gemara.AssessmentLog, eval *gemara.ControlEvaluation, origins *[]oscal.Origin, fallback time.Time, mappingResourceUUIDs map[string]string, logId string, occurrence int) oscal.Observation {
	collected := oscalUtils.GetTimeWithFallback(string(alog.Start), fallback)

	description := alog.Description
	if description == "" {
		description = alog.Message
	}
	if description == "" {
		description = fmt.Sprintf("Assessment of requirement %s", alog.Requirement.EntryId)
	}

	methods := []string{"EXAMINE"}
	if alog.StepsExecuted > 0 {
		methods = []string{"TEST"}
	}

	obs := oscal.Observation{
		UUID:        uuid.NewUUIDWithSource(assessmentSource(logId, eval, alog, occurrence) + "/observation"),
		Description: description,
		Collected:   collected,
		Methods:     methods,
		Origins:     origins,
		Props: &[]oscal.Property{
			{
				Name:  "result",
				Value: alog.Result.String(),
				Ns:    oscalUtils.GemaraNamespace,
			},
			{
				Name:  "requirement-id",
				Value: alog.Requirement.EntryId,
				Ns:    oscalUtils.GemaraNamespace,
			},
			{
				Name:  "control-id",
				Value: eval.Control.EntryId,
				Ns:    oscalUtils.GemaraNamespace,
			},
		},
	}

	if alog.Recommendation != "" {
		obs.Remarks = alog.Recommendation
	}

	if len(alog.Evidence) > 0 {
		evidence := buildRelevantEvidence(alog.Evidence, mappingResourceUUIDs)
		obs.RelevantEvidence = &evidence
	}

	return obs
}

func buildRelevantEvidence(evidence []gemara.Evidence, mappingResourceUUIDs map[string]string) []oscal.RelevantEvidence {
	relevantEvidence := make([]oscal.RelevantEvidence, 0, len(evidence))
	for _, item := range evidence {
		var props []oscal.Property
		if item.Id != "" {
			props = append(props, oscal.Property{Name: "id", Value: item.Id, Ns: oscalUtils.GemaraNamespace})
		}
		if item.Type != "" {
			props = append(props, oscal.Property{Name: "type", Value: string(item.Type), Ns: oscalUtils.GemaraNamespace})
		}
		if item.CollectedAt != "" {
			props = append(props, oscal.Property{Name: "collected-at", Value: string(item.CollectedAt), Ns: oscalUtils.GemaraNamespace})
		}
		if item.Source.ReferenceId != "" {
			props = append(props, oscal.Property{Name: "source-reference-id", Value: item.Source.ReferenceId, Ns: oscalUtils.GemaraNamespace})
		}
		if item.Source.Coordinate != "" {
			props = append(props, oscal.Property{Name: "source-coordinate", Value: item.Source.Coordinate, Ns: oscalUtils.GemaraNamespace})
		}
		if item.Source.EntryId != "" {
			props = append(props, oscal.Property{Name: "source-entry-id", Value: item.Source.EntryId, Ns: oscalUtils.GemaraNamespace})
		}
		if item.Source.Digest != "" {
			props = append(props, oscal.Property{Name: "source-digest", Value: item.Source.Digest, Ns: oscalUtils.GemaraNamespace})
		}
		itemEvidence := oscal.RelevantEvidence{
			Description: item.Description,
			Props:       oscalUtils.NilIfEmpty(props),
			Remarks:     item.Source.Remarks,
		}
		if item.Source.ReferenceId != "" {
			if resourceUUID, found := mappingResourceUUIDs[item.Source.ReferenceId]; found {
				itemEvidence.Links = &[]oscal.Link{{Href: "#" + resourceUUID, Rel: "resource"}}
			}
		}
		relevantEvidence = append(relevantEvidence, itemEvidence)
	}
	return relevantEvidence
}

// assessmentSource builds the shared identity segment of the observation and
// log-entry UUID sources: log id, control mapping, requirement, recorded
// plan, and occurrence counter. Every field carries a fixed marker (see
// mappingSource) and the plan= slot is always emitted, so segment boundaries
// cannot shift when a field is empty: a control with no reference cannot
// render like a control whose reference holds what the first one's entry
// holds. A recorded plan distinguishes logs that evaluate the same
// requirement under different plans without spending counter values on them;
// the counter breaks only ties among logs sharing the same control,
// requirement, and plan.
func assessmentSource(logId string, eval *gemara.ControlEvaluation, alog *gemara.AssessmentLog, occurrence int) string {
	return fmt.Sprintf("gemara:%s/%s/req=%s/plan=%s/%d",
		url.PathEscape(logId), mappingSource(eval.Control), url.PathEscape(alog.Requirement.EntryId), planSource(alog.Plan), occurrence)
}

func buildLogEntry(alog *gemara.AssessmentLog, eval *gemara.ControlEvaluation, partyUUID string, fallback time.Time, logId string, occurrence int) oscal.AssessmentLogEntry {
	start := oscalUtils.GetTimeWithFallback(string(alog.Start), fallback)

	title := fmt.Sprintf("%s / %s", eval.Control.EntryId, alog.Requirement.EntryId)

	entry := oscal.AssessmentLogEntry{
		UUID:        uuid.NewUUIDWithSource(assessmentSource(logId, eval, alog, occurrence) + "/log-entry"),
		Title:       title,
		Start:       start,
		Description: alog.Description,
	}

	// LoggedBy must reference a party UUID, and the party is emitted only
	// for named authors; an author with no name (with or without an id) has
	// none, so the entry is logged by no one rather than by a phantom party.
	if partyUUID != "" {
		entry.LoggedBy = &[]oscal.LoggedBy{
			{PartyUuid: partyUUID, RoleId: "assessor"},
		}
	}

	if end := oscalUtils.GetTime(string(alog.End)); end != nil {
		entry.End = end
	}

	return entry
}

func buildTargetInventoryItem(target gemara.Resource) oscal.InventoryItem {
	description := target.Description
	if description == "" {
		description = target.Name
	}

	return oscal.InventoryItem{
		UUID:        uuid.NewUUIDWithSource(fmt.Sprintf("gemara:%s/inventory-item", url.PathEscape(target.Id))),
		Description: description,
		Props: &[]oscal.Property{
			{
				Name:  "gemara-resource-id",
				Value: target.Id,
				Ns:    oscalUtils.GemaraNamespace,
			},
			{
				Name:  "name",
				Value: target.Name,
				Ns:    oscalUtils.GemaraNamespace,
			},
		},
	}
}

func collectControlIds(log gemara.EvaluationLog) []string {
	seen := make(map[string]struct{})
	var ids []string
	for _, eval := range log.Evaluations {
		if eval == nil {
			continue
		}
		id := eval.Control.EntryId
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

// mapResultToObjectiveStatus maps a Gemara result to an OSCAL objective status.
// OSCAL types the reason as a token, so Result.String() ("Not Run") cannot be
// used. OSCAL suggests pass, fail and other but accepts any token, so results
// other than Passed and Failed get a descriptive reason. A value outside the
// Result enum cannot come from a valid EvaluationLog, so it is "invalid".
func mapResultToObjectiveStatus(r gemara.Result) oscal.ObjectiveStatus {
	switch r {
	case gemara.Passed:
		return oscal.ObjectiveStatus{State: "satisfied"}
	case gemara.Failed:
		return oscal.ObjectiveStatus{State: "not-satisfied"}
	case gemara.NotRun:
		return oscal.ObjectiveStatus{State: "not-satisfied", Reason: "not-run"}
	case gemara.NeedsReview:
		return oscal.ObjectiveStatus{State: "not-satisfied", Reason: "needs-review"}
	case gemara.NotApplicable:
		return oscal.ObjectiveStatus{State: "not-satisfied", Reason: "not-applicable"}
	case gemara.Unknown:
		return oscal.ObjectiveStatus{State: "not-satisfied", Reason: "unknown"}
	default:
		return oscal.ObjectiveStatus{State: "not-satisfied", Reason: "invalid"}
	}
}

// sanitizeTitle collapses newlines and surrounding whitespace into a single
// space so the value satisfies the OSCAL StringDatatype pattern (^[^\n]+$).
func sanitizeTitle(s string) string {
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}

func mapActorType(t gemara.EntityType) string {
	switch t {
	case gemara.Human:
		return "person"
	default:
		return "tool"
	}
}

func mapPartyType(t gemara.EntityType) string {
	switch t {
	case gemara.Human:
		return "person"
	default:
		return "organization"
	}
}

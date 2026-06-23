package gemara

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"time"
)

// AssessmentStep is a function type that inspects the provided targetData and returns a Result with a message and confidence level.
// The message may be an error string or other descriptive text.
type AssessmentStep func(payload interface{}) (Result, string, ConfidenceLevel)

// EvidenceCollector is an embeddable helper that gives a targetData payload the
// well-known evidence location and satisfies HasEvidence via method promotion,
// so the consumer writes no methods. Embed it in a payload struct:
//
//	type myTarget struct {
//		gemara.EvidenceCollector
//		Config SomeConfig
//	}
//
//	t := &myTarget{Config: cfg}   // pointer, so step mutations propagate
//	t.AddEvidence(ev)             // inside a step; may be called more than once
//	assessment.Run(t)             // copies evidence into assessment.Evidence,
//	                              // clearing the payload after each step
type EvidenceCollector struct {
	// Long name to avoid any possibility of collision
	gemaraStepEvidence []Evidence
}

// GetEvidence returns the evidence recorded since the last clear.
func (e *EvidenceCollector) GetEvidence() []Evidence {
	return e.gemaraStepEvidence
}

// AddEvidence appends a piece of evidence, so a single step may record evidence
// more than once. The accumulated evidence is copied into the log and cleared
// after each step runs.
func (e *EvidenceCollector) AddEvidence(evidence Evidence) {
	e.gemaraStepEvidence = append(e.gemaraStepEvidence, evidence)
}

// ClearEvidence empties the evidence location. The assessment calls it after
// copying each step's evidence into the log, so evidence does not linger
// in memory or get re-copied by a later step that records nothing.
func (e *EvidenceCollector) ClearEvidence() {
	e.gemaraStepEvidence = nil
}

// HasEvidence is the well-known interface a targetData payload may implement to
// surface Evidence collected while an assessment runs. A step records evidence
// into the payload (typically by mutating a shared field, which requires the
// payload to be passed by reference), and the AssessmentLog harvests it from
// this single location after running.
//
// Implementing the interface is optional: a payload that does not implement it
// simply contributes no evidence, which is not an error.
type HasEvidence interface {
	// GetEvidence returns the evidence recorded since the last clear.
	GetEvidence() []Evidence

	// AddEvidence appends a piece of evidence; it may be called more than once
	// within a single step.
	AddEvidence(evidence Evidence)

	// ClearEvidence empties the evidence location. The assessment calls it after
	// copying each step's evidence into the log, so a later step that records
	// nothing does not cause this step's evidence to be re-copied.
	ClearEvidence()
}

func (as AssessmentStep) String() string {
	// Get the function pointer correctly
	fn := runtime.FuncForPC(reflect.ValueOf(as).Pointer())
	if fn == nil {
		return "<unknown function>"
	}
	return fn.Name()
}

func (as AssessmentStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(as.String())
}

func (as AssessmentStep) MarshalYAML() (interface{}, error) {
	return as.String(), nil
}

// NewAssessment creates a new AssessmentLog object and returns a pointer to it.
func NewAssessment(requirementId string, description string, applicability []string, steps []AssessmentStep) (*AssessmentLog, error) {
	a := &AssessmentLog{
		Requirement: EntryMapping{
			EntryId: requirementId,
		},
		Description:   description,
		Applicability: applicability,
		Result:        NotRun,
		Steps:         steps,
	}
	err := a.precheck()
	return a, err
}

// AddStep queues a new step in the AssessmentLog
func (a *AssessmentLog) AddStep(step AssessmentStep) {
	a.Steps = append(a.Steps, step)
}

func (a *AssessmentLog) runStep(targetData interface{}, step AssessmentStep) Result {
	a.StepsExecuted++
	result, message, confidence := step(targetData)
	a.Result = UpdateAggregateResult(a.Result, result)

	// Move any evidence the step recorded from the payload into the log: copy it
	// out, then clear the payload. The clear is required so a later step that
	// records nothing does not leave this step's evidence in place for the next
	// harvest to re-copy as a duplicate.
	if provider, ok := targetData.(HasEvidence); ok {
		a.Evidence = append(a.Evidence, provider.GetEvidence()...)
		provider.ClearEvidence()
	}

	// Always update message to show what steps have been run and their context.
	a.Message = message

	// Always use the confidence level from the last step executed.
	// This gives step implementers full control over how confidence builds
	// as steps are executed, allowing them to adapt confidence based on
	// the cumulative context of all previous steps.
	a.ConfidenceLevel = confidence

	return result
}

// Run will execute all steps, halting if any step does not return Passed.
func (a *AssessmentLog) Run(targetData interface{}) Result {
	a.Result = NotRun

	a.Start = Datetime(time.Now().Format(time.RFC3339))
	err := a.precheck()
	if err != nil {
		a.Result = Unknown
		a.ConfidenceLevel = Undetermined
		return a.Result
	}
	for _, step := range a.Steps {
		if a.runStep(targetData, step) == Failed {
			return Failed
		}
	}
	a.End = Datetime(time.Now().Format(time.RFC3339))
	return a.Result
}

// precheck verifies that the assessment has all the required fields.
// It returns an error if the assessment is not valid.
func (a *AssessmentLog) precheck() error {
	if a.Requirement.EntryId == "" || a.Description == "" || a.Applicability == nil || a.Steps == nil || len(a.Applicability) == 0 || len(a.Steps) == 0 {
		message := fmt.Sprintf(
			"expected all AssessmentLog fields to have a value, but got: requirementId=len(%v), description=len=(%v), applicability=len(%v), steps=len(%v)",
			len(a.Requirement.EntryId), len(a.Description), len(a.Applicability), len(a.Steps),
		)
		a.Result = Unknown
		a.Message = message
		a.ConfidenceLevel = Undetermined
		return errors.New(message)
	}

	return nil
}

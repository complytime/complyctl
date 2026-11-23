package gemara

import "fmt"

// AddAssessment creates a new AssessmentLog object and adds it to the ControlEvaluation.
func (c *ControlEvaluation) AddAssessment(requirementId string, description string, applicability []string, steps []AssessmentStep) (assessment *AssessmentLog) {
	assessment, err := NewAssessment(requirementId, description, applicability, steps)
	if err != nil {
		c.Result = Failed
		c.Message = err.Error()
	}
	c.AssessmentLogs = append(c.AssessmentLogs, assessment)
	return
}

// Evaluate runs each step in each assessment, updating the relevant fields on the control evaluation.
// Every applicable assessment (sub-requirement) is evaluated independently; a failing sub-requirement
// must not suppress evaluation of its siblings. The control's aggregate result is accumulated via
// UpdateAggregateResult, so an earlier Failed still wins the rollup without skipping later assessments.
// Message retains the first assessment message that establishes the aggregate result's current severity;
// tied results do not replace it. Each assessment's own result and message remain in AssessmentLogs so
// consumers can report every condition rather than relying only on the control-level summary.
// Consequently, a later sibling's steps may produce side effects after an earlier assessment fails;
// each assessment retains its own step-level fail-fast behavior.
// An assessment whose applicability matches none of the userApplicability values is not run. It is
// recorded as NotApplicable with a message naming both lists, and rolls into the control result like
// any other assessment, so a control where nothing applies is NotApplicable.
// The targetData is the data that the assessment will be run against. The userApplicability is a slice
// of strings that determine when the assessment is applicable.
func (c *ControlEvaluation) Evaluate(targetData interface{}, userApplicability []string) {
	// Reset everything Evaluate accumulates, so a second evaluation does not
	// compound its results with the first: Failed is sticky in
	// UpdateAggregateResult, and a first-run message would otherwise persist.
	c.Result = NotRun
	c.Message = ""

	if len(c.AssessmentLogs) == 0 {
		c.Result = NeedsReview
		return
	}
	for _, assessment := range c.AssessmentLogs {
		var applicable bool
		for _, aa := range assessment.Applicability {
			for _, ua := range userApplicability {
				if aa == ua {
					applicable = true
					break
				}
			}
		}
		var result Result
		if applicable {
			result = assessment.Run(targetData)
		} else {
			assessment.Result = NotApplicable
			assessment.Message = fmt.Sprintf("Not applicable: requirement applies to %v, evaluation requested %v", assessment.Applicability, userApplicability)
			result = assessment.Result
		}
		aggregateResult := UpdateAggregateResult(c.Result, result)
		if aggregateResult != c.Result {
			c.Message = assessment.Message
		}
		c.Result = aggregateResult
	}
}

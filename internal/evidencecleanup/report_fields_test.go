// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateReportFields(t *testing.T) {
	for _, data := range []string{
		`{"evaluations":[],"metadata":{"id":"policy"}}`,
		`[{"id":"one"},{"id":"two"}]`,
		`{"first":{"id":"one"},"second":{"Id":"two"}}`,
	} {
		require.NoError(t, validateReportFields([]byte(data)))
	}
	for _, data := range []string{
		`{"evaluations":[],"Evaluations":[]}`,
		`{"Evaluations":[],"evaluations":[]}`,
		`{"evaluations":[{"evidence":[],"Evidence":[]}]}`,
		`{"source":{"coordinate":"first","Coordinate":"second"}}`,
		`{"k":1,"K":2}`,
		`{"s":1,"ſ":2}`,
		`{"evaluations":`,
	} {
		require.Error(t, validateReportFields([]byte(data)), data)
	}
}

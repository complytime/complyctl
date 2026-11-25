// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package evidencecleanup

import (
	"fmt"

	"github.com/complytime/complyctl/internal/workspacecoord"
)

func executeGroups(_ *workspacecoord.Access, _ *GroupResult, _ *Result) error {
	return fmt.Errorf("confined cleanup execution requires Linux or macOS")
}

func preparePreviewGroups(_ *workspacecoord.Access, _ *GroupResult) error {
	return fmt.Errorf("confined cleanup preview requires Linux or macOS")
}

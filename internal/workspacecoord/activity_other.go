// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package workspacecoord

// BeginActivity is unavailable where workspace coordination is unsupported.
func (a *Access) BeginActivity(ActivityOptions) (*Activity, error) {
	return nil, platformError()
}

// CompleteNormal is unavailable where workspace coordination is unsupported.
func (a *Activity) CompleteNormal() error {
	return platformError()
}

// CheckNoActivity is unavailable where workspace coordination is unsupported.
func (a *Access) CheckNoActivity() error {
	return platformError()
}

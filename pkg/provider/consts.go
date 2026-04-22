// SPDX-License-Identifier: Apache-2.0

package provider

// WorkspaceDir is the workspace-local directory complyctl uses for all
// per-project artifacts (generated configs, scan output, state). Provider
// authors should use this constant when constructing paths under the
// workspace rather than hard-coding the directory name.
const WorkspaceDir = ".complytime"

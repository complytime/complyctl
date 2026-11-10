// SPDX-License-Identifier: Apache-2.0

package policy

// Bump when resolver/config projection changes require providers to regenerate
// artifacts even with unchanged OCI digests. Version 1 includes imported plans;
// older, unversioned generation state may contain only root plans.
const currentGenerationProjectionVersion = 1

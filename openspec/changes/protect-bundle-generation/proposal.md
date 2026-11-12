# Protect resolved plan identities and upgrade generation

The approved consumer council fixes address two distinct defects:
duplicate plan IDs can misattribute results, and unchanged digests can
reuse root-only generated artifacts after imported plans become active.

This tactical change covers graph collision rejection and automatic
generation invalidation for changed projection semantics. It preserves
authored artifacts, plan IDs, routing precedence, and bundle inventory.

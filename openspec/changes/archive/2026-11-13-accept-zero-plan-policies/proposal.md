# Accept guidance-only Gemara Policies

## Why

Consumer acceptance exposed a valid governance Policy with no assessment
plans. The user clarified: "Root policies may have zero plans. Sometimes
they only mandate guidance". This corrects R39's nonempty-plan assumption
for every Policy role, including roots, rather than exempting imports.

## What Changes

The shared Policy decoder MUST accept legitimate zero-plan Policies in
bundle roots, split layers, metadata extraction, and supporting imports.
It MUST retain typed decoding and minimal Policy identity/type checks.
The existing inventory, projection, and root-first precedence remain.

## Impact

This single-repository correction affects `internal/policy` and its
tests. Dependencies, vendor, authored artifacts, and CLI execution
semantics MUST remain unchanged. Successful loading MUST NOT imply
successful assessment execution.

## Constitution Alignment

One shared decoder follows I and II; the bounded correction follows III
and IV. Existing Gemara types/type detection follow V. Retaining raw
inventories and configuration follows VI and VII. The user authorizes
local signed specification commits before implementation without push.

## Archive clarification

The original delta used `MODIFIED` to correct Speckit research R39,
but no canonical `policy-decoding` OpenSpec existed. Archival records
these requirements as `ADDED` to establish that canonical capability.
The supporting-Policy scenario also follows the later declaring-policy
evaluator correction merged in PR #967; root-first timeline precedence
is unchanged. This clarification introduces no implementation changes.

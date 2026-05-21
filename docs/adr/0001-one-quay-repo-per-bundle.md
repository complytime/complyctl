# ADR-0001: One Quay Repository Per Policy Bundle

- **Status:** Accepted
- **Date:** 2026-05-20
- **Canonical Location:** [complytime-policies/docs/adr/0001-one-quay-repo-per-bundle.md](https://github.com/complytime/complytime-policies/blob/main/docs/adr/0001-one-quay-repo-per-bundle.md)

## Summary

Policy bundles published from `complytime-policies` will each get their
own Quay.io repository with a `policies-` prefix (e.g.,
`quay.io/complytime/policies-ampel-branch-protection`) instead of sharing
a single repository with bundle-name tags.

## Impact on complyctl

**None.** `complyctl` treats `complytime.yaml` URLs as opaque OCI
references. The `ParsePolicyRef` and registry client code already
support any valid OCI reference format — single-repo or multi-repo.

### Consumer configuration change

Before (single repo, bundle-name tag):

```yaml
policies:
  - id: ampel-branch-protection
    url: quay.io/complytime/complytime-policies@ampel-branch-protection
```

After (per-bundle repo with policies- prefix, version tag):

```yaml
policies:
  - id: ampel-branch-protection
    url: quay.io/complytime/policies-ampel-branch-protection:latest
```

### Documentation updates needed

- `docs/QUICK_START.md` — update OCI reference examples
- README policy reference examples (if any)

## Full Decision Record

See the canonical ADR in
[complytime-policies](https://github.com/complytime/complytime-policies/blob/main/docs/adr/0001-one-quay-repo-per-bundle.md)
for full context, rationale, and alternatives considered.

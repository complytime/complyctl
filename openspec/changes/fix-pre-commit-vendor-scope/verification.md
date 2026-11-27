# Hook-scope baseline inventory and verification

Use this inventory to review task 3.3 and FR-006 without access to
the original temporary evidence directories. It records one
first-party repair and three preserved command defects, separating
historical observations from checks of PR #976 at
`0d51d716361deef78164a6af925eda2540c16e79`.
Vendor and command exclusions are preservation results, not repairs.

## Historical evidence lineage

The task-3.1 baseline was the explicitly unchanged local HEAD
`e858fd04e2e9a4d34b36a9f01e61310d495a10ed`, with the original hook
configuration. It was **not remote main**, nor a claimed main
ancestor. The baseline clone was restored between individual hook
runs. The candidate used that HEAD plus the seven-file local patch
whose SHA-256 was
`93efcaa6079e1cf3fe0c5fa052fb6cc3eaedd16811705c8e4ee13d728ea8aa84`.

Recovered sources, named here for provenance rather than as required
links, were:

- `/tmp/opencode/task3.1-inventory.q8AkCkPM/`: per-hook results,
  mutation inventories, patches, protected hashes, versions, and
  `provenance.json`.
- `/tmp/opencode/task3.2-reconcile.hE8dDNdR/`: the four reconciled
  first-party findings, archive patch comparison, and targeted runs.
- `/tmp/opencode/task3.3-handoff.JYcVJIQb/`: `verification.json`,
  its verifier, and the 142-file input SHA-256 manifest.
- `/tmp/opencode/task4.2-stage-checks.ICkbU89e/`: staged/all-files
  individual-hook and full-stage results and the earlier handoff.
- `/tmp/opencode/phase2-resume.86ozdE85/candidate`: the retained
  candidate tree, also identified in the task-3.1 provenance.

During this documentation fix, all 142 input manifest hashes matched.
The task-3.3 `verification.json` SHA-256 was
`7a1fb3cff55ad821a23ff66148514b7bd1e4cbc51f56cb912da73b00aad0c7ed`.
Its recorded result was PASS: four first-party dispositions, one
archive repair, three command exceptions, and zero unaccounted
first-party findings. The per-hook inventory rows, result files,
protected hashes, and reconciliation patches independently confirm
the counts and byte changes below; they are not inferred from the
handoff prose alone.

### Historical tools and inputs

- Git 2.47.3; Python and Python 3, both 3.11.13; pre-commit 4.7.0.
- Go and gofmt 1.26.10, linux/arm64, matching the recorded `go.mod`.
- `pre-commit/pre-commit-hooks` pin:
  `3e8a8703264a2f4a69428a0aa4dcb512790b2c8c`;
  installed package version 6.0.0.
- `dnephin/pre-commit-golang` pin:
  `fb24a639f7c938759fe56eeebbb7713b69d60494`
  (configuration label v0.5.1). Its formatter runs `gofmt -l -w`
  on supplied filenames.
- Local SPDX entry: `python ./scripts/check-and-fix-spdx.py`.
- Baseline configuration SHA-256:
  `53674911ca2018944006b7771ac2712245d9cf136a8743a7702b4d994b965515`.
- Candidate configuration SHA-256:
  `51505877a93b6c3e9d05386b8023fdf2c29e4a94a33a3516f08283f0711eaef4`.

The parsed configuration delta consisted only of four hook-level
exclusions. Whitespace/EOF exclude root vendor and the three exact
commands below; SPDX and `go-fmt` exclude root vendor only. No
global, whole-command-directory, or archive exemption was added.
Other hooks, pins, and stage assignments stayed unchanged.

## Historical per-hook inventory

Each task-3.1 invocation used `pre-commit run <hook> --all-files
--hook-stage <stage> --verbose --color never` in a disposable normal
checkout. The table counts baseline **mutated paths**, not selected
files. Candidate changed-path counts were zero for every hook.

| Hook                  | Stage      | Vendor | Other first-party | Exempt commands | Baseline exit | Candidate exit |
| --------------------- | ---------- | -----: | ----------------: | --------------: | ------------: | -------------: |
| `check-spdx`          | pre-commit |  2,143 |                 0 |               0 |             1 |              0 |
| `trailing-whitespace` | pre-push   |     40 |                 1 |               0 |             1 |              0 |
| `end-of-file-fixer`   | pre-push   |     51 |                 0 |               3 |             1 |              0 |
| `go-fmt`              | pre-push   |     38 |                 0 |               0 |             1 |              0 |

The union contained **2,219 unique baseline vendor paths**; hook
counts overlap and MUST NOT be summed as unique files. Each
candidate run preserved before/after hashes for all **3,183 vendor
files** and all three commands, matching original baseline bytes.
Vendor content was excluded, not reformatted or relicensed.
These are historical measurements, never pass criteria or an
expected inventory size for another revision.

## Exact first-party findings and dispositions

All four baseline findings occurred at pre-push. No unaccounted or
unresolved non-exempt first-party formatter finding remained in
this historical inventory.

### Archived design: verified whitespace repair

- **Path:**
  `openspec/changes/archive/2026-09-22-workspace-configuration/design.md`
- **Hook:** `trailing-whitespace`.
- **Defect/reason:** lines 151, 157, 165, 180, 186, 209, 213, 242,
  and 247 each contained four spaces on an otherwise blank line.
  The pinned hook removed them without Markdown-preservation args.
- **Disposition:** verified existing parent repair against the exact
  baseline hook patch; no additional task-3.2 repair was needed.
  Each `202020200a` byte sequence became `0a`: 36 space bytes
  removed, all 386 lines retained, and every other byte unchanged.
  The source patch and baseline formatter patch were identical.
  The archive remains eligible; it has no exclusion.
- **Baseline SHA-256:**
  `e951d699c31e79ee4589d5c753ec3fb1acf3eff9b9393ea2626382ebd7e8cf46`.
- **Repaired candidate and PR-head SHA-256:**
  `06efdf713bd18067e1acf3de85599556f06799166755cf1fba8c669419ba640f`.

### Address-feedback command: preserved template defect

- **Path:** `.opencode/commands/uf.address-feedback.md`.
- **Hook:** `end-of-file-fixer`.
- **Defect/reason:** two terminal LF bytes rather than one; the
  baseline hook removed exactly one excess LF (24,140 to 24,139
  bytes).
- **Disposition:** excluded from formatters; preserved uf-scaffolded bytes
- **Follow-up under #972:** correct this exact path in the uf source
  template. It is excluded from both whitespace and EOF hooks,
  **not repaired**; no GitHub publication is claimed.
- **Preserved baseline/candidate/PR-head SHA-256:**
  `3b2bbba634329d264c67e08c7a98ef5341c61973e0158ccc03d7f6ca34ec920e`.

### Review-council command: preserved template defect

- **Path:** `.opencode/commands/uf.review-council.md`.
- **Hook:** `end-of-file-fixer`.
- **Defect/reason:** two terminal LF bytes rather than one; the
  baseline hook removed exactly one excess LF (38,865 to 38,864
  bytes).
- **Disposition:** excluded from formatters; preserved uf-scaffolded bytes
- **Follow-up under #972:** correct this exact path in the uf source
  template. It is excluded from both whitespace and EOF hooks,
  **not repaired**; no GitHub publication is claimed.
- **Preserved baseline/candidate/PR-head SHA-256:**
  `9cee5a435a98a4a7044ca4570b3bfedec7deb414a0fc79312a26bdb87070d797`.

### Review-pr command: preserved template defect

- **Path:** `.opencode/commands/uf.review-pr.md`.
- **Hook:** `end-of-file-fixer`.
- **Defect/reason:** two terminal LF bytes rather than one; the
  baseline hook removed exactly one excess LF (41,452 to 41,451
  bytes).
- **Disposition:** excluded from formatters; preserved uf-scaffolded bytes
- **Follow-up under #972:** correct this exact path in the uf source
  template. It is excluded from both whitespace and EOF hooks,
  **not repaired**; no GitHub publication is claimed.
- **Preserved baseline/candidate/PR-head SHA-256:**
  `50a20b48cf101e24d8cd21c0c1b064d0b39c7fc99f588055a64cb73d74a737bd`.

Task 3.2 explicitly supplied these four paths to both whitespace/EOF
hooks at pre-push. Both exited 0 without candidate mutations:
the archive was selected and the commands were excluded. This is
historical targeted verification, not a fresh full-stage run.

## Mapping the historical candidate to the PR tree

The retained candidate still has HEAD
`e858fd04e2e9a4d34b36a9f01e61310d495a10ed`. Comparing its tracked
files with the saved task-3.1 fingerprint found only `tasks.md`
drift, consistent with later checkbox bookkeeping. This does
**not** establish equivalence to the current PR tree.

The maintained-file Git blob comparisons below used candidate
worktree bytes, not its unchanged HEAD. Each row records the
historical blob followed by the PR-head blob. Matching identifiers
establish byte equality only for that file.

```text
.pre-commit-config.yaml
  historical: 1477fdd05004ea6b92b297176d0a1ee58f3f519e
  PR head:    1477fdd05004ea6b92b297176d0a1ee58f3f519e
scripts/check-and-fix-spdx.py
  historical: 1e32769ca780132b5b166d5bf706378ffd02c621
  PR head:    1e32769ca780132b5b166d5bf706378ffd02c621
docs/STYLE_GUIDE.md
  historical: d56eb7e05f4804b5bfa3dcc3b3666172a20195f7
  PR head:    d56eb7e05f4804b5bfa3dcc3b3666172a20195f7
openspec/changes/archive/2026-09-22-workspace-configuration/design.md
  historical: 00d44366f10211e18241bd4de29f1e68b6b19d78
  PR head:    00d44366f10211e18241bd4de29f1e68b6b19d78
go.mod
  historical: d385b7d0fdd7cd796831f1d2b907f75362f2e7e2
  PR head:    1e5f86f147a7e5e1df87af6e63b4073a5ccb8913
go.sum
  historical: 446c2d825626a47ec83c009fd064e453be4fafb5
  PR head:    466907b34be9b235c16a5f1b54ec3a5850c4151f
Makefile
  historical: dbc72e02803f73a4ae3f938e13472a48d49b1989
  PR head:    5388fb5261cb8f6f0f3037f4898086adc06650ef
tests/pre_commit_scope_test.py
  historical: 78b37a44d86e46d04c97235b914a37db552608f4
  PR head:    108d847beaf834f10a95f11ff78559188fd5e721
CHANGELOG.md
  historical: 4e20e54e4f7b79a89f066d4051561e6226e738a4
  PR head:    7ae751706e2c7dbcea22d18a82c3dfd6b05181c4
```

The SPDX script's matching SHA-256 is
`bf70db23d1648afcdd2ad273a8f800df64ac285d7fab0d7b3726ad046afcd8fa`.
The hook config matches the candidate SHA-256 recorded above.
The four first-party finding hashes also match the PR head.
However, dependencies, regression tests, other maintained files,
and vendor content differ. The historical vendor Git tree was
`19bcc30d4358a3ff7bab7829771e960945c52a5d`; the PR-head tree is
`c646b0c54b9c31a7212067e209619ddc8b1de039`.
Historical totals and test results therefore cannot certify this
PR's complete tree or its current regression suite.

To reproduce the maintained-file comparisons from the repository
root while the original candidate remains available, run this
read-only sequence. The two outputs per path are historical and
PR-head Git blobs, respectively; no candidate checkout is changed.

```bash
historical=/tmp/opencode/phase2-resume.86ozdE85/candidate
pr=0d51d716361deef78164a6af925eda2540c16e79
for path in .pre-commit-config.yaml scripts/check-and-fix-spdx.py \
    docs/STYLE_GUIDE.md \
    openspec/changes/archive/2026-09-22-workspace-configuration/design.md \
    go.mod go.sum Makefile tests/pre_commit_scope_test.py CHANGELOG.md; do
    git --no-optional-locks -C "$historical" hash-object --no-filters -- "$path"
    git --no-optional-locks rev-parse "$pr:$path"
done
```

## Fresh PR-head preservation checks

A disposable normal clone at the exact PR head ran the following
four commands during this documentation fix. These are fresh
all-files checks of the pre-documentation PR head, not a rerun of
the original baseline or the full CI suite. No modifying hook ran
in the source worktree or project root.

```bash
pre-commit run check-spdx --all-files --hook-stage pre-commit --verbose --color never
pre-commit run trailing-whitespace --all-files --hook-stage pre-push --verbose --color never
pre-commit run end-of-file-fixer --all-files --hook-stage pre-push --verbose --color never
pre-commit run go-fmt --all-files --hook-stage pre-push --verbose --color never
```

All **4/4 hooks executed**, exited **0**, and changed **0** tracked
files or modes. Before/after SHA-256 comparisons covered all 3,921
tracked files, including **3,196 current vendor files** and all
three exempt commands, after each hook. `git diff --exit-code`
also exited 0. The different vendor count is a revision difference,
not an acceptance threshold or an update to the historical counts.

Fresh tools were Git 2.47.3, Python/Python 3 3.11.13, pre-commit
4.7.0, and Go/gofmt 1.26.10 linux/arm64. The runs reused the
recorded task-1.4 hook environments and selected the cached 1.26.10
toolchain explicitly, with `GOTOOLCHAIN=local`, `GOPROXY=off`, and
`GOSUMDB=off`. Hook pins and configuration were unchanged.

### Fresh documentation checks

- Markdownlint CLI 0.49.1 checked both changed Markdown files with
  `openspec/.markdownlint.json`: exit 0, no findings.
- Lychee 0.24.2 checked both files with `--offline` and
  `--include-fragments`: 4 links, 4 unique, 4 OK, 0 errors; exit 0.
  This checks local targets and fragments, not external services.
- OpenSpec 1.14.3 ran
  `openspec validate fix-pre-commit-vendor-scope --strict`:
  one change valid, exit 0. This result is version-specific;
  FR-001 was not edited.
- Git 2.47.3 `diff --check` exited 0. The new file's
  `--no-index --check` comparison against `/dev/null` emitted no
  whitespace diagnostics; its exit 1 reports the added-file
  difference. A byte comparison also confirmed the task file
  contains only the inventory-link insertion, preserving all
  checkbox states and other existing bytes.

### Scope and remaining handoffs

The historical task-4.2 records contain 12 exit-0 staged/all-files
individual-hook and full-stage commands with no file mutations.
Its initial summary stopped on raw index stat-cache differences;
the later handoff classifies these as optional Git metadata, not
source or logical staging changes. Neither record is a fresh CI
result for the PR head. This documentation fix does not claim a
new full-stage, regression-suite, Go-test, or council approval.

For documentation CI, `.github/workflows/ci_checks.yml` delegates
to org-infra `reusable_ci.yml` at
`8f347ba1aa2c7e9eb3b70e69c1709ddb456cd64a`. That workflow runs
MegaLinter; this repository enables `MARKDOWN_MARKDOWNLINT` and
`SPELL_LYCHEE` in `.mega-linter.yml`. The local `ci_local.yml`
jobs cover protobuf/API/Go tests, not a separate docs command.
Only applicable Markdown/link and strict-spec checks are needed
for this two-file documentation fix; it does not establish full
CI parity for the cumulative implementation PR.

The existing `CHANGELOG.md` entry already covers the hook fix.
There is no runtime, structure, README, or AGENTS change here.
The previously recorded AGENTS test-command follow-up remains
human-only; it is not an unaccounted formatter finding. The parent
workflow will review the cumulative PR. FR-001 and all task
checkbox states remain unchanged.

See [task 3.3](tasks.md), the [design](design.md), and
[FR-006](specs/project-hook-scope/spec.md#requirement-fr-006-account-for-first-party-baseline-findings)
for the inventory contract and scope.

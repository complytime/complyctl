#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Focused regression tests for the API compatibility workflow gate.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly REPO_ROOT
readonly WORKFLOW_PATH="${REPO_ROOT}/.github/workflows/ci_local.yml"
readonly RELEASE_WORKFLOW_PATH="${REPO_ROOT}/.github/workflows/release.yml"

python3 - "${WORKFLOW_PATH}" "${RELEASE_WORKFLOW_PATH}" <<'PY'
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile

workflow_path = pathlib.Path(sys.argv[1])
workflow = workflow_path.read_text(encoding="utf-8")
release_workflow_path = pathlib.Path(sys.argv[2])
release_workflow = release_workflow_path.read_text(encoding="utf-8")
job_start = workflow.index("    api-compatibility:\n")
job_end = workflow.index("    unit-test:\n", job_start)
job = workflow[job_start:job_end]

for forbidden_value in (
    "api-compatibility-analysis",
    "breaking-change-justification",
    "breaking-change-override",
    "release/v2",
    "environment:",
    "deployments: write",
):
    if forbidden_value in workflow:
        raise SystemExit(f"FAIL: deferred override orchestration remains: {forbidden_value}")

for required_value in (
    "fetch-depth: 0",
    "set -euo pipefail",
    "make api-compatibility 2>&1 | tee api-compatibility.log",
    "if: always()",
    "Selected baseline",
    "Compatibility analysis did not produce diagnostics.",
    'version: "1.50.0"',
):
    if required_value not in job:
        raise SystemExit(f"FAIL: missing fail-closed diagnostics behavior: {required_value}")

if workflow.count("    api-compatibility:\n") != 1:
    raise SystemExit("FAIL: workflow must define exactly one api-compatibility job")
if "        branches:\n            - main" not in workflow:
    raise SystemExit("FAIL: api compatibility must run on pushes to main")
if "needs:" in job or "environment:" in job:
    raise SystemExit("FAIL: api compatibility must not depend on override orchestration")

ci_checks_match = re.search(
    r"^\s*ci_checks:\s*'(?P<checks>\[.*\])'\s*$", release_workflow, re.MULTILINE
)
if ci_checks_match is None:
    raise SystemExit("FAIL: release preflight must declare ci_checks")
ci_checks = json.loads(ci_checks_match.group("checks"))
if ci_checks.count("api-compatibility") != 1:
    raise SystemExit("FAIL: release preflight ci_checks must include api-compatibility exactly once")

def run_analysis(diagnostic: str) -> subprocess.CompletedProcess[str]:
    with tempfile.TemporaryDirectory() as temporary_directory:
        temporary_path = pathlib.Path(temporary_directory)
        fake_make = temporary_path / "make"
        fake_make.write_text(
            "#!/usr/bin/env bash\nprintf '%s\\n' \"${API_COMPATIBILITY_DIAGNOSTIC}\" >&2\nexit 1\n",
            encoding="utf-8",
        )
        fake_make.chmod(0o755)
        environment = {
            "API_COMPATIBILITY_DIAGNOSTIC": diagnostic,
            "PATH": f"{temporary_directory}:{os.environ['PATH']}",
        }
        return subprocess.run(
            ["bash", "-c", "set -euo pipefail; make api-compatibility 2>&1 | tee log"],
            check=False,
            cwd=temporary_directory,
            env=environment,
            capture_output=True,
            text=True,
        )


for failed_operation, diagnostic in (
    ("incompatible API", "Go API compatibility: incompatible Go API detected"),
    ("baseline resolution", "baseline resolution: no stable baseline tag is available"),
    ("tool installation", "protobuf compatibility: tool setup: buf is not installed"),
    ("package loading", "Go API compatibility: package loading: cannot load candidate"),
    ("analyzer operation", "Go API compatibility: analyzer operation failed"),
):
    result = run_analysis(diagnostic)
    if result.returncode == 0:
        raise SystemExit(f"FAIL: {failed_operation} failure passed")
    if diagnostic not in result.stdout:
        raise SystemExit(f"FAIL: {failed_operation} diagnostic was not preserved")

print("PASS: API compatibility workflow fail-closed tests")
PY

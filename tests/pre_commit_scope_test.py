# SPDX-License-Identifier: Apache-2.0
"""Exercise real hooks in disposable normal repositories, never the source checkout.

Run with Python >=3.10, pytest, Git, pre-commit, and the Go toolchain in go.mod.
Pinned hook environments initialize in a session-local PRE_COMMIT_HOME; initial
setup can require network access. Use --basetemp outside the source checkout.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

import pytest

SOURCE_ROOT = Path(__file__).resolve().parents[1]
CONFIG_PATH = Path(".pre-commit-config.yaml")
SPDX_SCRIPT_PATH = Path("scripts/check-and-fix-spdx.py")
COMMAND_TIMEOUT_SECONDS = 30
HOOK_TIMEOUT_SECONDS = 180
MINIMUM_PYTHON_VERSION = (3, 10)
REQUIRED_EXECUTABLES = ("git", "python", "pre-commit", "go", "gofmt")
HOOK_STAGES = {
    "trailing-whitespace": "pre-push",
    "end-of-file-fixer": "pre-push",
    "go-fmt": "pre-push",
    "check-spdx": "pre-commit",
}
COMPLIANT_FILES = {
    "notes.txt": b"Maintained first-party control.\n",
    "example.go": b"// SPDX-License-Identifier: Apache-2.0\npackage example\n",
    "example.py": b"# SPDX-License-Identifier: Apache-2.0\nVALUE = 1\n",
}
TEXT_FORMAT_CASES = {
    "trailing-whitespace": (b"Maintained content.  \n", b"Maintained content.\n"),
    "end-of-file-fixer": (b"Maintained content.\n\n", b"Maintained content.\n"),
}
EXEMPT_COMMANDS = (
    ".opencode/commands/uf.review-council.md",
    ".opencode/commands/uf.review-pr.md",
    ".opencode/commands/uf.address-feedback.md",
)
COMMAND_NEIGHBORS = (
    ".opencode/commands/uf.other.md",
    ".opencode/commands/uf.review-pr.md.bak",
    ".opencode/commands/xuf.review-pr.md",
    "nested/.opencode/commands/uf.review-pr.md",
    "openspec/changes/archive/fixture/design.md",
)
VENDOR_INPUTS = {
    "vendor/upstream/example.go": (
        b"// SPDX-License-Identifier: MIT\npackage upstream\n\n"
        b"func value( )int {return 1}  \n\n"
    ),
    "vendor/upstream/example.py": (b"# SPDX-License-Identifier: MIT\nVALUE = 1  \n\n"),
    "vendor/upstream/example.sh": (
        b"#!/bin/sh\n# SPDX-License-Identifier: MIT\nexit 0  \n\n"
    ),
    "vendor/upstream/NOTICE.txt": (
        b"SPDX-License-Identifier: MIT\nUpstream license notice.  \n\n"
    ),
}


def _run_command(
    arguments: list[str],
    *,
    cwd: Path,
    env: dict[str, str],
    timeout: float = COMMAND_TIMEOUT_SECONDS,
) -> subprocess.CompletedProcess[str]:
    """Capture diagnostics, preserving hook failure codes but bounding every call."""
    try:
        # All callers supply test-owned argument lists; no shell evaluates them.
        return subprocess.run(
            arguments,
            cwd=cwd,
            env=env,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
    except OSError as error:
        raise RuntimeError(
            f"Cannot execute {arguments!r} in {cwd}: {error}. "
            "Install the required executable and check PATH before rerunning."
        ) from error
    except subprocess.TimeoutExpired as error:
        raise RuntimeError(
            f"Timed out after {timeout}s running {arguments!r} in {cwd}. "
            "Check tool availability and network access for pinned hook setup; "
            f"rerun with a fresh temporary directory. "
            f"stdout={error.stdout!r}; stderr={error.stderr!r}"
        ) from error


def _checked_command(arguments: list[str], *, cwd: Path, env: dict[str, str]) -> str:
    """Return successful setup output, or raise with an actionable diagnostic."""
    result = _run_command(arguments, cwd=cwd, env=env)
    if result.returncode:
        raise RuntimeError(
            f"Setup command {arguments!r} failed in {cwd} (exit {result.returncode}). "
            "Check the installed prerequisites before rerunning.\n"
            f"{result.stdout}{result.stderr}"
        )
    return result.stdout.strip()


def _require_python_version(version: tuple[int, ...]) -> None:
    if version[:2] < MINIMUM_PYTHON_VERSION:
        raise RuntimeError(
            f"Python >=3.10 is required; found {version}. "
            "Run python3 -m pytest with a supported interpreter."
        )


def _first_party_cases(hook_id: str) -> dict[str, tuple[bytes, bytes]]:
    """Return explicit defective/repaired bytes, not a replacement formatter."""
    if hook_id in TEXT_FORMAT_CASES:
        return {
            path: TEXT_FORMAT_CASES[hook_id]
            for path in (
                "first_party/notes.txt",
                "internal/vendor_helpers.txt",
                "tests/vendor_cases/example.txt",
            )
        }
    if hook_id == "go-fmt":
        return {
            path: (
                (
                    b"// SPDX-License-Identifier: Apache-2.0\npackage example\n\n"
                    b"func answer( )int {return 42}\n"
                ),
                (
                    b"// SPDX-License-Identifier: Apache-2.0\npackage example\n\n"
                    b"func answer() int { return 42 }\n"
                ),
            )
            for path in (
                "first_party/example.go",
                "internal/vendor_helpers.go",
                "tests/vendor_cases/example.go",
            )
        }
    assert hook_id == "check-spdx"
    return {
        "internal/vendor_helpers.go": (
            b"package example\n",
            b"// SPDX-License-Identifier: Apache-2.0\npackage example\n",
        ),
        "tests/vendor_cases/example.py": (
            b"VALUE = 1\n",
            b"# SPDX-License-Identifier: Apache-2.0\nVALUE = 1\n",
        ),
        "tests/vendor_cases/example.sh": (
            b"exit 0\n",
            b"# SPDX-License-Identifier: Apache-2.0\nexit 0\n",
        ),
        "first_party/shebang.py": (
            b"#!/usr/bin/env python3\nVALUE = 1\n",
            b"#!/usr/bin/env python3\n# SPDX-License-Identifier: Apache-2.0\nVALUE = 1\n",
        ),
        "first_party/shebang.sh": (
            b"#!/bin/sh\nexit 0\n",
            b"#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nexit 0\n",
        ),
    }


def _assert_hook_result(
    result: subprocess.CompletedProcess[str], hook_id: str, *, failed: bool
) -> str:
    """Require actual targeted execution, including expected validation or repair failures."""
    diagnostics = result.stdout + result.stderr
    assert result.returncode == (1 if failed else 0), diagnostics
    assert re.findall(r"^- hook id: (.+)$", diagnostics, re.MULTILINE) == [hook_id], (
        diagnostics
    )
    assert ("Failed" if failed else "Passed") in diagnostics, diagnostics
    assert "Skipped" not in diagnostics, diagnostics
    assert "no files to check" not in diagnostics, diagnostics
    return diagnostics


def _verify_prerequisites(cwd: Path, env: dict[str, str]) -> dict[str, str]:
    """Verify installed tools; select bundled gofmt rather than an opaque shim.

    Mutates only the fixture environment's PATH. Raises RuntimeError for missing
    tools or mismatched versions; never installs tools or skips verification.
    """
    _require_python_version(tuple(sys.version_info[:3]))
    for executable in REQUIRED_EXECUTABLES:
        if shutil.which(executable, path=env.get("PATH", "")) is None:
            raise RuntimeError(
                f"Missing prerequisite: {executable}. Install {executable} and add it "
                "to PATH; use Python >=3.10 with pytest and Go from go.mod."
            )

    versions = {
        "git": _checked_command(["git", "--version"], cwd=cwd, env=env),
        "python": sys.version.split()[0],
        "pytest": _checked_command(
            [sys.executable, "-m", "pytest", "--version"], cwd=cwd, env=env
        ),
        "pre-commit": _checked_command(["pre-commit", "--version"], cwd=cwd, env=env),
    }
    hook_python = _checked_command(
        ["python", "-c", "import sys; print(*sys.version_info[:3], sep='.')"],
        cwd=cwd,
        env=env,
    )
    _require_python_version(tuple(int(part) for part in hook_python.split(".")))
    versions["hook-python"] = hook_python
    module = (SOURCE_ROOT / "go.mod").read_text(encoding="utf-8")
    directive = re.search(r"^go (\d+\.\d+(?:\.\d+)?)$", module, re.MULTILINE)
    if directive is None:
        raise RuntimeError(
            "Cannot find the Go version in go.mod; check its go directive."
        )
    versions["go.mod"] = directive.group(1)
    # Resolve an already-installed automatic toolchain in the module context.
    # Disabling downloads avoids silently installing or downgrading Go when the
    # fixture runs outside the module and a tool-manager shim has another default.
    resolution_env = {**env, "GOTOOLCHAIN": "auto", "GOPROXY": "off"}
    goroot = _checked_command(
        ["go", "env", "GOROOT"], cwd=SOURCE_ROOT, env=resolution_env
    )
    gofmt = Path(goroot) / "bin" / "gofmt"
    env["PATH"] = os.pathsep.join((str(gofmt.parent), env["PATH"]))
    versions["go"] = _checked_command(["go", "version"], cwd=cwd, env=env)
    if versions["go"].split()[2] != f"go{versions['go.mod']}":
        raise RuntimeError(
            f"Go toolchain mismatch: go.mod requires {versions['go.mod']}; "
            f"found {versions['go']}. Select the declared installed toolchain."
        )
    versions["gofmt"] = _checked_command(
        ["go", "version", str(gofmt)], cwd=cwd, env=env
    )
    if versions["gofmt"].split()[-1] != f"go{versions['go.mod']}":
        raise RuntimeError(
            f"gofmt toolchain mismatch: {versions['gofmt']}. "
            "Select the gofmt bundled with the Go version in go.mod."
        )
    return versions


@dataclass
class HookRepository:
    """A normal temporary repository and its private process environment."""

    path: Path
    env: dict[str, str]

    def write(self, relative_path: str, content: bytes) -> Path:
        """Write fixture bytes inside this repository, rejecting escaping paths."""
        destination = (self.path / relative_path).resolve()
        if not destination.is_relative_to(self.path.resolve()):
            raise ValueError(
                f"Fixture path escapes temporary repository: {relative_path}"
            )
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(content)
        return destination

    def git(self, *arguments: str) -> str:
        """Run a bounded Git setup/query command only in this fixture."""
        return _checked_command(["git", *arguments], cwd=self.path, env=self.env)

    def commit_fixture_baseline(self) -> None:
        """Commit staged fixture bytes with process-only identity and no installed hooks."""
        hooks_path = Path(self.git("config", "--get", "core.hooksPath"))
        assert hooks_path.is_dir() and list(hooks_path.iterdir()) == []
        commit_env = {
            **self.env,
            "GIT_CONFIG_COUNT": "3",
            "GIT_CONFIG_KEY_1": "user.name",
            "GIT_CONFIG_VALUE_1": "Pre-commit Fixture",
            "GIT_CONFIG_KEY_2": "user.email",
            "GIT_CONFIG_VALUE_2": "hook-fixture@example.invalid",
        }
        name = _checked_command(
            ["git", "config", "user.name"], cwd=self.path, env=commit_env
        )
        email = _checked_command(
            ["git", "config", "user.email"], cwd=self.path, env=commit_env
        )
        _checked_command(
            [
                "git",
                "commit",
                "-s",
                "-m",
                "test: record hook selection baseline",
                "-m",
                "Assisted-by: OpenCode (gpt-6.1-sol)",
            ],
            cwd=self.path,
            env=commit_env,
        )
        assert f"Signed-off-by: {name} <{email}>" in self.git(
            "log", "-1", "--format=%B"
        )
        assert list(hooks_path.iterdir()) == []
        assert "user." not in self.git("config", "--local", "--list")

    def run_hook(
        self, hook_id: str, *, all_files: bool = False, hook_stage: str | None = None
    ) -> subprocess.CompletedProcess[str]:
        """Invoke one real hook at the requested or default verification stage."""
        arguments = [
            "pre-commit",
            "run",
            hook_id,
            "--hook-stage",
            HOOK_STAGES[hook_id] if hook_stage is None else hook_stage,
            "--color=never",
            "--verbose",
        ]
        if all_files:
            arguments.append("--all-files")
        return _run_command(
            arguments, cwd=self.path, env=self.env, timeout=HOOK_TIMEOUT_SECONDS
        )


@pytest.fixture(scope="session")
def hook_environment(tmp_path_factory: pytest.TempPathFactory) -> dict[str, str]:
    """Keep hook installs and user configuration outside the source repository."""
    root = tmp_path_factory.mktemp("hook-runtime").resolve()
    if root.is_relative_to(SOURCE_ROOT):
        pytest.fail(
            "Use --basetemp /tmp/... outside the source checkout for hook tests."
        )
    # Git/pre-commit overrides from the caller must not redirect fixture commands
    # to the source index, skip hooks, or reuse the developer's mutable cache.
    env = {
        key: value
        for key, value in os.environ.items()
        if not key.startswith(("GIT_", "PRE_COMMIT_")) and key != "SKIP"
    }
    for name in ("cache", "config", "pre-commit", "empty-hooks"):
        (root / name).mkdir()
    # Retain HOME so installed Python user-site packages remain discoverable;
    # Git configuration, hook environments, and writable caches are isolated below.
    env.update(
        XDG_CACHE_HOME=str(root / "cache"),
        XDG_CONFIG_HOME=str(root / "config"),
        PRE_COMMIT_HOME=str(root / "pre-commit"),
        GIT_CONFIG_NOSYSTEM="1",
        GIT_CONFIG_GLOBAL=os.devnull,
        GIT_CONFIG_COUNT="1",
        GIT_CONFIG_KEY_0="core.hooksPath",
        GIT_CONFIG_VALUE_0=str(root / "empty-hooks"),
        GIT_TERMINAL_PROMPT="0",
        GOTOOLCHAIN="local",
        GOWORK="off",
        PYTHONDONTWRITEBYTECODE="1",
    )
    versions = _verify_prerequisites(root, env)
    print(f"Verified real-hook prerequisites: {versions}")
    return env


@pytest.fixture
def hook_repository(tmp_path: Path, hook_environment: dict[str, str]) -> HookRepository:
    """Copy actual policy and script bytes into a fresh, non-worktree Git repo."""
    if tmp_path.resolve().is_relative_to(SOURCE_ROOT):
        pytest.fail("Fixture repositories must be outside the source checkout.")
    repository = HookRepository(tmp_path, hook_environment.copy())
    repository.git("init", "--template=", "--initial-branch=fixture")
    for source in (CONFIG_PATH, SPDX_SCRIPT_PATH):
        repository.write(str(source), (SOURCE_ROOT / source).read_bytes())
    module = (SOURCE_ROOT / "go.mod").read_text(encoding="utf-8")
    directive = next(line for line in module.splitlines() if line.startswith("go "))
    repository.write(
        "go.mod", f"module example.com/hook-fixture\n\n{directive}\n".encode()
    )
    return repository


def _tracked_bytes(repository: HookRepository) -> dict[str, bytes]:
    return {
        path: (repository.path / path).read_bytes()
        for path in repository.git("ls-files").splitlines()
    }


def _remove_exclusion_branch(
    repository: HookRepository, hook_id: str, branch: str
) -> None:
    """Delete one branch from the actual copied hook policy, never construct policy."""
    config_path = repository.path / CONFIG_PATH
    original = config_path.read_text(encoding="utf-8")
    match = re.search(
        rf"(?m)^ +- id: {re.escape(hook_id)}\n"
        r"(?P<line> +exclude: (?P<expression>[^\n]+)\n)",
        original,
    )
    assert match is not None, f"Expected an exclusion on copied hook {hook_id}"
    expression = match["expression"].strip("'\"")
    replacement = ""
    if hook_id in TEXT_FORMAT_CASES:
        assert expression.startswith("^(") and expression.endswith(")")
        vendor, commands = expression[2:-1].split("|", 1)
        assert vendor == "vendor/"
        assert branch in ("vendor", "commands")
        retained = commands if branch == "vendor" else vendor
        indentation = match["line"].split("exclude:", 1)[0]
        replacement = f"{indentation}exclude: '^({retained})'\n"
    else:
        assert branch == "vendor" and expression == "^vendor/"
    # Slice only this hook's exclusion line: pinned revisions, stages, entries,
    # and every other hook's bytes remain the candidate's actual configuration.
    repository.write(
        str(CONFIG_PATH),
        (
            original[: match.start("line")]
            + replacement
            + original[match.end("line") :]
        ).encode(),
    )


def _assert_idempotent(
    repository: HookRepository, hook_id: str, *, all_files: bool
) -> None:
    """Restage repairs so staged-mode pre-commit does not stash the corrected bytes."""
    before = _tracked_bytes(repository)
    repository.git("add", "--force", ".")
    result = repository.run_hook(hook_id, all_files=all_files)
    _assert_hook_result(result, hook_id, failed=False)
    assert _tracked_bytes(repository) == before
    assert repository.git("diff", "--name-only") == ""


@pytest.mark.parametrize("hook_id", HOOK_STAGES)
def test_real_hook_compliant_control_executes_without_mutation(
    hook_repository: HookRepository, hook_id: str
) -> None:
    """A passing hook must actually execute on eligible tracked first-party files."""
    repository = hook_repository
    for name, content in COMPLIANT_FILES.items():
        repository.write(name, content)
    repository.git("add", "--", *COMPLIANT_FILES)
    assert (repository.path / ".git").is_dir()
    assert repository.git("rev-parse", "--is-bare-repository") == "false"
    assert repository.git("rev-parse", "--show-toplevel") == str(
        repository.path.resolve()
    )
    before = {
        path: (repository.path / path).read_bytes()
        for path in (
            *COMPLIANT_FILES,
            str(CONFIG_PATH),
            str(SPDX_SCRIPT_PATH),
            "go.mod",
        )
    }

    result = repository.run_hook(hook_id)

    _assert_hook_result(result, hook_id, failed=False)
    assert repository.git("diff", "--name-only") == ""
    for path, content in before.items():
        assert (repository.path / path).read_bytes() == content, path
    for source in (CONFIG_PATH, SPDX_SCRIPT_PATH):
        assert (repository.path / source).read_bytes() == (
            SOURCE_ROOT / source
        ).read_bytes()
    hooks_path = Path(repository.git("config", "--get", "core.hooksPath"))
    assert list(hooks_path.iterdir()) == []
    assert not (repository.path / ".git" / "hooks" / "pre-commit").exists()


@pytest.mark.parametrize("all_files", (False, True), ids=("staged", "all-files"))
@pytest.mark.parametrize(
    "remove_vendor", (False, True), ids=("candidate", "no-vendor-exclusion")
)
@pytest.mark.parametrize("hook_id", HOOK_STAGES)
def test_vendor_preservation_and_first_party_repairs(
    hook_repository: HookRepository,
    hook_id: str,
    remove_vendor: bool,
    all_files: bool,
) -> None:
    """Real repairs retain first-party checks; independent mutations expose vendor damage."""
    repository = hook_repository
    if remove_vendor:
        _remove_exclusion_branch(repository, hook_id, "vendor")
    cases = _first_party_cases(hook_id)
    compliant = {f"clean/{path}": corrected for path, (_, corrected) in cases.items()}
    for path, content in VENDOR_INPUTS.items():
        repository.write(path, content)
    for path, (defective, _) in cases.items():
        repository.write(path, defective)
    for path, content in compliant.items():
        repository.write(path, content)
    repository.git("add", "--force", ".")
    before = _tracked_bytes(repository)

    result = repository.run_hook(hook_id, all_files=all_files)
    diagnostics = _assert_hook_result(result, hook_id, failed=True)
    after = _tracked_bytes(repository)

    vendor_changed = set()
    for path, content in VENDOR_INPUTS.items():
        eligible = (
            hook_id in TEXT_FORMAT_CASES
            or (hook_id == "go-fmt" and path.endswith(".go"))
            or (hook_id == "check-spdx" and not path.endswith(".txt"))
        )
        if remove_vendor and eligible:
            assert after[path] != content, path
            assert path in diagnostics, diagnostics
            vendor_changed.add(path)
            if hook_id == "check-spdx":
                assert b"SPDX-License-Identifier: Apache-2.0" in after[path]
        else:
            assert after[path] == content, path
            assert path not in diagnostics, diagnostics
        assert b"SPDX-License-Identifier: MIT" in after[path], path
    for path, (_, corrected) in cases.items():
        assert after[path] == corrected, path
        assert path in diagnostics, diagnostics
    for path, content in compliant.items():
        assert after[path] == content, path
    changed = {path for path in before if before[path] != after[path]}
    assert changed == set(cases) | vendor_changed
    assert set(repository.git("diff", "--name-only").splitlines()) == changed
    _assert_idempotent(repository, hook_id, all_files=all_files)


@pytest.mark.parametrize("all_files", (False, True), ids=("staged", "all-files"))
@pytest.mark.parametrize("removed_branch", (None, "commands", "vendor"))
@pytest.mark.parametrize("hook_id", TEXT_FORMAT_CASES)
def test_exact_command_exceptions_and_selected_neighbors(
    hook_repository: HookRepository,
    hook_id: str,
    removed_branch: str | None,
    all_files: bool,
) -> None:
    """Each branch mutation damages only its own protected paths, not the other branch."""
    repository = hook_repository
    if removed_branch is not None:
        _remove_exclusion_branch(repository, hook_id, removed_branch)
    defective, corrected = TEXT_FORMAT_CASES[hook_id]
    vendor_path = "vendor/upstream/command-notes.txt"
    eligible_path = "first_party/control.txt"
    clean_path = "first_party/clean.txt"
    for path in (*EXEMPT_COMMANDS, *COMMAND_NEIGHBORS, vendor_path, eligible_path):
        repository.write(path, defective)
    repository.write(clean_path, corrected)
    repository.git("add", "--force", ".")
    before = _tracked_bytes(repository)

    result = repository.run_hook(hook_id, all_files=all_files)
    diagnostics = _assert_hook_result(result, hook_id, failed=True)
    after = _tracked_bytes(repository)

    expected_changed = {*COMMAND_NEIGHBORS, eligible_path}
    if removed_branch == "commands":
        expected_changed.update(EXEMPT_COMMANDS)
    if removed_branch == "vendor":
        expected_changed.add(vendor_path)
    findings = {
        line.removeprefix("Fixing ")
        for line in diagnostics.splitlines()
        if line.startswith("Fixing ")
    }
    # Compare complete paths: an exempt command is also a substring of the
    # deliberately eligible .bak and nested-copy controls.
    assert findings == expected_changed, diagnostics
    for path in expected_changed:
        assert after[path] == corrected, path
    for path in (*EXEMPT_COMMANDS, vendor_path):
        if path not in expected_changed:
            assert after[path] == before[path] == defective, path
    assert after[clean_path] == corrected
    changed = {path for path in before if before[path] != after[path]}
    assert changed == expected_changed
    assert set(repository.git("diff", "--name-only").splitlines()) == changed
    _assert_idempotent(repository, hook_id, all_files=all_files)


@pytest.mark.parametrize(
    ("hook_id", "hook_stage"),
    [
        (hook_id, stage)
        for hook_id in TEXT_FORMAT_CASES
        for stage in ("pre-commit", "pre-push", "manual")
    ]
    + [("go-fmt", "pre-push"), ("check-spdx", "pre-commit")],
)
def test_staged_selection_preserves_committed_defect_until_all_files(
    hook_repository: HookRepository, hook_id: str, hook_stage: str
) -> None:
    """A clean staged change cannot hide a committed defect from an all-files run."""
    repository = hook_repository
    defect_path, (defective, corrected) = next(
        iter(_first_party_cases(hook_id).items())
    )
    control_path = f"staged/{defect_path}"
    repository.write(defect_path, defective)
    repository.write(control_path, corrected)
    repository.git("add", "--force", ".")
    repository.commit_fixture_baseline()
    assert repository.git("status", "--porcelain") == ""

    comment = (
        b"// staged change\n" if control_path.endswith(".go") else b"# staged change\n"
    )
    repository.write(control_path, comment + corrected)
    repository.git("add", "--", control_path)
    assert repository.git("diff", "--cached", "--name-only") == control_path
    assert repository.git("diff", "--name-only") == ""
    before = _tracked_bytes(repository)

    staged = repository.run_hook(hook_id, hook_stage=hook_stage)
    staged_diagnostics = _assert_hook_result(staged, hook_id, failed=False)
    assert defect_path not in staged_diagnostics, staged_diagnostics
    assert _tracked_bytes(repository) == before
    assert repository.git("diff", "--name-only") == ""
    assert repository.git("diff", "--cached", "--name-only") == control_path

    all_files = repository.run_hook(hook_id, hook_stage=hook_stage, all_files=True)
    diagnostics = _assert_hook_result(all_files, hook_id, failed=True)
    assert defect_path in diagnostics, diagnostics
    assert _tracked_bytes(repository) == {**before, defect_path: corrected}
    assert repository.git("diff", "--name-only") == defect_path
    assert repository.git("diff", "--cached", "--name-only") == control_path


@pytest.mark.parametrize("all_files", (False, True), ids=("staged", "all-files"))
@pytest.mark.parametrize(
    ("hook_id", "wrong_stage"),
    [("go-fmt", "pre-commit"), ("check-spdx", "pre-push")],
)
def test_wrong_stage_is_rejected_not_counted_as_hook_verification(
    hook_repository: HookRepository,
    hook_id: str,
    wrong_stage: str,
    all_files: bool,
) -> None:
    """A stage-filtered hook cannot pass verification; the configured stage repairs it."""
    repository = hook_repository
    path, (defective, corrected) = next(iter(_first_party_cases(hook_id).items()))
    repository.write(path, defective)
    repository.git("add", "--force", ".")
    before = _tracked_bytes(repository)

    wrong_stage_result = repository.run_hook(
        hook_id, hook_stage=wrong_stage, all_files=all_files
    )
    diagnostics = wrong_stage_result.stdout + wrong_stage_result.stderr
    assert wrong_stage_result.returncode == 1, diagnostics
    assert f"No hook with id `{hook_id}` in stage `{wrong_stage}`" in diagnostics
    assert f"- hook id: {hook_id}" not in diagnostics
    assert _tracked_bytes(repository) == before
    assert repository.git("diff", "--name-only") == ""
    for failed in (False, True):
        with pytest.raises(AssertionError):
            _assert_hook_result(wrong_stage_result, hook_id, failed=failed)

    configured_stage_result = repository.run_hook(hook_id, all_files=all_files)
    _assert_hook_result(configured_stage_result, hook_id, failed=True)
    assert _tracked_bytes(repository) == {**before, path: corrected}
    assert repository.git("diff", "--name-only") == path


@pytest.mark.parametrize("all_files", (False, True), ids=("staged", "all-files"))
def test_unrelated_check_yaml_still_validates_vendor_inputs(
    hook_repository: HookRepository, all_files: bool
) -> None:
    """Vendor formatter exclusions must not hide malformed YAML from the real validator."""
    repository = hook_repository
    hook_id = "check-yaml"
    invalid_path = "vendor/upstream/invalid.yaml"
    vendor_control = "vendor/upstream/compliant.yaml"
    maintained_control = "first_party/compliant.yaml"
    repository.write(invalid_path, b"policy: [unterminated\n")
    repository.write(vendor_control, b"policy: upstream\n")
    repository.write(maintained_control, b"policy: maintained\n")
    repository.git("add", "--force", ".")
    before = _tracked_bytes(repository)

    invalid = repository.run_hook(hook_id, hook_stage="pre-push", all_files=all_files)
    diagnostics = _assert_hook_result(invalid, hook_id, failed=True)
    assert invalid_path in diagnostics, diagnostics
    assert "while parsing a flow sequence" in diagnostics, diagnostics
    assert vendor_control not in diagnostics, diagnostics
    assert maintained_control not in diagnostics, diagnostics
    assert _tracked_bytes(repository) == before
    assert repository.git("diff", "--name-only") == ""

    # Repair the input, not hook policy. A successful second invocation proves
    # the failure came from YAML validation rather than setup or stage filtering.
    corrected = b"policy: [corrected]\n"
    repository.write(invalid_path, corrected)
    repository.git("add", "--", invalid_path)
    valid = repository.run_hook(hook_id, hook_stage="pre-push", all_files=all_files)
    _assert_hook_result(valid, hook_id, failed=False)
    assert _tracked_bytes(repository) == {**before, invalid_path: corrected}
    assert repository.git("diff", "--name-only") == ""


@pytest.mark.parametrize("missing", REQUIRED_EXECUTABLES)
def test_prerequisites_missing_tool_fails_actionably(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, missing: str
) -> None:
    """Each missing executable fails before versions, network setup, or hook execution."""
    for executable in REQUIRED_EXECUTABLES:
        if executable != missing:
            resolved = shutil.which(executable)
            assert resolved is not None, (
                f"Install prerequisite {executable} before testing."
            )
            (tmp_path / executable).symlink_to(resolved)
    monkeypatch.setenv("PATH", str(tmp_path))
    with pytest.raises(
        RuntimeError, match=f"Missing prerequisite: {missing}"
    ) as caught:
        _verify_prerequisites(tmp_path, dict(os.environ))
    assert f"Install {missing}" in str(caught.value)
    assert "go.mod" in str(caught.value)


def test_prerequisites_old_python_fails_actionably() -> None:
    """Reject unsupported interpreters rather than silently weakening prerequisites."""
    _require_python_version(MINIMUM_PYTHON_VERSION)
    with pytest.raises(RuntimeError, match="Python >=3.10 is required") as caught:
        _require_python_version((3, 9, 0))
    assert "python3 -m pytest" in str(caught.value)


def test_command_missing_executable_reports_actionable_failure(tmp_path: Path) -> None:
    """A missing tool at invocation time is a failure, not a skipped hook."""
    with pytest.raises(RuntimeError, match="Cannot execute") as caught:
        _run_command(["git", "--version"], cwd=tmp_path, env={"PATH": str(tmp_path)})
    assert "Install the required executable" in str(caught.value)
    assert str(tmp_path) in str(caught.value)


def test_command_timeout_reports_bounded_actionable_failure(tmp_path: Path) -> None:
    """Exercise the real timeout path without sleeping for a hook setup timeout."""
    with pytest.raises(RuntimeError, match="Timed out after 0.1s") as caught:
        _run_command(
            [
                sys.executable,
                "-c",
                "import time; print('started', flush=True); time.sleep(30)",
            ],
            cwd=tmp_path,
            env=dict(os.environ),
            timeout=0.1,
        )
    assert "pinned hook setup" in str(caught.value)
    assert "stdout=" in str(caught.value)


def test_setup_nonzero_exit_preserves_diagnostics(tmp_path: Path) -> None:
    """Failed prerequisite probes retain the command, status, and stderr."""
    with pytest.raises(RuntimeError, match="exit 7") as caught:
        _checked_command(
            [
                sys.executable,
                "-c",
                "import sys; sys.stderr.write('probe failed'); sys.exit(7)",
            ],
            cwd=tmp_path,
            env=dict(os.environ),
        )
    assert "probe failed" in str(caught.value)
    assert "installed prerequisites" in str(caught.value)


def test_prerequisites_python_without_pytest_fails_actionably(tmp_path: Path) -> None:
    """A Python interpreter without site packages must fail its real pytest probe."""
    env = dict(os.environ)
    env.pop("PYTHONPATH", None)
    with pytest.raises(RuntimeError, match="installed prerequisites") as caught:
        _checked_command(
            [sys.executable, "-S", "-m", "pytest", "--version"], cwd=tmp_path, env=env
        )
    assert "No module named pytest" in str(caught.value)


def test_fixture_write_escaping_path_is_rejected(
    hook_repository: HookRepository,
) -> None:
    """Fixture helpers cannot write into a sibling directory or the source checkout."""
    with pytest.raises(ValueError, match="escapes temporary repository"):
        hook_repository.write("../escaped.txt", b"must not be written\n")
    assert not (hook_repository.path.parent / "escaped.txt").exists()

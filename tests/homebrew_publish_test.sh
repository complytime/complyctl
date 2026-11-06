#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Exercise the release publishing step offline, including its Git credential helper.
# Requires Bash, Python 3, Git, and gh; no credentials or network access are needed.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly REPO_ROOT

python3 - "${1:-${REPO_ROOT}/.github/workflows/release.yml}" <<'PY'
import json
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap

TIMEOUT = 20
TAG = "v1.2.0"
BRANCH = "formula/complyctl-1.2.0"
TOKEN = "offline-credential-not-a-real-token"
PR_URL = "https://github.com/complytime/homebrew-tap/pull/123"
STEP_NAME = "Publish Homebrew formula"
TEMP_DIRECTORY = "/tmp/opencode" if pathlib.Path("/tmp/opencode").is_dir() else None
EXPRESSIONS = {
    "${{ steps.homebrew-token.outputs.token }}": TOKEN,
    "${{ needs.preflight.outputs.tag }}": TAG,
}


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def extract_step(workflow):
    # This intentionally supports only the workflow's current narrow YAML shape.
    # Reject drift rather than execute an accidentally selected or partial script.
    matches = list(re.finditer(
        rf"^(?P<indent> +)- name: {re.escape(STEP_NAME)}\n", workflow, re.MULTILINE
    ))
    require(len(matches) == 1, "expected exactly one publishing step")
    match = matches[0]
    indent = match.group("indent")
    remainder = workflow[match.end():]
    boundary = re.search(rf"^ {{0,{len(indent)}}}\S", remainder, re.MULTILINE)
    step = remainder[:boundary.start()] if boundary else remainder
    shape = re.fullmatch(
        rf"{indent}  env:\n(?P<env>(?:{indent}      [^\n]+\n)+)"
        rf"{indent}  run: \|\n(?P<script>.*)", step, re.DOTALL
    )
    require(shape is not None, "unsupported publishing step shape")
    environment = {}
    for line in shape.group("env").splitlines():
        key, value = line.strip().split(": ", 1)
        require(key in {"GH_TOKEN", "TAP_TOKEN", "TAG"}, "unexpected step environment key")
        require(key not in environment, "duplicate step environment key")
        require(value in EXPRESSIONS, "unexpected workflow expression")
        environment[key] = EXPRESSIONS[value]
    require(environment.get("TAG") == TAG, "missing tag environment")
    script = textwrap.dedent(shape.group("script"))
    require("${{" not in script, "unresolved workflow expression in script")
    require(all(not line or line.startswith(indent + "      ")
                for line in shape.group("script").splitlines()), "invalid run indentation")
    return environment, script


# Only these commands can leave the test process. All Git transport and GitHub
# API operations terminate here; the credential protocol itself uses real tools.
STUB = r'''#!/usr/bin/python3
import json
import os
import pathlib
import subprocess
import sys

TIMEOUT = 10
root = pathlib.Path(os.environ["TEST_ROOT"])
command = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
events_path = root / "events.jsonl"

def event(name):
    with events_path.open("a", encoding="utf-8") as output:
        output.write(json.dumps(name) + "\n")

def fail(message):
    print(message, file=sys.stderr)
    sys.exit(1)

def real_git(arguments, **kwargs):
    return subprocess.run([os.environ["REAL_GIT"], *arguments],
                          timeout=TIMEOUT, capture_output=True, text=True, **kwargs)

if command == "curl":
    expected = ["-sL", "-o", "/tmp/source.tar.gz", "-w", "%{http_code}",
                "--fail", "--retry", "3",
                "https://github.com/complytime/complyctl/archive/refs/tags/v1.2.0.tar.gz"]
    if args != expected:
        fail("unexpected download command")
    event("download")
    print("200")
elif command == "sha256sum" and args == ["/tmp/source.tar.gz"]:
    print("a" * 64 + "  /tmp/source.tar.gz")
elif command == "gh":
    if os.environ.get("GH_TOKEN") != os.environ["TEST_TOKEN"]:
        fail("API command missing step credential")
    if args == ["repo", "clone", "complytime/homebrew-tap", "tap-repo", "--", "--depth", "1"]:
        event("clone")
        for git_args in (["init", "-q", "-b", "main", "tap-repo"],
                         ["-C", "tap-repo", "remote", "add", "origin",
                          "https://github.com/complytime/homebrew-tap.git"]):
            if real_git(git_args).returncode:
                fail("local clone fixture failed")
    elif args == ["pr", "list", "--repo", "complytime/homebrew-tap", "--head",
                  os.environ["TEST_BRANCH"], "--state", "open", "--json", "number",
                  "--jq", ".[0].number // empty"]:
        event("pr-list")
        if os.environ["TEST_SCENARIO"] == "existing-pr":
            print("123")
    elif args == ["auth", "setup-git"]:
        event("setup-helper")
        helper_environment = {**os.environ, "PATH": str(root / "helper-bin")}
        result = subprocess.run([os.environ["REAL_GH"], *args], env=helper_environment,
                                timeout=TIMEOUT, capture_output=True, text=True)
        if result.returncode:
            fail("credential helper setup failed")
    elif args == ["pr", "create", "--repo", "complytime/homebrew-tap", "--title",
                  "complyctl 1.2.0", "--body", "Automated formula update for complyctl 1.2.0.\n\n"
                  "Source: https://github.com/complytime/complyctl/releases/tag/v1.2.0",
                  "--base", "main", "--head", os.environ["TEST_BRANCH"]]:
        event("pr-create")
        print(os.environ["TEST_PR_URL"])
    else:
        fail("unexpected GitHub command")
elif command == "git":
    if args == ["push", "origin", os.environ["TEST_BRANCH"]]:
        event("push")
        events = [json.loads(line) for line in events_path.read_text().splitlines()]
        if "setup-helper" not in events:
            fail("missing credential helper setup")
        # Reset all inherited helpers and disable prompts. gh reads GH_TOKEN
        # in the helper subprocess, not in the earlier API command's process.
        helper = "!" + os.environ["REAL_GH"] + " auth git-credential"
        configured = real_git(["config", "--global", "--get-all",
                               "credential.https://github.com.helper"])
        if configured.returncode or configured.stdout.splitlines() != ["", helper]:
            fail("unexpected GitHub credential helper configuration")
        result = real_git(["-c", "credential.helper=", "-c",
                           "credential.https://github.com.helper=" + helper,
                           "credential", "fill"],
                          input="protocol=https\nhost=github.com\n\n")
        if result.returncode:
            fail("Git credential helper could not resolve publishing credential")
        credential = dict(line.split("=", 1) for line in result.stdout.splitlines() if "=" in line)
        if credential.get("password") != os.environ["TEST_TOKEN"]:
            fail("Git credential helper could not resolve publishing credential")
        event("credential-resolved")
        if os.environ["TEST_SCENARIO"] == "push-failure":
            fail("simulated push transport failure")
    elif (args in (["config", "user.name", "github-actions[bot]"],
                   ["config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com"],
                   ["checkout", "-b", os.environ["TEST_BRANCH"]],
                   ["add", "Formula/complyctl.rb"], ["commit", "-m", "complyctl 1.2.0"])):
        event(args[0])
        result = real_git(args)
        if result.returncode:
            fail("local Git operation failed")
    else:
        fail("unexpected Git command")
else:
    fail("unexpected external command")
'''


def run_step(environment, script, scenario):
    with tempfile.TemporaryDirectory(prefix="homebrew-publish-", dir=TEMP_DIRECTORY) as directory:
        root = pathlib.Path(directory)
        binaries = root / "bin"
        binaries.mkdir()
        helper_binaries = root / "helper-bin"
        helper_binaries.mkdir()
        (helper_binaries / "git").symlink_to(shutil.which("git"))
        home = root / "home"
        home.mkdir()
        stub = root / "stub.py"
        stub.write_text(STUB.replace("#!/usr/bin/python3", "#!" + sys.executable, 1),
                        encoding="utf-8")
        stub.chmod(0o700)
        for command in ("curl", "sha256sum", "gh", "git"):
            (binaries / command).symlink_to(stub)
        for command in ("bash", "cat", "awk", "sed", "mkdir", "cp"):
            executable = shutil.which(command)
            require(executable is not None, f"required command not found: {command}")
            (binaries / command).symlink_to(executable)
        process_environment = {
            "PATH": str(binaries), "HOME": str(home), "XDG_CONFIG_HOME": str(home / ".config"),
            "GH_CONFIG_DIR": str(home / "gh"), "GH_PROMPT_DISABLED": "1",
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": str(home / ".gitconfig"),
            "GIT_TERMINAL_PROMPT": "0", "GIT_ASKPASS": "/bin/false", "LC_ALL": "C",
            "GIT_ALLOW_PROTOCOL": "", "TEST_ROOT": str(root), "TEST_TOKEN": TOKEN,
            "TEST_SCENARIO": scenario, "TEST_BRANCH": BRANCH, "TEST_PR_URL": PR_URL,
            "REAL_GIT": shutil.which("git"), "REAL_GH": shutil.which("gh"),
            **environment,
        }
        result = subprocess.run([shutil.which("bash"), "--noprofile", "--norc", "-c", script],
                                cwd=root, env=process_environment, capture_output=True,
                                text=True, timeout=TIMEOUT, check=False)
        output = result.stdout + result.stderr
        require(TOKEN not in output, "credential leaked to subprocess output")
        # Inspect both the global and repository configs, even on failed runs.
        for config in (home / ".gitconfig", root / "tap-repo/.git/config", home / "gh/hosts.yml"):
            require(not config.exists() or TOKEN not in config.read_text(),
                    "credential persisted in configuration")
        events_file = root / "events.jsonl"
        events = ([json.loads(line) for line in events_file.read_text().splitlines()]
                  if events_file.exists() else [])
        formula = root / "tap-repo/Formula/complyctl.rb"
        return result.returncode, output, events, formula.read_text() if formula.exists() else ""


def assert_success(environment, script):
    code, output, events, formula = run_step(environment, script, "success")
    require(code == 0, "new-formula success assertion failed: " + output)
    require(events == ["download", "clone", "pr-list", "config", "config", "checkout",
                       "add", "commit", "setup-helper", "push", "credential-resolved", "pr-create"],
            "unexpected success operations")
    require(f"::notice::Opened {PR_URL}" in output, "missing PR success notice")
    require(f"/refs/tags/{TAG}.tar.gz" in formula and 'sha256 "' + "a" * 64 + '"' in formula,
            "formula source or checksum mismatch")


def main():
    for command in ("bash", "python3", "git", "gh"):
        require(shutil.which(command) is not None, f"required command not found: {command}")
    workflow = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
    environment, script = extract_step(workflow)
    for invalid_workflow in (
        workflow.replace(STEP_NAME, "Missing publishing step"),
        workflow + workflow,
        workflow.replace("${{ steps.homebrew-token.outputs.token }}", "${{ secrets.UNKNOWN }}"),
        workflow.replace("              run: |", "              run: >"),
    ):
        try:
            extract_step(invalid_workflow)
        except AssertionError:
            pass
        else:
            raise AssertionError("unsupported publishing step shape was accepted")
    print("PASS: missing, ambiguous, or unsupported workflow shapes fail closed")
    assert_success(environment, script)
    print("PASS: new formula uses real Git/gh credential resolution and opens a PR")

    code, output, events, _ = run_step(environment, script, "existing-pr")
    require(code == 0 and "already exists" in output, "existing PR did not exit successfully")
    require(events == ["download", "clone", "pr-list"], "existing PR performed write operations")
    print("PASS: existing open PR is a no-op")

    code, output, events, _ = run_step(environment, script, "push-failure")
    require(code != 0 and "simulated push transport failure" in output,
            "push failure was not propagated")
    require("credential-resolved" in events and "pr-create" not in events
            and "::notice::Opened" not in output, "failed push reported PR success")
    print("PASS: push failure prevents PR creation and success notice")

    # Mutate the actual YAML, not a copied script, to reproduce the original
    # per-command API assignments that left the Git helper without GH_TOKEN.
    require("GH_TOKEN" in environment and "TAP_TOKEN" not in environment,
            "publishing environment must export GH_TOKEN")
    mutated = workflow.replace("GH_TOKEN: ${{ steps.homebrew-token.outputs.token }}",
                               "TAP_TOKEN: ${{ steps.homebrew-token.outputs.token }}")
    mutated = re.sub(r"\bgh (?=repo clone|pr list|auth setup-git|pr create)",
                     'GH_TOKEN="${TAP_TOKEN}" gh ', mutated)
    with tempfile.TemporaryDirectory(prefix="homebrew-mutation-", dir=TEMP_DIRECTORY) as directory:
        fixture = pathlib.Path(directory) / "release.yml"
        fixture.write_text(mutated, encoding="utf-8")
        legacy_environment, legacy_script = extract_step(fixture.read_text(encoding="utf-8"))
        code, output, events, _ = run_step(legacy_environment, legacy_script, "success")
        require(code != 0 and "Git credential helper could not resolve" in output,
                "TAP_TOKEN-only mutation did not fail at credential lookup")
        require("push" in events and "pr-create" not in events and "::notice::Opened" not in output,
                "legacy mutation did not reach push or falsely reported success")
        try:
            assert_success(legacy_environment, legacy_script)
        except AssertionError as error:
            require("new-formula success assertion failed" in str(error),
                    "mutation failed an unrelated assertion")
        else:
            raise AssertionError("TAP_TOKEN-only mutation passed the success assertion")
    print("PASS: TAP_TOKEN-only mutation fails the new-formula success assertion")

    missing_environment = {key: value for key, value in environment.items() if key != "GH_TOKEN"}
    code, output, events, _ = run_step(missing_environment, script, "success")
    require(code != 0 and "missing step credential" in output and "pr-create" not in events,
            "missing credential did not fail noninteractively")
    print("PASS: missing credentials fail without cached host credentials")


try:
    main()
except (AssertionError, OSError, subprocess.TimeoutExpired) as error:
    # Never include captured credential protocol output in diagnostics.
    print("FAIL: " + str(error).replace(TOKEN, "[redacted]"), file=sys.stderr)
    sys.exit(1)
PY

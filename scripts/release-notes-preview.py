#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Render the canonical release delta without publishing artifacts."""

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from urllib.parse import quote, urlparse

try:
    import yaml
except ImportError:
    yaml = None


UPSTREAM = "complytime/complyctl"
API_ROOT = f"repos/{UPSTREAM}"
MIN_VERSION = (2, 20, 0)
MIN_PYTHON_VERSION = (3, 9)
COMMAND_TIMEOUT = 60
RENDER_TIMEOUT = 180
SKIP_PIPELINES = "publish,before,sign,sbom,archive,validate"
SHA_PATTERN = re.compile(r"[0-9a-f]{40,64}")
TAG_PATTERN = re.compile(r"[A-Za-z0-9][A-Za-z0-9._/-]*")


class PreviewError(Exception):
    """An actionable preview failure, without child process output."""


def renderer_failure_hint(output):
    """Translate renderer errors into safe diagnostics without echoing child output."""
    output = output.lower()
    if "rate limit" in output:
        return "GitHub rate limit reached; retry after the limit resets."
    if any(message in output for message in ("bad credentials", "unauthorized", "403")):
        return "Check GitHub authentication and repository permissions."
    if any(message in output for message in ("not found", "404")):
        return "Check that the release and candidate are available on GitHub."
    if any(message in output for message in ("configuration", "config", "yaml")):
        return "Check the GoReleaser changelog configuration."
    return "Check GitHub access and the GoReleaser changelog configuration."


def run(args, purpose, *, env=None, timeout=COMMAND_TIMEOUT, diagnose_renderer=False):
    """Run a bounded command without exposing its potentially secret output."""
    try:
        result = subprocess.run(
            args, capture_output=True, text=True, env=env, timeout=timeout, check=True
        )
    except FileNotFoundError as error:
        raise PreviewError(f"Missing {args[0]}; install it and retry.") from error
    except subprocess.TimeoutExpired as error:
        raise PreviewError(
            f"{purpose} timed out; check connectivity and retry."
        ) from error
    except subprocess.CalledProcessError as error:
        if diagnose_renderer:
            hint = renderer_failure_hint(f"{error.stdout or ''}\n{error.stderr or ''}")
            raise PreviewError(f"{purpose} failed: {hint}") from error
        raise PreviewError(
            f"{purpose} failed; check prerequisites and retry."
        ) from error
    except UnicodeError as error:
        raise PreviewError(f"{purpose} returned invalid text; retry.") from error
    return result.stdout.strip()


def check_prerequisites():
    if sys.version_info < MIN_PYTHON_VERSION:
        raise PreviewError("Python 3.9 or newer is required; upgrade Python and retry.")
    if yaml is None:
        raise PreviewError("Missing PyYAML; install it for Python 3 and retry.")
    for tool in ("git", "gh", "goreleaser"):
        if shutil.which(tool) is None:
            raise PreviewError(f"Missing {tool}; install it and retry.")
    version = run(["goreleaser", "--version"], "GoReleaser version check")
    match = re.search(r"(?m)^GitVersion:\s*v?(\d+)\.(\d+)\.(\d+)\s*$", version)
    if (
        not match
        or tuple(map(int, match.groups())) < MIN_VERSION
        or match.group(1) != "2"
    ):
        raise PreviewError("Install open-source GoReleaser v2.20.0 or newer (v2 only).")


def api_json(endpoint, purpose):
    response = run(["gh", "api", "--hostname", "github.com", endpoint], purpose)
    try:
        value = json.loads(response)
    except json.JSONDecodeError as error:
        raise PreviewError(f"{purpose} returned invalid JSON; retry.") from error
    if not isinstance(value, dict):
        raise PreviewError(f"{purpose} returned an invalid response; retry.")
    return value


def discover_comparison():
    release = api_json(
        f"{API_ROOT}/releases/latest", "Latest upstream release discovery"
    )
    tag = release.get("tag_name")
    if (
        not isinstance(tag, str)
        or not TAG_PATTERN.fullmatch(tag)
        or ".." in tag
        or release.get("draft") is not False
        or release.get("prerelease") is not False
    ):
        raise PreviewError(
            "Latest upstream release is invalid or not stable; check GitHub releases."
        )
    head = run(["git", "rev-parse", "--verify", "HEAD^{commit}"], "HEAD resolution")
    if not SHA_PATTERN.fullmatch(head):
        raise PreviewError("Git returned an invalid HEAD commit; check the checkout.")
    endpoint = f"{API_ROOT}/compare/{quote(tag, safe='')}...{head}"
    comparison = api_json(
        endpoint, "Upstream comparison (push the candidate commit first)"
    )
    status = comparison.get("status")
    if status not in ("ahead", "identical"):
        raise PreviewError(
            "Candidate is behind, diverged, or unavailable; update and push the candidate "
            "to GitHub before retrying."
        )
    return tag, head, status


def canonical_url(url):
    """Accept only GitHub URLs identifying the canonical repository."""
    if re.fullmatch(
        rf"(?:[^@/:]+@)?github\.com:{re.escape(UPSTREAM)}(?:\.git)?/?", url
    ):
        return True
    try:
        parsed = urlparse(url)
        return (
            parsed.scheme in ("https", "ssh", "git")
            and parsed.hostname == "github.com"
            and parsed.port is None
            and not parsed.query
            and not parsed.fragment
            and parsed.path.rstrip("/") in (f"/{UPSTREAM}", f"/{UPSTREAM}.git")
        )
    except ValueError:
        return False


def canonical_remote(env):
    remotes = run(["git", "remote"], "Git remote discovery", env=env).splitlines()
    canonical = []
    for remote in remotes:
        url = run(
            ["git", "remote", "get-url", remote], "Git remote resolution", env=env
        )
        if canonical_url(url):
            canonical.append(remote)
    if not canonical:
        raise PreviewError(f"Add a Git remote for github.com/{UPSTREAM} and retry.")
    for preferred in ("upstream", "origin"):
        if preferred in canonical:
            return preferred
    return sorted(canonical)[0]


def renderer_environment(tag, head, token):
    env = os.environ.copy()
    env.update(
        GORELEASER_PREVIOUS_TAG=tag,
        GORELEASER_CURRENT_TAG=head,
        GITHUB_TOKEN=token,
    )
    remote = canonical_remote(env)
    branch = run(["git", "branch", "--show-current"], "Git branch discovery", env=env)
    if branch:
        try:
            count = int(env.get("GIT_CONFIG_COUNT", "0"))
            if count < 0:
                raise ValueError
        except ValueError as error:
            raise PreviewError(
                "Invalid GIT_CONFIG_COUNT; correct the Git environment and retry."
            ) from error
        env[f"GIT_CONFIG_KEY_{count}"] = f"branch.{branch}.remote"
        env[f"GIT_CONFIG_VALUE_{count}"] = remote
        env["GIT_CONFIG_COUNT"] = str(count + 1)
    effective_url = run(
        ["git", "ls-remote", "--get-url"], "Effective Git remote verification", env=env
    )
    if not canonical_url(effective_url):
        raise PreviewError(
            "Default Git remote is not canonical; use a branch or configure origin."
        )
    return env


def load_release_config(path):
    try:
        config = yaml.safe_load(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, yaml.YAMLError) as error:
        raise PreviewError(
            "Cannot read .goreleaser.yaml; fix the release configuration."
        ) from error
    if (
        not isinstance(config, dict)
        or config.get("version") != 2
        or not isinstance(config.get("project_name"), str)
        or not config["project_name"]
        or not isinstance(config.get("changelog"), dict)
        or not config["changelog"]
        or config["changelog"].get("use") != "github"
    ):
        raise PreviewError(
            "Invalid GoReleaser v2 GitHub changelog configuration; fix it and retry."
        )
    return config


def preview_config(path, directory):
    config = load_release_config(path)
    return {
        "version": config["version"],
        "project_name": config["project_name"],
        "dist": str(directory / "dist"),
        "builds": [{"id": "preview", "skip": "true"}],
        "checksum": {"disable": True},
        "release": {"github": dict(zip(("owner", "name"), UPSTREAM.split("/")))},
        "changelog": config["changelog"],
    }


def render(tag, head, token):
    env = renderer_environment(tag, head, token)
    temp_root = Path(tempfile.gettempdir()).resolve()
    if temp_root.is_relative_to(Path.cwd().resolve()):
        raise PreviewError(
            "Temporary directory is inside the checkout; set TMPDIR outside it."
        )
    with tempfile.TemporaryDirectory(prefix="complyctl-release-preview-") as temp:
        directory = Path(temp).resolve()
        config_path = directory / "preview.yaml"
        config = preview_config(Path(".goreleaser.yaml"), directory)
        config_path.write_text(
            yaml.safe_dump(config, sort_keys=False), encoding="utf-8"
        )
        run(
            [
                "goreleaser",
                "release",
                "--config",
                str(config_path),
                f"--skip={SKIP_PIPELINES}",
            ],
            "GoReleaser changelog generation",
            env=env,
            timeout=RENDER_TIMEOUT,
            diagnose_renderer=True,
        )
        try:
            return (directory / "dist" / "CHANGELOG.md").read_text(encoding="utf-8")
        except (OSError, UnicodeError) as error:
            raise PreviewError(
                "GoReleaser did not create the changelog; check its configuration."
            ) from error


def main():
    if len(sys.argv) != 1:
        raise PreviewError(
            "No arguments are supported; the latest upstream release is automatic."
        )
    check_prerequisites()
    load_release_config(Path(".goreleaser.yaml"))
    token = run(
        ["gh", "auth", "token", "--hostname", "github.com"],
        "GitHub authentication (run gh auth login)",
    )
    if not token:
        raise PreviewError(
            "GitHub authentication returned no token; run gh auth login."
        )
    tag, head, status = discover_comparison()
    print(
        f"Baseline: {tag}; candidate: {head} (committed HEAD only; uncommitted changes excluded).",
        file=sys.stderr,
    )
    if status == "identical":
        print("No unreleased commits.", file=sys.stderr)
        return
    sys.stdout.write(render(tag, head, token))


if __name__ == "__main__":
    try:
        main()
    except PreviewError as error:
        print(f"release-notes-preview: {error}", file=sys.stderr)
        sys.exit(1)
    except OSError:
        print(
            "release-notes-preview: temporary file operation failed; check permissions.",
            file=sys.stderr,
        )
        sys.exit(1)

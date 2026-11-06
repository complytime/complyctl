#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Offline regression tests for the release notes preview coordinator."""

from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Any
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "release_notes_preview", ROOT / "scripts" / "release-notes-preview.py"
)
preview = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(preview)


class PrerequisiteTests(unittest.TestCase):
    def test_missing_tools_yaml_and_unsupported_versions(self) -> None:
        with mock.patch.object(preview.sys, "version_info", (3, 8)):
            with self.assertRaisesRegex(preview.PreviewError, "Python 3.9"):
                preview.check_prerequisites()
        with mock.patch.object(preview, "yaml", None):
            with self.assertRaisesRegex(preview.PreviewError, "PyYAML"):
                preview.check_prerequisites()
        for missing in ("git", "gh", "goreleaser"):
            with self.subTest(missing=missing):
                with mock.patch.object(
                    preview.shutil,
                    "which",
                    side_effect=lambda tool: None if tool == missing else tool,
                ):
                    with self.assertRaisesRegex(preview.PreviewError, missing):
                        preview.check_prerequisites()
        with mock.patch.object(preview.shutil, "which", return_value="tool"):
            for version in (
                "GitVersion: 2.19.9",
                "GitVersion: 3.0.0",
                "GitVersion: 2.20.0-rc1",
                "invalid",
            ):
                with (
                    self.subTest(version=version),
                    mock.patch.object(preview, "run", return_value=version),
                ):
                    with self.assertRaisesRegex(preview.PreviewError, "v2.20.0"):
                        preview.check_prerequisites()
            with mock.patch.object(preview, "run", return_value="GitVersion: 2.20.0"):
                preview.check_prerequisites()

    def test_failures_and_timeouts_never_print_child_output(self) -> None:
        secret = "secret-token"
        failures = (
            subprocess.CalledProcessError(1, ["gh"], output=secret, stderr=secret),
            subprocess.TimeoutExpired(["gh"], 1, output=secret, stderr=secret),
            FileNotFoundError(),
        )
        for failure in failures:
            with self.subTest(failure=type(failure).__name__):
                with mock.patch.object(preview.subprocess, "run", side_effect=failure):
                    with self.assertRaises(preview.PreviewError) as context:
                        preview.run(
                            ["gh", "auth", "token"],
                            "GitHub authentication (run gh auth login)",
                        )
                    self.assertNotIn(secret, str(context.exception))

    def test_renderer_error_is_actionable_without_exposing_output(self) -> None:
        cases = (
            ("rate limit exceeded secret-token", "rate limit"),
            ("bad credentials secret-token", "authentication"),
            ("404 secret-token", "available on GitHub"),
            ("invalid yaml secret-token", "configuration"),
            ("unknown failure secret-token", "GitHub access"),
        )
        for output, hint in cases:
            failure = subprocess.CalledProcessError(1, ["goreleaser"], stderr=output)
            with (
                self.subTest(output=output),
                mock.patch.object(preview.subprocess, "run", side_effect=failure),
            ):
                with self.assertRaises(preview.PreviewError) as context:
                    preview.run(["goreleaser"], "Renderer", diagnose_renderer=True)
                self.assertIn(hint, str(context.exception))
                self.assertNotIn("secret-token", str(context.exception))

    def test_api_failure_hints_are_specific_and_secret_safe(self) -> None:
        cases = (
            (
                "repos/complytime/complyctl/releases/latest",
                "HTTP 404",
                "stable release",
            ),
            (
                "repos/complytime/complyctl/compare/v1...abc",
                "HTTP 404",
                "push the candidate",
            ),
            ("latest", "HTTP 401", "authentication"),
            ("latest", "rate limit exceeded", "rate limit"),
        )
        for endpoint, output, hint in cases:
            failure = subprocess.CalledProcessError(
                1, ["gh"], stderr=f"{output} secret-token"
            )
            with (
                self.subTest(output=output, endpoint=endpoint),
                mock.patch.object(preview.subprocess, "run", side_effect=failure),
            ):
                with self.assertRaises(preview.PreviewError) as context:
                    preview.api_json(endpoint, "GitHub request")
                self.assertIn(hint, str(context.exception))
                self.assertNotIn("secret-token", str(context.exception))

    def test_subprocess_receives_bounded_timeout(self) -> None:
        result = subprocess.CompletedProcess(["git"], 0, stdout="result")
        with mock.patch.object(
            preview.subprocess, "run", return_value=result
        ) as command:
            self.assertEqual(preview.run(["git"], "check"), "result")
            self.assertEqual(
                command.call_args.kwargs["timeout"], preview.COMMAND_TIMEOUT
            )
            self.assertTrue(command.call_args.kwargs["check"])
            preview.run(["git"], "check", timeout=preview.RENDER_TIMEOUT)
            self.assertEqual(
                command.call_args.kwargs["timeout"], preview.RENDER_TIMEOUT
            )


class DiscoveryTests(unittest.TestCase):
    head = "a" * 40

    def test_latest_release_and_comparison_use_canonical_endpoints(self) -> None:
        release = json.dumps(
            {"tag_name": "v1.2.0", "draft": False, "prerelease": False}
        )
        for status in ("ahead", "identical"):
            with self.subTest(status=status):
                with mock.patch.object(
                    preview,
                    "run",
                    side_effect=[release, self.head, json.dumps({"status": status})],
                ) as run:
                    self.assertEqual(
                        preview.discover_comparison(), ("v1.2.0", self.head, status)
                    )
                self.assertEqual(
                    run.call_args_list[0].args[0],
                    [
                        "gh",
                        "api",
                        "--hostname",
                        "github.com",
                        "repos/complytime/complyctl/releases/latest",
                    ],
                )
                self.assertEqual(
                    run.call_args_list[2].args[0][-1],
                    f"repos/complytime/complyctl/compare/v1.2.0...{self.head}",
                )

    def test_invalid_releases_comparisons_and_responses_fail(self) -> None:
        invalid_releases = [
            "[]",
            "bad-json",
            "{}",
            json.dumps({"tag_name": "../bad", "draft": False, "prerelease": False}),
            json.dumps({"tag_name": "v1", "draft": False, "prerelease": True}),
        ]
        for release in invalid_releases:
            with (
                self.subTest(release=release),
                mock.patch.object(preview, "run", return_value=release),
            ):
                with self.assertRaises(preview.PreviewError):
                    preview.discover_comparison()
        valid = json.dumps({"tag_name": "v1.2.0", "draft": False, "prerelease": False})
        for status in ("behind", "diverged", "unknown"):
            with (
                self.subTest(status=status),
                mock.patch.object(
                    preview,
                    "run",
                    side_effect=[valid, self.head, json.dumps({"status": status})],
                ),
            ):
                with self.assertRaisesRegex(preview.PreviewError, "push"):
                    preview.discover_comparison()
        with mock.patch.object(preview, "run", side_effect=[valid, "invalid-sha"]):
            with self.assertRaisesRegex(preview.PreviewError, "HEAD"):
                preview.discover_comparison()
        with mock.patch.object(
            preview,
            "run",
            side_effect=[valid, self.head, preview.PreviewError("push candidate")],
        ):
            with self.assertRaisesRegex(preview.PreviewError, "push"):
                preview.discover_comparison()


class RemoteTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.old_cwd = Path.cwd()
        os.chdir(self.repo)
        self.addCleanup(os.chdir, self.old_cwd)
        self.git("init", "-q", "-b", "opsx/feature")
        self.git("config", "user.name", "Test")
        self.git("config", "user.email", "test@example.com")
        self.git("commit", "-q", "--allow-empty", "-m", "initial")
        self.git("remote", "add", "origin", "https://github.com/fork/complyctl.git")
        self.git(
            "remote", "add", "upstream", "https://github.com/complytime/complyctl.git"
        )

    def git(self, *args: str) -> str:
        return subprocess.run(
            ["git", *args], check=True, capture_output=True, text=True
        ).stdout.strip()

    def test_fork_branch_override_preserves_git_config_and_existing_entries(
        self,
    ) -> None:
        before = (self.repo / ".git" / "config").read_bytes()
        inherited = {
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "test.key",
            "GIT_CONFIG_VALUE_0": "value",
            "GORELEASER_PREVIOUS_TAG": "wrong",
            "GORELEASER_CURRENT_TAG": "wrong",
        }
        with mock.patch.dict(os.environ, inherited):
            env = preview.renderer_environment("v1.2.0", "a" * 40, "token")
        self.assertEqual(env["GIT_CONFIG_COUNT"], "2")
        self.assertEqual(env["GIT_CONFIG_VALUE_0"], "value")
        self.assertEqual(env["GIT_CONFIG_KEY_1"], "branch.opsx/feature.remote")
        self.assertEqual(env["GIT_CONFIG_VALUE_1"], "upstream")
        self.assertEqual(env["GORELEASER_PREVIOUS_TAG"], "v1.2.0")
        self.assertEqual(env["GORELEASER_CURRENT_TAG"], "a" * 40)
        self.assertEqual((self.repo / ".git" / "config").read_bytes(), before)

    def test_detached_fork_fails_and_canonical_origin_succeeds(self) -> None:
        self.git("checkout", "-q", "--detach")
        with self.assertRaisesRegex(preview.PreviewError, "not canonical"):
            preview.renderer_environment("v1", "a" * 40, "token")
        self.git(
            "remote", "set-url", "origin", "git@github.com:complytime/complyctl.git"
        )
        with mock.patch.dict(os.environ, {}, clear=True):
            env = preview.renderer_environment("v1", "a" * 40, "token")
        self.assertNotIn("GIT_CONFIG_COUNT", env)
        self.assertEqual(env["GORELEASER_PREVIOUS_TAG"], "v1")
        self.assertEqual(env["GORELEASER_CURRENT_TAG"], "a" * 40)

    def test_main_branch_prefers_upstream_even_when_origin_is_canonical(self) -> None:
        self.git("branch", "-m", "main")
        self.git(
            "remote", "set-url", "origin", "https://github.com/complytime/complyctl.git"
        )
        env = preview.renderer_environment("v1", "a" * 40, "token")
        index = int(env["GIT_CONFIG_COUNT"]) - 1
        self.assertEqual(env[f"GIT_CONFIG_KEY_{index}"], "branch.main.remote")
        self.assertEqual(env[f"GIT_CONFIG_VALUE_{index}"], "upstream")

    def test_url_validation_and_missing_canonical_remote(self) -> None:
        for url in (
            "https://github.com/complytime/complyctl.git",
            "ssh://git@github.com/complytime/complyctl",
            "git@github.com:complytime/complyctl.git",
        ):
            self.assertTrue(preview.canonical_url(url), url)
        for url in (
            "https://github.com.evil/complytime/complyctl",
            "https://github.com/fork/complyctl",
            "https://github.com:bad/complytime/complyctl",
        ):
            self.assertFalse(preview.canonical_url(url), url)
        self.git("remote", "remove", "upstream")
        with self.assertRaisesRegex(preview.PreviewError, "Add a Git remote"):
            preview.renderer_environment("v1", "a" * 40, "token")


class RenderingTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        old_cwd = Path.cwd()
        os.chdir(self.temp.name)
        self.addCleanup(os.chdir, old_cwd)
        Path(".goreleaser.yaml").write_text(
            "version: 2\nproject_name: example\nchangelog:\n  use: github\n"
        )

    def test_configuration_copies_only_changelog_and_disables_builds(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp)
            source = {
                "version": 2,
                "project_name": "example",
                "changelog": {"use": "github", "groups": [{"title": "Feature"}]},
                "before": {"hooks": ["danger"]},
                "builds": [{"id": "real"}],
            }
            config_path = directory / "config.yaml"
            config_path.write_text(preview.yaml.safe_dump(source))
            result = preview.preview_config(config_path, directory)
            self.assertEqual(result["changelog"], source["changelog"])
            self.assertEqual(result["builds"], [{"id": "preview", "skip": "true"}])
            self.assertEqual(result["checksum"], {"disable": True})
            self.assertNotIn("before", result)
            self.assertEqual(result["dist"], str(directory / "dist"))
            config_path.write_text("version: 2\nproject_name: x\n")
            with self.assertRaisesRegex(preview.PreviewError, "configuration"):
                preview.preview_config(config_path, directory)
            config_path.write_bytes(b"\xff")
            with self.assertRaisesRegex(preview.PreviewError, "configuration"):
                preview.preview_config(config_path, directory)
            invalid_configs = (
                "version: 1\nproject_name: x\nchangelog: {use: github}\n",
                "version: 2\nproject_name: x\nchangelog: {use: git}\n",
                "version: 2\nproject_name: ''\nchangelog: {use: github}\n",
                "- version: 2\n",
                "version: [2\n",
            )
            for invalid in invalid_configs:
                with self.subTest(invalid=invalid):
                    config_path.write_text(invalid)
                    with self.assertRaises(preview.PreviewError):
                        preview.preview_config(config_path, directory)

    def test_renderer_success_failure_and_cleanup(self) -> None:
        observed = []

        def fake_run(args: list[str], purpose: str, **kwargs: Any) -> str:
            config_path = Path(args[3])
            observed.append(config_path.parent)
            config = preview.yaml.safe_load(config_path.read_text())
            self.assertEqual(
                args[-1], "--skip=publish,before,sign,sbom,archive,validate"
            )
            self.assertEqual(kwargs["timeout"], preview.RENDER_TIMEOUT)
            output = Path(config["dist"])
            output.mkdir()
            (output / "CHANGELOG.md").write_text("# Notes\n")
            if len(observed) == 2:
                raise preview.PreviewError("renderer failed")
            return ""

        with (
            mock.patch.object(preview, "renderer_environment", return_value={}),
            mock.patch.object(preview, "run", side_effect=fake_run),
        ):
            self.assertEqual(preview.render("v1", "a" * 40, "token"), "# Notes\n")
            with self.assertRaisesRegex(preview.PreviewError, "renderer failed"):
                preview.render("v1", "a" * 40, "token")
        self.assertTrue(all(not directory.exists() for directory in observed))

    def test_missing_changelog_fails_without_partial_stdout(self) -> None:
        stdout = io.StringIO()
        with (
            contextlib.redirect_stdout(stdout),
            mock.patch.object(preview, "renderer_environment", return_value={}),
            mock.patch.object(preview, "run", return_value=""),
        ):
            with self.assertRaisesRegex(preview.PreviewError, "did not create"):
                preview.render("v1", "a" * 40, "token")
        self.assertEqual(stdout.getvalue(), "")

    def test_temporary_directory_inside_checkout_is_rejected_before_creation(
        self,
    ) -> None:
        with (
            mock.patch.object(preview, "renderer_environment", return_value={}),
            mock.patch.object(
                preview.tempfile, "gettempdir", return_value=str(Path.cwd())
            ),
            mock.patch.object(
                preview.tempfile, "TemporaryDirectory"
            ) as temporary_directory,
        ):
            with self.assertRaisesRegex(preview.PreviewError, "inside the checkout"):
                preview.render("v1", "a" * 40, "token")
            temporary_directory.assert_not_called()

    def test_arguments_and_authentication_fail_before_rendering(self) -> None:
        with mock.patch.object(preview.sys, "argv", ["preview", "v1"]):
            with self.assertRaisesRegex(preview.PreviewError, "No arguments"):
                preview.main()
        with (
            mock.patch.object(preview.sys, "argv", ["preview"]),
            mock.patch.object(preview, "check_prerequisites"),
            mock.patch.object(preview, "run", return_value=""),
            mock.patch.object(preview, "render") as render,
        ):
            with self.assertRaisesRegex(preview.PreviewError, "gh auth login"):
                preview.main()
            render.assert_not_called()

    def test_main_output_channels_and_empty_delta(self) -> None:
        for status in ("ahead", "identical"):
            with (
                self.subTest(status=status),
                mock.patch.object(preview.sys, "argv", ["preview"]),
                mock.patch.object(preview, "check_prerequisites"),
                mock.patch.object(preview, "run", return_value="token"),
                mock.patch.object(
                    preview,
                    "discover_comparison",
                    return_value=("v1", "a" * 40, status),
                ),
                mock.patch.object(
                    preview, "render", return_value="# Notes\n"
                ) as render,
            ):
                stdout, stderr = io.StringIO(), io.StringIO()
                with (
                    contextlib.redirect_stdout(stdout),
                    contextlib.redirect_stderr(stderr),
                ):
                    preview.main()
                self.assertIn("v1", stderr.getvalue())
                self.assertIn("a" * 40, stderr.getvalue())
                self.assertIn("uncommitted changes excluded", stderr.getvalue())
                if status == "identical":
                    self.assertEqual(stdout.getvalue(), "")
                    self.assertIn("No unreleased commits", stderr.getvalue())
                    render.assert_not_called()
                else:
                    self.assertEqual(stdout.getvalue(), "# Notes\n")


class MakeInterfaceTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("make"), "make is not installed")
    def test_phony_targets_help_and_redirectable_output(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp)
            shutil.copy(ROOT / "Makefile", directory / "Makefile")
            (directory / "release-notes-preview").touch()
            (directory / "test-release-notes-preview").touch()
            executable = directory / "python3"
            executable.write_text(
                '#!/bin/sh\nprintf "%s\\n" "$@" > "$ARGS_FILE"\nprintf "# Markdown\\n"\n'
            )
            executable.chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = f"{directory}{os.pathsep}{env['PATH']}"
            env["ARGS_FILE"] = str(directory / "args")
            for target in ("release-notes-preview", "test-release-notes-preview"):
                with self.subTest(target=target):
                    result = subprocess.run(
                        ["make", "--no-print-directory", target],
                        cwd=directory,
                        env=env,
                        check=True,
                        text=True,
                        capture_output=True,
                        timeout=30,
                    )
                    self.assertEqual(result.stdout, "# Markdown\n")
                    self.assertEqual(result.stderr, "")
                    arguments = (directory / "args").read_text().splitlines()
                    if target == "release-notes-preview":
                        self.assertEqual(
                            arguments, ["scripts/release-notes-preview.py"]
                        )
                    else:
                        self.assertEqual(
                            arguments,
                            [
                                "-m",
                                "unittest",
                                "discover",
                                "-s",
                                "tests",
                                "-p",
                                "release_notes_preview_test.py",
                                "-v",
                            ],
                        )
                        self.assertTrue((ROOT / "tests" / arguments[-2]).is_file())
            help_output = subprocess.run(
                ["make", "help"],
                cwd=directory,
                env=env,
                check=True,
                text=True,
                capture_output=True,
                timeout=30,
            ).stdout
            self.assertIn("release-notes-preview", help_output)
            self.assertIn("test-release-notes-preview", help_output)


class EntrypointTests(unittest.TestCase):
    def test_renderer_failure_exits_nonzero_without_partial_notes_or_token(
        self,
    ) -> None:
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp)
            (directory / ".goreleaser.yaml").write_text(
                "version: 2\nproject_name: example\nchangelog:\n  use: github\n"
            )
            scripts = {
                "goreleaser": (
                    'if [ "$1" = "--version" ]; then echo "GitVersion: 2.20.0"; '
                    'else mkdir -p "$(dirname "$3")/dist"; '
                    'echo "partial notes" > "$(dirname "$3")/dist/CHANGELOG.md"; '
                    'echo "bad credentials secret-token" >&2; exit 1; fi'
                ),
                "gh": (
                    'if [ "$1" = "auth" ]; then echo secret-token; '
                    'elif [ "${4##*/}" = "latest" ]; then '
                    'echo \'{"tag_name":"v1","draft":false,"prerelease":false}\'; '
                    'else echo \'{"status":"ahead"}\'; fi'
                ),
                "git": (
                    'case "$1" in '
                    f"rev-parse) echo {'a' * 40} ;; "
                    "branch) echo fixture ;; "
                    'remote) if [ "$#" = 1 ]; then echo upstream; '
                    "else echo https://github.com/complytime/complyctl.git; fi ;; "
                    "ls-remote) echo https://github.com/complytime/complyctl.git ;; esac"
                ),
            }
            for name, body in scripts.items():
                executable = directory / name
                executable.write_text(f"#!/bin/sh\n{body}\n")
                executable.chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = f"{directory}{os.pathsep}{env['PATH']}"
            script = str(ROOT / "scripts/release-notes-preview.py")
            commands = (
                ([sys.executable, script], "authentication"),
                (
                    [
                        sys.executable,
                        "-c",
                        "import runpy, sys; from unittest.mock import patch; "
                        "patch('tempfile.TemporaryDirectory', side_effect=OSError('secret-token'))"
                        ".start(); sys.argv = [sys.argv[1]]; "
                        "runpy.run_path(sys.argv[0], run_name='__main__')",
                        script,
                    ],
                    "temporary file operation failed",
                ),
            )
            for command, expected in commands:
                with self.subTest(expected=expected):
                    result = subprocess.run(
                        command,
                        cwd=directory,
                        env=env,
                        text=True,
                        capture_output=True,
                        timeout=30,
                    )
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(result.stdout, "")
                    self.assertIn("release-notes-preview:", result.stderr)
                    self.assertIn(expected, result.stderr)
                    self.assertNotIn("secret-token", result.stderr)


if __name__ == "__main__":
    unittest.main()

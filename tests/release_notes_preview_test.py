#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Offline regression tests for the release notes preview coordinator."""

import contextlib
import importlib.util
import io
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "release_notes_preview", ROOT / "scripts" / "release-notes-preview.py"
)
preview = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(preview)


class PrerequisiteTests(unittest.TestCase):
    def test_missing_tools_yaml_and_unsupported_versions(self):
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

    def test_failures_and_timeouts_never_print_child_output(self):
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

    def test_renderer_error_is_actionable_without_exposing_output(self):
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


class DiscoveryTests(unittest.TestCase):
    head = "a" * 40

    def test_latest_release_and_comparison_use_canonical_endpoints(self):
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

    def test_invalid_releases_comparisons_and_responses_fail(self):
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
    def setUp(self):
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

    def git(self, *args):
        return subprocess.run(
            ["git", *args], check=True, capture_output=True, text=True
        ).stdout.strip()

    def test_fork_branch_override_preserves_git_config_and_existing_entries(self):
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

    def test_detached_fork_fails_and_canonical_origin_succeeds(self):
        self.git("checkout", "-q", "--detach")
        with self.assertRaisesRegex(preview.PreviewError, "not canonical"):
            preview.renderer_environment("v1", "a" * 40, "token")
        self.git(
            "remote", "set-url", "origin", "git@github.com:complytime/complyctl.git"
        )
        preview.renderer_environment("v1", "a" * 40, "token")

    def test_main_branch_prefers_upstream_even_when_origin_is_canonical(self):
        self.git("branch", "-m", "main")
        self.git(
            "remote", "set-url", "origin", "https://github.com/complytime/complyctl.git"
        )
        env = preview.renderer_environment("v1", "a" * 40, "token")
        index = int(env["GIT_CONFIG_COUNT"]) - 1
        self.assertEqual(env[f"GIT_CONFIG_KEY_{index}"], "branch.main.remote")
        self.assertEqual(env[f"GIT_CONFIG_VALUE_{index}"], "upstream")

    def test_url_validation_and_missing_canonical_remote(self):
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
    def test_configuration_copies_only_changelog_and_disables_builds(self):
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

    def test_renderer_success_failure_and_cleanup(self):
        observed = []

        def fake_run(args, purpose, **kwargs):
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

    def test_missing_changelog_fails_without_partial_stdout(self):
        stdout = io.StringIO()
        with (
            contextlib.redirect_stdout(stdout),
            mock.patch.object(preview, "renderer_environment", return_value={}),
            mock.patch.object(preview, "run", return_value=""),
        ):
            with self.assertRaisesRegex(preview.PreviewError, "did not create"):
                preview.render("v1", "a" * 40, "token")
        self.assertEqual(stdout.getvalue(), "")

    def test_temporary_directory_inside_checkout_is_rejected_before_creation(self):
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

    def test_arguments_and_authentication_fail_before_rendering(self):
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

    def test_main_output_channels_and_empty_delta(self):
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
                self.assertIn("uncommitted changes excluded", stderr.getvalue())
                if status == "identical":
                    self.assertEqual(stdout.getvalue(), "")
                    self.assertIn("No unreleased commits", stderr.getvalue())
                    render.assert_not_called()
                else:
                    self.assertEqual(stdout.getvalue(), "# Notes\n")


class MakeInterfaceTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("make"), "make is not installed")
    def test_phony_targets_help_and_redirectable_output(self):
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp)
            shutil.copy(ROOT / "Makefile", directory / "Makefile")
            (directory / "release-notes-preview").touch()
            (directory / "test-release-notes-preview").touch()
            executable = directory / "python3"
            executable.write_text('#!/bin/sh\nprintf "# Markdown\\n"\n')
            executable.chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = f"{directory}{os.pathsep}{env['PATH']}"
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


if __name__ == "__main__":
    unittest.main()

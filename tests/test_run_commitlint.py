# Copyright 2026 The ARCORIS Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Tests for the repository-local commitlint orchestrator."""

from __future__ import annotations

import contextlib
import importlib.util
import io
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).resolve().parents[1] / "scripts" / "run_commitlint.py"
MODULE_SPEC = importlib.util.spec_from_file_location("run_commitlint", MODULE_PATH)
assert MODULE_SPEC is not None
assert MODULE_SPEC.loader is not None
run_commitlint = importlib.util.module_from_spec(MODULE_SPEC)
sys.modules[MODULE_SPEC.name] = run_commitlint
MODULE_SPEC.loader.exec_module(run_commitlint)


class CommitlintTestCase(unittest.TestCase):
    """Base helper for tests that need a temporary repository tree."""

    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.repo_root = Path(self.temp_dir.name)

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def write(self, relative_path: str, content: str) -> Path:
        path = self.repo_root / relative_path
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
        return path

    def init_git_repo(self) -> None:
        subprocess.run(["git", "init"], cwd=self.repo_root, check=True, capture_output=True)
        subprocess.run(
            ["git", "config", "user.name", "Test User"],
            cwd=self.repo_root,
            check=True,
            capture_output=True,
        )
        subprocess.run(
            ["git", "config", "user.email", "test@example.com"],
            cwd=self.repo_root,
            check=True,
            capture_output=True,
        )

    def commit_file(self, relative_path: str, content: str, message: str) -> str:
        self.write(relative_path, content)
        subprocess.run(
            ["git", "add", relative_path],
            cwd=self.repo_root,
            check=True,
            capture_output=True,
        )
        subprocess.run(
            ["git", "commit", "-m", message],
            cwd=self.repo_root,
            check=True,
            capture_output=True,
        )
        return (
            subprocess.run(
                ["git", "rev-parse", "HEAD"],
                cwd=self.repo_root,
                check=True,
                capture_output=True,
                text=True,
            )
            .stdout.strip()
        )


class CommitLintReportTests(unittest.TestCase):
    def test_commitlint_report_renders_summary_and_success_lines(self) -> None:
        report = run_commitlint.CommitLintReport(
            event_name="pull_request",
            config_path="commitlint.config.cjs",
            description="pull request range a..b",
            commits_checked=2,
            checked_commits=["abc123", "def456"],
            errors=["bad commit subject"],
        )

        self.assertFalse(report.ok())
        summary = "\n".join(report.summary_lines())
        self.assertIn("- Result: failure", summary)
        self.assertIn("## Checked Commits", summary)
        self.assertIn("## Errors", summary)

        ok_report = run_commitlint.CommitLintReport(
            event_name="push",
            config_path="commitlint.config.cjs",
            description="push fallback",
        )
        self.assertTrue(ok_report.ok())
        self.assertEqual(ok_report.success_lines()[0], "Commitlint validation passed.")


class HelperFunctionTests(CommitlintTestCase):
    def test_parse_args_and_build_context_support_defaults_and_overrides(self) -> None:
        args = run_commitlint.parse_args(
            [
                "--event-name",
                "push",
                "--github-sha",
                "abc",
                "--push-before",
                "def",
                "--pr-base-sha",
                "ghi",
                "--pr-head-sha",
                "jkl",
                "--merge-group-base-sha",
                "mno",
                "--repo-root",
                "/tmp/repo",
                "--config-path",
                "custom.config.cjs",
                "--summary-path",
                "summary.md",
            ]
        )

        self.assertEqual(args.event_name, "push")
        self.assertEqual(args.github_sha, "abc")
        self.assertEqual(args.push_before, "def")
        self.assertEqual(args.repo_root, "/tmp/repo")
        self.assertEqual(args.config_path, "custom.config.cjs")
        self.assertEqual(args.summary_path, "summary.md")

        context = run_commitlint.build_context(args)
        self.assertEqual(context.event_name, "push")
        self.assertEqual(context.github_sha, "abc")
        self.assertEqual(context.push_before, "def")

    def test_sha_and_summary_helpers_behave_as_expected(self) -> None:
        self.assertTrue(run_commitlint.is_zero_sha("000000"))
        self.assertFalse(run_commitlint.is_zero_sha(""))
        self.assertFalse(run_commitlint.is_zero_sha("abc123"))

        resolved = run_commitlint.resolve_summary_path(self.repo_root, "reports/summary.md")
        self.assertEqual(resolved, self.repo_root / "reports" / "summary.md")

        absolute = run_commitlint.resolve_summary_path(
            self.repo_root,
            str((self.repo_root / "absolute.md").resolve()),
        )
        self.assertEqual(absolute, (self.repo_root / "absolute.md").resolve())

    def test_default_commitlint_command_uses_local_pinned_binary(self) -> None:
        self.assertEqual(
            run_commitlint.DEFAULT_COMMITLINT_COMMAND,
            ("tools/commitlint/node_modules/.bin/commitlint",),
        )

    def test_write_summary_creates_parent_directory(self) -> None:
        report = run_commitlint.CommitLintReport(
            event_name="push",
            config_path="commitlint.config.cjs",
            description="push fallback",
        )
        summary_path = self.repo_root / "artifacts" / "commitlint.md"

        run_commitlint.write_summary(summary_path, report)

        summary = summary_path.read_text(encoding="utf-8")
        self.assertIn("# Commit Lint Summary", summary)
        self.assertIn("- Result: success", summary)

    def test_run_git_helpers_work_against_real_git_history(self) -> None:
        self.init_git_repo()
        first_sha = self.commit_file("file.txt", "one\n", "feat: first")
        second_sha = self.commit_file("file.txt", "two\n", "fix: second")

        message = run_commitlint.get_commit_message(self.repo_root, second_sha)
        commits = run_commitlint.list_commits_in_range(self.repo_root, first_sha, second_sha)

        self.assertIn("fix: second", message)
        self.assertEqual(commits, [second_sha])

    def test_lint_commit_message_invokes_commitlint_with_commit_message_stdin(self) -> None:
        config_path = self.write("commitlint.config.cjs", "module.exports = {};\n")

        with (
            mock.patch.object(
                run_commitlint,
                "get_commit_message",
                return_value="feat: add test coverage\n",
            ) as get_message,
            mock.patch.object(run_commitlint.subprocess, "run") as mocked_run,
        ):
            run_commitlint.lint_commit_message(
                self.repo_root,
                config_path,
                "abc123",
                commitlint_command=("mocklint",),
            )

        get_message.assert_called_once_with(self.repo_root, "abc123")
        mocked_run.assert_called_once_with(
            ["mocklint", "--config", str(config_path), "--verbose"],
            cwd=self.repo_root,
            check=True,
            input="feat: add test coverage\n",
            text=True,
        )


class PlanSelectionTests(CommitlintTestCase):
    def test_build_plan_for_pull_request_range(self) -> None:
        context = run_commitlint.CommitLintContext(
            event_name="pull_request",
            pr_base_sha="base",
            pr_head_sha="head",
        )

        with mock.patch.object(
            run_commitlint,
            "list_commits_in_range",
            return_value=["a1", "b2"],
        ) as list_commits:
            plan = run_commitlint.build_commitlint_plan(self.repo_root, context)

        list_commits.assert_called_once_with(self.repo_root, "base", "head")
        self.assertEqual(plan.mode, "range")
        self.assertEqual(plan.commit_shas, ["a1", "b2"])
        self.assertIn("pull request range base..head", plan.description)

    def test_build_plan_for_merge_group_range(self) -> None:
        context = run_commitlint.CommitLintContext(
            event_name="merge_group",
            github_sha="head",
            merge_group_base_sha="base",
        )

        with mock.patch.object(
            run_commitlint,
            "list_commits_in_range",
            return_value=["m1"],
        ) as list_commits:
            plan = run_commitlint.build_commitlint_plan(self.repo_root, context)

        list_commits.assert_called_once_with(self.repo_root, "base", "head")
        self.assertEqual(plan.commit_shas, ["m1"])
        self.assertEqual(plan.mode, "range")

    def test_build_plan_for_push_range_and_zero_sha_fallback(self) -> None:
        context = run_commitlint.CommitLintContext(
            event_name="push",
            github_sha="head",
            push_before="before",
        )

        with mock.patch.object(
            run_commitlint,
            "list_commits_in_range",
            return_value=["p1", "p2"],
        ):
            plan = run_commitlint.build_commitlint_plan(self.repo_root, context)

        self.assertEqual(plan.mode, "range")
        self.assertEqual(plan.commit_shas, ["p1", "p2"])

        fallback_context = run_commitlint.CommitLintContext(
            event_name="push",
            github_sha="head",
            push_before="0000000000000000000000000000000000000000",
        )
        fallback_plan = run_commitlint.build_commitlint_plan(self.repo_root, fallback_context)
        self.assertEqual(fallback_plan.mode, "single-commit-fallback")
        self.assertEqual(fallback_plan.commit_shas, ["head"])

    def test_build_plan_falls_back_when_range_has_no_non_merge_commits(self) -> None:
        context = run_commitlint.CommitLintContext(
            event_name="pull_request",
            pr_base_sha="base",
            pr_head_sha="head",
        )

        with mock.patch.object(
            run_commitlint,
            "list_commits_in_range",
            return_value=[],
        ):
            plan = run_commitlint.build_commitlint_plan(self.repo_root, context)

        self.assertEqual(plan.mode, "single-commit-fallback")
        self.assertEqual(plan.commit_shas, ["head"])

    def test_build_plan_rejects_missing_or_unsupported_context(self) -> None:
        with self.assertRaisesRegex(ValueError, "missing required SHA"):
            run_commitlint.build_commitlint_plan(
                self.repo_root,
                run_commitlint.CommitLintContext(event_name="pull_request"),
            )

        with self.assertRaisesRegex(ValueError, "unsupported event"):
            run_commitlint.build_commitlint_plan(
                self.repo_root,
                run_commitlint.CommitLintContext(event_name="workflow_dispatch"),
            )


class ExecutionFlowTests(CommitlintTestCase):
    def test_run_commitlint_plan_invokes_lint_for_each_commit(self) -> None:
        config_path = self.write("commitlint.config.cjs", "module.exports = {};\n")
        plan = run_commitlint.CommitLintPlan(
            event_name="push",
            description="push range before..head",
            commit_shas=["a1", "b2"],
            mode="range",
        )

        with mock.patch.object(run_commitlint, "lint_commit_message") as lint_message:
            report = run_commitlint.run_commitlint_plan(
                self.repo_root,
                config_path,
                plan,
                commitlint_command=("mocklint",),
            )

        self.assertEqual(lint_message.call_count, 2)
        self.assertTrue(report.ok())
        self.assertEqual(report.commits_checked, 2)
        self.assertEqual(report.checked_commits, ["a1", "b2"])

    def test_run_commitlint_requires_existing_config_and_returns_report(self) -> None:
        context = run_commitlint.CommitLintContext(
            event_name="push",
            github_sha="head",
        )

        with self.assertRaisesRegex(FileNotFoundError, "commitlint configuration not found"):
            run_commitlint.run_commitlint(
                self.repo_root,
                context,
                config_path="commitlint.config.cjs",
            )

        config_path = self.write("commitlint.config.cjs", "module.exports = {};\n")
        expected_report = run_commitlint.CommitLintReport(
            event_name="push",
            config_path=str(config_path),
            description="push fallback",
            commits_checked=1,
            checked_commits=["head"],
        )

        with (
            mock.patch.object(
                run_commitlint,
                "build_commitlint_plan",
                return_value=run_commitlint.CommitLintPlan(
                    event_name="push",
                    description="push fallback",
                    commit_shas=["head"],
                    mode="single-commit-fallback",
                ),
            ) as build_plan,
            mock.patch.object(
                run_commitlint,
                "run_commitlint_plan",
                return_value=expected_report,
            ) as run_plan,
        ):
            report = run_commitlint.run_commitlint(
                self.repo_root,
                context,
                config_path="commitlint.config.cjs",
            )

        build_plan.assert_called_once()
        run_plan.assert_called_once()
        self.assertEqual(report, expected_report)

    def test_main_success_writes_summary_and_stdout(self) -> None:
        report = run_commitlint.CommitLintReport(
            event_name="push",
            config_path="commitlint.config.cjs",
            description="push fallback",
            commits_checked=1,
            checked_commits=["abc123"],
        )

        stdout_buffer = io.StringIO()
        stderr_buffer = io.StringIO()
        with (
            mock.patch.object(run_commitlint, "run_commitlint", return_value=report),
            contextlib.redirect_stdout(stdout_buffer),
            contextlib.redirect_stderr(stderr_buffer),
        ):
            exit_code = run_commitlint.main(
                [
                    "--event-name",
                    "push",
                    "--repo-root",
                    str(self.repo_root),
                    "--summary-path",
                    "artifacts/summary.md",
                ]
            )

        self.assertEqual(exit_code, 0)
        self.assertEqual(stderr_buffer.getvalue(), "")
        self.assertIn("Commitlint validation passed.", stdout_buffer.getvalue())
        summary = (self.repo_root / "artifacts" / "summary.md").read_text(encoding="utf-8")
        self.assertIn("- Result: success", summary)

    def test_main_failure_writes_summary_and_stderr(self) -> None:
        stdout_buffer = io.StringIO()
        stderr_buffer = io.StringIO()
        with (
            mock.patch.object(
                run_commitlint,
                "run_commitlint",
                side_effect=ValueError("unsupported event 'bad'"),
            ),
            contextlib.redirect_stdout(stdout_buffer),
            contextlib.redirect_stderr(stderr_buffer),
        ):
            exit_code = run_commitlint.main(
                [
                    "--event-name",
                    "bad",
                    "--repo-root",
                    str(self.repo_root),
                    "--summary-path",
                    "artifacts/summary.md",
                ]
            )

        self.assertEqual(exit_code, 1)
        self.assertEqual(stdout_buffer.getvalue(), "")
        self.assertIn("Commitlint validation failed.", stderr_buffer.getvalue())
        self.assertIn("unsupported event 'bad'", stderr_buffer.getvalue())

        summary = (self.repo_root / "artifacts" / "summary.md").read_text(encoding="utf-8")
        self.assertIn("- Result: failure", summary)
        self.assertIn("## Errors", summary)

    def test_main_handles_subprocess_failure(self) -> None:
        error = subprocess.CalledProcessError(returncode=1, cmd=["mocklint"])
        stdout_buffer = io.StringIO()
        stderr_buffer = io.StringIO()
        with (
            mock.patch.object(run_commitlint, "run_commitlint", side_effect=error),
            contextlib.redirect_stdout(stdout_buffer),
            contextlib.redirect_stderr(stderr_buffer),
        ):
            exit_code = run_commitlint.main(
                [
                    "--event-name",
                    "push",
                    "--repo-root",
                    str(self.repo_root),
                ]
            )

        self.assertEqual(exit_code, 1)
        self.assertIn("Commitlint validation failed.", stderr_buffer.getvalue())


if __name__ == "__main__":
    unittest.main()

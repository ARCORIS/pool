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

"""Tests for the repository-local docs smoke checker."""

from __future__ import annotations

import contextlib
import importlib.util
import io
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).resolve().parents[1] / "scripts" / "check_docs_smoke.py"
MODULE_SPEC = importlib.util.spec_from_file_location("check_docs_smoke", MODULE_PATH)
assert MODULE_SPEC is not None
assert MODULE_SPEC.loader is not None
check_docs_smoke = importlib.util.module_from_spec(MODULE_SPEC)
sys.modules[MODULE_SPEC.name] = check_docs_smoke
MODULE_SPEC.loader.exec_module(check_docs_smoke)


class DocsSmokeTestCase(unittest.TestCase):
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

    def populate_required_repo(self, *, skip: set[str] | None = None) -> None:
        skip = skip or set()
        for relative_path in check_docs_smoke.REQUIRED_PATHS:
            if relative_path in skip:
                continue

            if relative_path == "README.md":
                content = (
                    "# README\n\n"
                    "[Docs Index](docs/index.md)\n"
                    "[Security](<SECURITY.md> \"Security policy\")\n"
                    "[Self](#readme)\n"
                    '<a href="docs/index.md#docs-index">Docs HTML</a>\n'
                    "```md\n[Ignored](missing.md)\n```\n"
                )
            elif relative_path == "docs/index.md":
                content = (
                    "# Docs Index\n\n"
                    "[Architecture](./architecture.md#architecture)\n"
                )
            elif relative_path.endswith(".md"):
                title = Path(relative_path).stem.replace("-", " ").replace("_", " ").title()
                content = f"# {title}\n"
            else:
                content = "package pool\n"

            self.write(relative_path, content)


class ValidationReportTests(unittest.TestCase):
    def test_validation_report_renders_summary_and_success_lines(self) -> None:
        report = check_docs_smoke.ValidationReport(
            required_paths_checked=16,
            markdown_files_checked=15,
            markdown_targets_checked=12,
            html_targets_checked=2,
            anchor_checks=7,
            missing_required_paths=["SECURITY.md"],
            broken_targets=["README.md: target not found -> docs/missing.md"],
            missing_anchors=["README.md: missing Markdown anchor #missing in docs/index.md"],
        )

        self.assertFalse(report.ok())

        summary_text = "\n".join(report.summary_lines())
        self.assertIn("- Result: failure", summary_text)
        self.assertIn("## Missing Required Paths", summary_text)
        self.assertIn("## Broken Local Targets", summary_text)
        self.assertIn("## Missing Anchors", summary_text)

        success_report = check_docs_smoke.ValidationReport(markdown_files_checked=3)
        self.assertTrue(success_report.ok())
        self.assertEqual(
            success_report.success_lines()[0],
            "Documentation smoke check passed.",
        )


class HelperFunctionTests(DocsSmokeTestCase):
    def test_parse_args_supports_defaults_and_overrides(self) -> None:
        defaults = check_docs_smoke.parse_args([])
        self.assertEqual(defaults.repo_root, ".")
        self.assertEqual(defaults.summary_path, "")

        custom = check_docs_smoke.parse_args(
            ["--repo-root", "/tmp/repo", "--summary-path", "summary.md"]
        )
        self.assertEqual(custom.repo_root, "/tmp/repo")
        self.assertEqual(custom.summary_path, "summary.md")

    def test_collect_markdown_files_respects_scope_and_deduplicates(self) -> None:
        self.write("README.md", "# README\n")
        self.write(".generated.md", "# Hidden\n")
        self.write("docs/index.md", "# Docs Index\n")
        self.write("docs/deeper/guide.md", "# Guide\n")
        self.write("other/ignored.md", "# Ignored\n")

        with mock.patch.object(
            check_docs_smoke,
            "MARKDOWN_GLOBS",
            ("*.md", "*.md", "docs/**/*.md"),
        ):
            collected = check_docs_smoke.collect_markdown_files(self.repo_root)

        collected_paths = [path.relative_to(self.repo_root).as_posix() for path in collected]
        self.assertEqual(
            collected_paths,
            ["README.md", "docs/deeper/guide.md", "docs/index.md"],
        )

    def test_text_normalization_helpers_behave_as_expected(self) -> None:
        text = (
            "Before `inline [ignored](missing.md)` after\n"
            "```md\n[also ignored](docs/missing.md)\n```\n"
            "[kept](docs/index.md)\n"
        )
        stripped = check_docs_smoke.strip_code(text)

        self.assertNotIn("missing.md", stripped)
        self.assertIn("[kept](docs/index.md)", stripped)
        self.assertEqual(
            check_docs_smoke.normalize_heading("Lifecycle: Accepted / Rejected?"),
            "lifecycle-accepted-rejected",
        )
        self.assertEqual(
            check_docs_smoke.normalize_markdown_target(
                '<docs/file name.md> "Friendly title"'
            ),
            "docs/file name.md",
        )
        self.assertTrue(check_docs_smoke.is_external_target("https://example.com"))
        self.assertFalse(check_docs_smoke.is_external_target("docs/index.md"))

    def test_iter_target_helpers_extract_only_local_targets(self) -> None:
        text = (
            "[Local](docs/index.md)\n"
            "![Chart](bench/charts/chart.svg)\n"
            "[External](https://example.com)\n"
            '<a href="docs/index.md#docs-index">Docs</a>\n'
            '<img src="https://example.com/logo.svg">\n'
        )

        self.assertEqual(
            list(check_docs_smoke.iter_markdown_targets(text)),
            ["docs/index.md", "bench/charts/chart.svg"],
        )
        self.assertEqual(
            list(check_docs_smoke.iter_html_targets(text)),
            ["docs/index.md#docs-index"],
        )

    def test_resolve_local_target_supports_local_anchor_and_blocks_repo_escape(self) -> None:
        source_path = self.write("docs/index.md", "# Docs Index\n")
        self.write("README.md", "# README\n")

        resolved, anchor = check_docs_smoke.resolve_local_target(
            self.repo_root,
            source_path,
            "#docs-index",
        )
        self.assertEqual(resolved, source_path)
        self.assertEqual(anchor, "docs-index")

        resolved, anchor = check_docs_smoke.resolve_local_target(
            self.repo_root,
            source_path,
            "../README.md#readme",
        )
        self.assertEqual(resolved, (self.repo_root / "README.md").resolve())
        self.assertEqual(anchor, "readme")

        resolved, anchor = check_docs_smoke.resolve_local_target(
            self.repo_root,
            source_path,
            "../../../outside.md#escaped",
        )
        self.assertIsNone(resolved)
        self.assertEqual(anchor, "escaped")

    def test_validate_anchor_records_missing_anchor_only_for_markdown_targets(self) -> None:
        source_path = self.write("README.md", "# README\n")
        target_markdown = self.write("docs/index.md", "# Docs Index\n")
        target_asset = self.write("bench/charts/chart.svg", "<svg/>\n")

        anchor_cache: dict[Path, set[str]] = {}
        report = check_docs_smoke.ValidationReport()

        check_docs_smoke.validate_anchor(
            self.repo_root,
            source_path,
            target_markdown,
            "docs-index",
            anchor_cache,
            report,
            html=False,
        )
        self.assertEqual(report.anchor_checks, 1)
        self.assertEqual(report.missing_anchors, [])

        check_docs_smoke.validate_anchor(
            self.repo_root,
            source_path,
            target_markdown,
            "missing-anchor",
            anchor_cache,
            report,
            html=True,
        )
        self.assertEqual(report.anchor_checks, 2)
        self.assertEqual(len(report.missing_anchors), 1)
        self.assertIn("missing HTML anchor #missing-anchor", report.missing_anchors[0])

        check_docs_smoke.validate_anchor(
            self.repo_root,
            source_path,
            target_asset,
            "ignored-for-non-markdown",
            anchor_cache,
            report,
            html=False,
        )
        self.assertEqual(report.anchor_checks, 2)

    def test_write_summary_uses_report_summary_lines(self) -> None:
        report = check_docs_smoke.ValidationReport(
            required_paths_checked=1,
            missing_required_paths=["README.md"],
        )
        summary_path = self.repo_root / "summary.md"

        check_docs_smoke.write_summary(summary_path, report)

        summary = summary_path.read_text(encoding="utf-8")
        self.assertIn("# Docs Smoke Summary", summary)
        self.assertIn("- Result: failure", summary)
        self.assertIn("## Missing Required Paths", summary)


class DocsSmokeExecutionTests(DocsSmokeTestCase):
    def test_run_docs_smoke_successfully_validates_repository_docs(self) -> None:
        self.populate_required_repo()

        report = check_docs_smoke.run_docs_smoke(self.repo_root)

        self.assertTrue(report.ok())
        self.assertEqual(
            report.required_paths_checked,
            len(check_docs_smoke.REQUIRED_PATHS),
        )
        self.assertGreater(report.markdown_files_checked, 0)
        self.assertGreater(report.markdown_targets_checked, 0)
        self.assertGreater(report.html_targets_checked, 0)
        self.assertGreater(report.anchor_checks, 0)

    def test_run_docs_smoke_reports_missing_paths_broken_targets_and_missing_anchors(
        self,
    ) -> None:
        self.populate_required_repo(skip={"THIRD_PARTY_NOTICES.md"})
        self.write(
            "README.md",
            (
                "# README\n\n"
                "[Broken](docs/missing.md)\n"
                "[Missing Anchor](docs/index.md#missing-anchor)\n"
            ),
        )

        report = check_docs_smoke.run_docs_smoke(self.repo_root)

        self.assertFalse(report.ok())
        self.assertIn("THIRD_PARTY_NOTICES.md", report.missing_required_paths)
        self.assertTrue(
            any("target not found -> docs/missing.md" in item for item in report.broken_targets)
        )
        self.assertTrue(
            any("#missing-anchor" in item for item in report.missing_anchors)
        )

    def test_run_docs_smoke_requires_at_least_one_markdown_file(self) -> None:
        self.write("doc.go", "package pool\n")

        with self.assertRaisesRegex(FileNotFoundError, "No markdown files found"):
            check_docs_smoke.run_docs_smoke(self.repo_root)

    def test_main_success_writes_summary_and_stdout(self) -> None:
        self.populate_required_repo()
        summary_relative_path = "artifacts/docs-smoke-summary.md"

        stdout_buffer = io.StringIO()
        stderr_buffer = io.StringIO()
        with contextlib.redirect_stdout(stdout_buffer), contextlib.redirect_stderr(
            stderr_buffer
        ):
            exit_code = check_docs_smoke.main(
                [
                    "--repo-root",
                    str(self.repo_root),
                    "--summary-path",
                    summary_relative_path,
                ]
            )

        self.assertEqual(exit_code, 0)
        self.assertEqual(stderr_buffer.getvalue(), "")
        self.assertIn("Documentation smoke check passed.", stdout_buffer.getvalue())

        summary_path = self.repo_root / summary_relative_path
        self.assertTrue(summary_path.exists())
        summary = summary_path.read_text(encoding="utf-8")
        self.assertIn("- Result: success", summary)

    def test_main_failure_writes_summary_and_grouped_errors(self) -> None:
        self.populate_required_repo(skip={"SECURITY.md"})
        self.write(
            "README.md",
            (
                "# README\n\n"
                "[Broken](docs/missing.md)\n"
                "[Missing Anchor](docs/index.md#missing-anchor)\n"
            ),
        )
        summary_relative_path = "artifacts/docs-smoke-summary.md"

        stdout_buffer = io.StringIO()
        stderr_buffer = io.StringIO()
        with contextlib.redirect_stdout(stdout_buffer), contextlib.redirect_stderr(
            stderr_buffer
        ):
            exit_code = check_docs_smoke.main(
                [
                    "--repo-root",
                    str(self.repo_root),
                    "--summary-path",
                    summary_relative_path,
                ]
            )

        self.assertEqual(exit_code, 1)
        self.assertEqual(stdout_buffer.getvalue(), "")

        stderr = stderr_buffer.getvalue()
        self.assertIn("Documentation smoke check failed.", stderr)
        self.assertIn("Missing Required Paths:", stderr)
        self.assertIn("Broken Local Targets:", stderr)
        self.assertIn("Missing Anchors:", stderr)

        summary = (self.repo_root / summary_relative_path).read_text(encoding="utf-8")
        self.assertIn("- Result: failure", summary)

    def test_main_returns_error_when_no_markdown_files_exist(self) -> None:
        stdout_buffer = io.StringIO()
        stderr_buffer = io.StringIO()
        with contextlib.redirect_stdout(stdout_buffer), contextlib.redirect_stderr(
            stderr_buffer
        ):
            exit_code = check_docs_smoke.main(["--repo-root", str(self.repo_root)])

        self.assertEqual(exit_code, 1)
        self.assertEqual(stdout_buffer.getvalue(), "")
        self.assertIn("No markdown files found to validate.", stderr_buffer.getvalue())


if __name__ == "__main__":
    unittest.main()

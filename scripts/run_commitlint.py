#!/usr/bin/env python3
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
#
"""Repository-local commitlint orchestration for arcoris.dev/pool.

This script keeps commit-range selection and commitlint invocation out of
workflow YAML so the same logic can be read, reviewed, and reproduced locally.

The script is responsible for:
- selecting the correct commit range for the current GitHub event;
- falling back to linting a single commit when a usable range does not exist;
- invoking commitlint with the repository's `commitlint.config.cjs`;
- writing a compact Markdown summary for GitHub Actions.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from collections.abc import Sequence
from dataclasses import dataclass, field
from pathlib import Path


DEFAULT_COMMITLINT_COMMAND = ("tools/commitlint/node_modules/.bin/commitlint",)
SUPPORTED_EVENTS = ("pull_request", "merge_group", "push")


@dataclass(slots=True)
class CommitLintContext:
    """GitHub event context needed to determine what should be linted."""

    # GitHub event name that selected the validation mode.
    event_name: str
    # SHA of the checked-out HEAD for the workflow run.
    github_sha: str = ""
    # Previous SHA from a push event; may be empty or all-zero on first push.
    push_before: str = ""
    # Base SHA of the pull request target branch.
    pr_base_sha: str = ""
    # Head SHA of the pull request branch.
    pr_head_sha: str = ""
    # Base SHA provided by merge queue / merge_group events.
    merge_group_base_sha: str = ""


@dataclass(slots=True)
class CommitLintPlan:
    """Concrete lint plan derived from the event context."""

    # Event name that produced this plan.
    event_name: str
    # Human-readable explanation of the selected lint target.
    description: str
    # Commits that will be passed through commitlint in order.
    commit_shas: list[str]
    # Whether the plan came from a range or a single-commit fallback.
    mode: str


@dataclass(slots=True)
class CommitLintReport:
    """Aggregated output of one commitlint run."""

    # Event name validated by the current run.
    event_name: str
    # Commitlint configuration file used for validation.
    config_path: str
    # Human-readable explanation of the lint target.
    description: str = ""
    # Number of commits passed to commitlint.
    commits_checked: int = 0
    # SHAs that were actually linted.
    checked_commits: list[str] = field(default_factory=list)
    # Fatal setup or validation errors produced before commitlint success.
    errors: list[str] = field(default_factory=list)

    def ok(self) -> bool:
        """Return True when the run completed without fatal errors."""
        return not self.errors

    def summary_lines(self) -> list[str]:
        """Render a Markdown summary suitable for GitHub step summaries."""
        lines = [
            "# Commit Lint Summary",
            "",
            f"- Event: `{self.event_name}`",
            f"- Configuration: `{self.config_path}`",
            f"- Commits checked: {self.commits_checked}",
            f"- Mode: {self.description or 'not determined'}",
            "- Result: success" if self.ok() else "- Result: failure",
        ]

        if self.checked_commits:
            lines.extend(["", "## Checked Commits", ""])
            lines.extend(f"- `{sha}`" for sha in self.checked_commits)

        if self.errors:
            lines.extend(["", "## Errors", ""])
            lines.extend(f"- {error}" for error in self.errors)

        return lines

    def success_lines(self) -> list[str]:
        """Render compact CLI output for a successful run."""
        return [
            "Commitlint validation passed.",
            f"Event: {self.event_name}",
            f"Commits checked: {self.commits_checked}",
            f"Mode: {self.description}",
        ]


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    """Parse CLI arguments for local or CI execution."""
    parser = argparse.ArgumentParser(
        description="Run repository-local commitlint orchestration."
    )
    parser.add_argument("--event-name", required=True, help="GitHub event name.")
    parser.add_argument("--github-sha", default="", help="GitHub head SHA.")
    parser.add_argument("--push-before", default="", help="Previous SHA for push events.")
    parser.add_argument("--pr-base-sha", default="", help="PR base SHA.")
    parser.add_argument("--pr-head-sha", default="", help="PR head SHA.")
    parser.add_argument(
        "--merge-group-base-sha",
        default="",
        help="Merge queue base SHA.",
    )
    parser.add_argument(
        "--repo-root",
        default=".",
        help="Repository root used for git commands and config resolution.",
    )
    parser.add_argument(
        "--config-path",
        default="commitlint.config.cjs",
        help="Path to the commitlint configuration file.",
    )
    parser.add_argument(
        "--summary-path",
        default="",
        help="Optional path for a Markdown summary file.",
    )
    return parser.parse_args(argv)


def build_context(args: argparse.Namespace) -> CommitLintContext:
    """Build a typed event context from parsed CLI arguments."""
    return CommitLintContext(
        event_name=args.event_name,
        github_sha=args.github_sha,
        push_before=args.push_before,
        pr_base_sha=args.pr_base_sha,
        pr_head_sha=args.pr_head_sha,
        merge_group_base_sha=args.merge_group_base_sha,
    )


def is_zero_sha(value: str) -> bool:
    """Return True when a SHA-like value is empty or all zeros."""
    return bool(value) and set(value) == {"0"}


def resolve_summary_path(repo_root: Path, summary_path: str) -> Path:
    """Resolve a summary path relative to the repository root."""
    resolved = Path(summary_path)
    if resolved.is_absolute():
        return resolved
    return repo_root / resolved


def run_git(repo_root: Path, args: Sequence[str]) -> str:
    """Run a git command inside the repository and return stdout."""
    completed = subprocess.run(
        ["git", *args],
        cwd=repo_root,
        check=True,
        capture_output=True,
        text=True,
    )
    return completed.stdout


def get_commit_message(repo_root: Path, sha: str) -> str:
    """Read the full commit message body for one SHA."""
    return run_git(repo_root, ["log", "--format=%B", "-n", "1", sha])


def list_commits_in_range(repo_root: Path, start_sha: str, end_sha: str) -> list[str]:
    """List non-merge commits in a Git revision range in chronological order."""
    output = run_git(
        repo_root,
        ["rev-list", "--reverse", "--no-merges", f"{start_sha}..{end_sha}"],
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


def lint_commit_message(
    repo_root: Path,
    config_path: Path,
    sha: str,
    *,
    commitlint_command: Sequence[str] = DEFAULT_COMMITLINT_COMMAND,
) -> None:
    """Run commitlint against one already-resolved commit message."""
    message = get_commit_message(repo_root, sha)
    subprocess.run(
        [
            *commitlint_command,
            "--config",
            str(config_path),
            "--verbose",
        ],
        cwd=repo_root,
        check=True,
        input=message,
        text=True,
    )


def require_sha(value: str, label: str) -> str:
    """Validate that a required SHA-like argument is present."""
    if not value:
        raise ValueError(f"missing required SHA for {label}")
    return value


def build_plan_for_range(
    repo_root: Path,
    *,
    event_name: str,
    description: str,
    start_sha: str,
    end_sha: str,
) -> CommitLintPlan:
    """Build a range-based plan and fall back to the end SHA when needed."""
    commits = list_commits_in_range(repo_root, start_sha, end_sha)
    if not commits:
        return CommitLintPlan(
            event_name=event_name,
            description=f"{description}; no non-merge commits found, linting head only",
            commit_shas=[end_sha],
            mode="single-commit-fallback",
        )

    return CommitLintPlan(
        event_name=event_name,
        description=description,
        commit_shas=commits,
        mode="range",
    )


def build_commitlint_plan(repo_root: Path, context: CommitLintContext) -> CommitLintPlan:
    """Select the correct commitlint target for the current event context."""
    event_name = context.event_name

    if event_name == "pull_request":
        start_sha = require_sha(context.pr_base_sha, "pull_request.base.sha")
        end_sha = require_sha(context.pr_head_sha, "pull_request.head.sha")
        return build_plan_for_range(
            repo_root,
            event_name=event_name,
            description=f"pull request range {start_sha}..{end_sha}",
            start_sha=start_sha,
            end_sha=end_sha,
        )

    if event_name == "merge_group":
        start_sha = require_sha(
            context.merge_group_base_sha,
            "merge_group.base_sha",
        )
        end_sha = require_sha(context.github_sha, "github.sha")
        return build_plan_for_range(
            repo_root,
            event_name=event_name,
            description=f"merge group range {start_sha}..{end_sha}",
            start_sha=start_sha,
            end_sha=end_sha,
        )

    if event_name == "push":
        end_sha = require_sha(context.github_sha, "github.sha")
        if context.push_before and not is_zero_sha(context.push_before):
            return build_plan_for_range(
                repo_root,
                event_name=event_name,
                description=f"push range {context.push_before}..{end_sha}",
                start_sha=context.push_before,
                end_sha=end_sha,
            )

        return CommitLintPlan(
            event_name=event_name,
            description="push fallback with no usable previous SHA; linting head only",
            commit_shas=[end_sha],
            mode="single-commit-fallback",
        )

    raise ValueError(
        f"unsupported event '{event_name}'; expected one of {', '.join(SUPPORTED_EVENTS)}"
    )


def run_commitlint_plan(
    repo_root: Path,
    config_path: Path,
    plan: CommitLintPlan,
    *,
    commitlint_command: Sequence[str] = DEFAULT_COMMITLINT_COMMAND,
) -> CommitLintReport:
    """Execute a commitlint plan and return the aggregated report."""
    report = CommitLintReport(
        event_name=plan.event_name,
        config_path=str(config_path),
        description=plan.description,
    )

    for sha in plan.commit_shas:
        lint_commit_message(
            repo_root,
            config_path,
            sha,
            commitlint_command=commitlint_command,
        )
        report.checked_commits.append(sha)

    report.commits_checked = len(report.checked_commits)
    return report


def write_summary(summary_path: Path, report: CommitLintReport) -> None:
    """Write a Markdown summary for GitHub step summaries."""
    summary_path.parent.mkdir(parents=True, exist_ok=True)
    summary_path.write_text("\n".join(report.summary_lines()) + "\n", encoding="utf-8")


def print_failures(report: CommitLintReport) -> None:
    """Emit grouped failure output that remains readable in CI logs."""
    print("Commitlint validation failed.", file=sys.stderr)
    for error in report.errors:
        print(f"- {error}", file=sys.stderr)


def print_success(report: CommitLintReport) -> None:
    """Emit compact success output for local or CI logs."""
    for line in report.success_lines():
        print(line)


def run_commitlint(
    repo_root: Path,
    context: CommitLintContext,
    *,
    config_path: str,
    commitlint_command: Sequence[str] = DEFAULT_COMMITLINT_COMMAND,
) -> CommitLintReport:
    """Run repository-local commitlint orchestration and return the report."""
    resolved_config_path = Path(config_path)
    if not resolved_config_path.is_absolute():
        resolved_config_path = repo_root / resolved_config_path
    if not resolved_config_path.exists():
        raise FileNotFoundError(
            f"commitlint configuration not found: {resolved_config_path.relative_to(repo_root)}"
        )

    plan = build_commitlint_plan(repo_root, context)
    return run_commitlint_plan(
        repo_root,
        resolved_config_path,
        plan,
        commitlint_command=commitlint_command,
    )


def main(argv: Sequence[str] | None = None) -> int:
    """Run commitlint orchestration and return a process exit code."""
    args = parse_args(argv)
    repo_root = Path(args.repo_root).resolve()
    context = build_context(args)

    try:
        report = run_commitlint(
            repo_root,
            context,
            config_path=args.config_path,
        )
    except (FileNotFoundError, ValueError, subprocess.CalledProcessError) as exc:
        report = CommitLintReport(
            event_name=context.event_name,
            config_path=args.config_path,
            errors=[str(exc)],
        )
        if args.summary_path:
            write_summary(resolve_summary_path(repo_root, args.summary_path), report)
        print_failures(report)
        return 1

    if args.summary_path:
        write_summary(resolve_summary_path(repo_root, args.summary_path), report)

    print_success(report)
    return 0


if __name__ == "__main__":
    sys.exit(main())

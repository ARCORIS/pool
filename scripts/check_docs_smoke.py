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
"""Repository-specific docs smoke checks for arcoris.dev/pool.

This script is intentionally repository-local instead of inline workflow code so
the exact same checks can run in CI and from a contributor's shell.

The smoke check is intentionally narrow and repository-shaped:
- verify the maintained documentation entrypoints that should always exist;
- validate local Markdown links and local HTML href/src targets;
- validate Markdown heading anchors for intra-repository links;
- produce a small summary file that GitHub Actions can publish directly.
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Iterator, Sequence
from dataclasses import dataclass, field
from pathlib import Path


# ---------------------------------------------------------------------------
# Repository policy inputs
# ---------------------------------------------------------------------------
#
# Keep the docs surface explicit. This prevents "helpful" broad scans from
# silently expanding into tool caches, hidden directories, or generated content
# that are not part of the maintained contributor-facing documentation flow.
REQUIRED_PATHS = (
    "README.md",
    "CONTRIBUTING.md",
    "SECURITY.md",
    "CODE_OF_CONDUCT.md",
    "THIRD_PARTY_NOTICES.md",
    "doc.go",
    "docs/index.md",
    "docs/architecture.md",
    "docs/lifecycle.md",
    "docs/non-goals.md",
    "docs/performance/README.md",
    "docs/performance/methodology.md",
    "docs/performance/benchmark-matrix.md",
    "docs/performance/interpretation-guide.md",
    "docs/performance/reports/README.md",
    "docs/performance/reports/2026-04-21-initial-baseline.md",
)

# Repository-facing documentation surfaces that should participate in smoke
# validation. Keep this scope narrow and explicit instead of scanning every
# Markdown-like file under hidden or tool-specific directories.
MARKDOWN_GLOBS = ("*.md", "docs/**/*.md")


# ---------------------------------------------------------------------------
# Lightweight parsing expressions
# ---------------------------------------------------------------------------
#
# The checker intentionally uses small regexes instead of a full Markdown
# parser. The repository docs are controlled content, and the goal here is a
# fast smoke check with understandable failure modes rather than full CommonMark
# compliance.
FENCED_CODE_RE = re.compile(r"```.*?```", re.DOTALL)
INLINE_CODE_RE = re.compile(r"`[^`]*`")
MARKDOWN_LINK_RE = re.compile(r"!?\[([^\]]*)\]\(([^)\n]+)\)")
HTML_TARGET_RE = re.compile(r"""(?P<attr>href|src)=["'](?P<target>[^"']+)["']""")
HEADING_RE = re.compile(r"^(#{1,6})\s+(.*)$", re.MULTILINE)


@dataclass(slots=True)
class ValidationReport:
    """Aggregated state for one docs smoke run.

    Keeping counters and problem buckets together makes the summary and CLI
    failure output deterministic and easier to extend.
    """

    # Number of required repository doc entrypoints whose existence was checked.
    required_paths_checked: int = 0
    # Number of Markdown files scanned for local navigation targets.
    markdown_files_checked: int = 0
    # Number of Markdown links or images considered repository-local.
    markdown_targets_checked: int = 0
    # Number of local HTML href/src targets considered repository-local.
    html_targets_checked: int = 0
    # Number of anchor lookups attempted against Markdown heading sets.
    anchor_checks: int = 0
    # Required documentation paths that were missing from the checked tree.
    missing_required_paths: list[str] = field(default_factory=list)
    # Broken repository-local Markdown or HTML targets that did not resolve.
    broken_targets: list[str] = field(default_factory=list)
    # Missing Markdown anchors in resolved Markdown targets.
    missing_anchors: list[str] = field(default_factory=list)

    def ok(self) -> bool:
        """Return True when the smoke check found no actionable failures."""
        return not (
            self.missing_required_paths
            or self.broken_targets
            or self.missing_anchors
        )

    def failure_sections(self) -> tuple[tuple[str, list[str]], ...]:
        """Return grouped failure buckets in display order."""
        return (
            ("Missing Required Paths", self.missing_required_paths),
            ("Broken Local Targets", self.broken_targets),
            ("Missing Anchors", self.missing_anchors),
        )

    def summary_lines(self) -> list[str]:
        """Render a Markdown summary suitable for GitHub step summaries."""
        lines = [
            "# Docs Smoke Summary",
            "",
            f"- Required paths checked: {self.required_paths_checked}",
            f"- Markdown files checked: {self.markdown_files_checked}",
            f"- Markdown link targets checked: {self.markdown_targets_checked}",
            f"- HTML href/src targets checked: {self.html_targets_checked}",
            f"- Anchor references checked: {self.anchor_checks}",
            "- Result: success" if self.ok() else "- Result: failure",
        ]

        for title, items in self.failure_sections():
            if not items:
                continue
            lines.extend(["", f"## {title}", ""])
            lines.extend(f"- {item}" for item in items)

        return lines

    def success_lines(self) -> list[str]:
        """Render compact CLI output for a successful smoke run."""
        return [
            "Documentation smoke check passed.",
            f"Required paths checked: {self.required_paths_checked}",
            f"Markdown files checked: {self.markdown_files_checked}",
            f"Markdown link targets checked: {self.markdown_targets_checked}",
            f"HTML href/src targets checked: {self.html_targets_checked}",
            f"Anchor references checked: {self.anchor_checks}",
        ]


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    """Parse CLI arguments for local and CI execution."""
    parser = argparse.ArgumentParser(
        description="Run repository-specific documentation smoke checks."
    )
    parser.add_argument(
        "--repo-root",
        default=".",
        help="Path to the repository root. Defaults to the current directory.",
    )
    parser.add_argument(
        "--summary-path",
        default="",
        help="Optional path for a Markdown summary report.",
    )
    return parser.parse_args(argv)


def collect_markdown_files(repo_root: Path) -> list[Path]:
    """Collect the maintained Markdown files that belong to docs smoke.

    The glob list is intentionally explicit. We do not recurse through the whole
    repository because that would make the check sensitive to unrelated tooling
    or generated Markdown fragments.

    Hidden files are excluded as well so local scratch files such as
    `.docs-smoke-summary.md` do not pollute the next smoke run.
    """

    files: list[Path] = []
    for pattern in MARKDOWN_GLOBS:
        files.extend(sorted(repo_root.glob(pattern)))
    unique_files = list(dict.fromkeys(files))
    return [
        path
        for path in unique_files
        if path.is_file() and not path.name.startswith(".")
    ]


def strip_code(text: str) -> str:
    """Remove fenced and inline code before link scanning.

    Code samples often contain pseudo-paths or example Markdown that should not
    be treated as live repository links.
    """

    text = FENCED_CODE_RE.sub("", text)
    text = INLINE_CODE_RE.sub("", text)
    return text


def normalize_heading(text: str) -> str:
    """Normalize a Markdown heading into the anchor shape GitHub commonly uses."""
    text = text.strip().lower()
    text = re.sub(r"[!\"#$%&'()*+,./:;<=>?@\[\\\]^`{|}~]", "", text)
    text = re.sub(r"\s+", "-", text)
    text = re.sub(r"-+", "-", text)
    return text.strip("-")


def normalize_markdown_target(raw_target: str) -> str:
    """Normalize a raw Markdown link target before path resolution.

    This handles two practical cases that show up in real repository docs:
    - `<path with spaces.md>` angle-bracket targets;
    - optional title text in links such as `(path.md "title")`.
    """

    target = raw_target.strip()
    if target.startswith("<") and ">" in target:
        return target[1 : target.index(">")].strip()
    for separator in (' "', " '", " ("):
        index = target.find(separator)
        if index != -1:
            return target[:index].strip()
    return target


def is_external_target(target: str) -> bool:
    """Return True for targets that should not be resolved inside the repo."""
    lowered = target.lower()
    return lowered.startswith(
        ("http://", "https://", "mailto:", "tel:", "data:")
    )


def collect_anchors(markdown_path: Path) -> set[str]:
    """Extract normalized heading anchors from one Markdown file."""
    raw = markdown_path.read_text(encoding="utf-8")
    anchors: set[str] = set()
    for _, heading in HEADING_RE.findall(raw):
        anchor = normalize_heading(heading)
        if anchor:
            anchors.add(anchor)
    return anchors


def validate_required_paths(repo_root: Path, report: ValidationReport) -> None:
    """Ensure the repository's maintained docs entrypoints still exist."""
    for relative_path in REQUIRED_PATHS:
        report.required_paths_checked += 1
        if not (repo_root / relative_path).exists():
            report.missing_required_paths.append(relative_path)


def split_target(target: str) -> tuple[str, str]:
    """Split a local target into its path and anchor components."""
    path_part, _, anchor = target.partition("#")
    return path_part, anchor


def resolve_local_target(
    repo_root: Path,
    source_path: Path,
    target: str,
) -> tuple[Path | None, str]:
    """Resolve a local docs target relative to the source document.

    Returning ``None`` for the path means the target escaped the repository root
    after resolution and should be treated as invalid for this smoke check.
    """

    if target.startswith("#"):
        return source_path, target[1:]

    path_part, anchor = split_target(target)
    resolved = (source_path.parent / path_part).resolve()

    try:
        # Refuse links that resolve outside the checked repository tree. This
        # keeps the smoke check focused on repository-local navigation instead
        # of accidentally blessing parent-directory hops.
        resolved.relative_to(repo_root)
    except ValueError:
        return None, anchor

    return resolved, anchor


def record_broken_target(
    repo_root: Path,
    source_path: Path,
    target: str,
    report: ValidationReport,
    *,
    html: bool,
) -> None:
    """Record a broken local target in a consistent human-readable format."""
    prefix = "HTML " if html else ""
    report.broken_targets.append(
        f"{source_path.relative_to(repo_root)}: {prefix}target not found -> {target}"
    )


def validate_anchor(
    repo_root: Path,
    source_path: Path,
    target_path: Path,
    anchor: str,
    anchor_cache: dict[Path, set[str]],
    report: ValidationReport,
    *,
    html: bool,
) -> None:
    """Validate a Markdown anchor reference when the target file is Markdown."""
    if not anchor:
        return
    if target_path.suffix.lower() != ".md":
        return

    report.anchor_checks += 1

    if target_path not in anchor_cache:
        # Cache anchors per file because heavily cross-linked docs can reference
        # the same document many times during one run.
        anchor_cache[target_path] = collect_anchors(target_path)

    if anchor not in anchor_cache[target_path]:
        prefix = "HTML" if html else "Markdown"
        report.missing_anchors.append(
            (
                f"{source_path.relative_to(repo_root)}: missing {prefix} anchor "
                f"#{anchor} in {target_path.relative_to(repo_root)}"
            )
        )


def iter_markdown_targets(text: str) -> Iterator[str]:
    """Yield repository-local Markdown link targets from stripped Markdown."""
    for _, raw_target in MARKDOWN_LINK_RE.findall(text):
        target = normalize_markdown_target(raw_target)
        if not target or is_external_target(target):
            continue
        yield target


def iter_html_targets(text: str) -> Iterator[str]:
    """Yield repository-local HTML href/src targets from stripped Markdown."""
    for match in HTML_TARGET_RE.finditer(text):
        target = match.group("target").strip()
        if not target or is_external_target(target):
            continue
        yield target


def validate_local_target_reference(
    repo_root: Path,
    source_path: Path,
    target: str,
    report: ValidationReport,
    anchor_cache: dict[Path, set[str]],
    *,
    html: bool,
) -> None:
    """Validate one repository-local Markdown or HTML target."""
    resolved_path, anchor = resolve_local_target(repo_root, source_path, target)
    if resolved_path is None or not resolved_path.exists():
        record_broken_target(
            repo_root,
            source_path,
            target,
            report,
            html=html,
        )
        return

    validate_anchor(
        repo_root,
        source_path,
        resolved_path,
        anchor,
        anchor_cache,
        report,
        html=html,
    )


def validate_markdown_links(
    repo_root: Path,
    markdown_files: list[Path],
    report: ValidationReport,
) -> None:
    """Validate repository-local Markdown and HTML link targets.

    The docs tree uses both normal Markdown links and a small amount of raw
    HTML attributes in richer landing-page sections, so both forms are checked
    in one pass over the stripped document text.
    """

    anchor_cache: dict[Path, set[str]] = {}

    for markdown_path in markdown_files:
        report.markdown_files_checked += 1
        raw = markdown_path.read_text(encoding="utf-8")
        # Ignore links that appear inside code examples. Those examples are part
        # of the docs content but are not themselves navigation.
        text = strip_code(raw)

        for target in iter_markdown_targets(text):
            report.markdown_targets_checked += 1
            validate_local_target_reference(
                repo_root,
                markdown_path,
                target,
                report,
                anchor_cache,
                html=False,
            )

        for target in iter_html_targets(text):
            # HTML href/src support is mainly here for badge-heavy landing pages
            # and any future rich README sections that still need local targets
            # to stay valid.
            report.html_targets_checked += 1
            validate_local_target_reference(
                repo_root,
                markdown_path,
                target,
                report,
                anchor_cache,
                html=True,
            )


def run_docs_smoke(repo_root: Path) -> ValidationReport:
    """Run repository-local docs validation and return the aggregated report."""
    markdown_files = collect_markdown_files(repo_root)
    if not markdown_files:
        raise FileNotFoundError("No markdown files found to validate.")

    report = ValidationReport()
    validate_required_paths(repo_root, report)
    validate_markdown_links(repo_root, markdown_files, report)
    return report


def resolve_summary_path(repo_root: Path, summary_path: str) -> Path:
    """Resolve the optional summary path relative to the repository root."""
    resolved = Path(summary_path)
    if resolved.is_absolute():
        return resolved
    return repo_root / resolved


def write_summary(summary_path: Path, report: ValidationReport) -> None:
    """Write a small Markdown summary for GitHub step summaries."""
    summary_path.parent.mkdir(parents=True, exist_ok=True)
    summary_path.write_text(
        "\n".join(report.summary_lines()) + "\n",
        encoding="utf-8",
    )


def print_failures(report: ValidationReport) -> None:
    """Emit grouped failure output that is readable in CI logs."""
    print("Documentation smoke check failed.", file=sys.stderr)
    for title, items in report.failure_sections():
        if not items:
            continue
        print(f"\n{title}:", file=sys.stderr)
        for item in items:
            print(f"- {item}", file=sys.stderr)


def print_success(report: ValidationReport) -> None:
    """Emit a compact success summary for local or CI logs."""
    for line in report.success_lines():
        print(line)


def main(argv: Sequence[str] | None = None) -> int:
    """Run the docs smoke check and return a process exit code."""
    args = parse_args(argv)
    repo_root = Path(args.repo_root).resolve()

    try:
        report = run_docs_smoke(repo_root)
    except FileNotFoundError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    if args.summary_path:
        write_summary(resolve_summary_path(repo_root, args.summary_path), report)

    if not report.ok():
        print_failures(report)
        return 1

    print_success(report)
    return 0


if __name__ == "__main__":
    sys.exit(main())

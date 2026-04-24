<div align="center">

# Contributing to `arcoris.dev/pool`

**Contributor workflow for a small Go library with a protected `main` branch and short-lived topic branches.**

[![Docs Index](https://img.shields.io/badge/Docs-Index-0F766E?style=flat)](docs/index.md)
[![Package Contract](https://img.shields.io/badge/Contract-doc.go-1D4ED8?style=flat)](doc.go)
[![Lifecycle](https://img.shields.io/badge/Guide-Lifecycle-B45309?style=flat)](docs/lifecycle.md)
[![PR Template](https://img.shields.io/badge/Review-PR%20Template-0F172A?style=flat)](.github/PULL_REQUEST_TEMPLATE.md)

[Overview](#overview) · [Branch Model](#branch-model) · [Branch Naming Convention](#branch-naming-convention) · [Commit Message Convention](#commit-message-convention) · [Pull Request Flow](#pull-request-flow) · [Local Validation](#local-validation) · [GitHub Actions Policy](#github-actions-policy)

</div>

`arcoris.dev/pool` is intentionally small. The repository centers on a narrow
typed pool runtime, explicit lifecycle callbacks, internal `sync.Pool`-backed
storage, documentation that acts as maintained surface area, and benchmark
tooling that is treated as engineering evidence rather than decoration.

That shape does not need a permanent integration branch. Normal work should
stay on short-lived topic branches, merge through pull requests, and release
from protected `main`.

## Overview

- `main` is the only long-lived branch.
- `main` is the default branch and the protected branch.
- All normal pull requests target `main`.
- Direct pushes to `main` are forbidden by repository policy.
- Stable releases come from SemVer tags created from commits reachable from
  `main`.
- Normal flow is:

```text
topic branch -> pull request into main -> merge -> optional stable tag from main
```

Read these first when the change touches package behavior:

- [README](README.md)
- [Package contract](doc.go)
- [Lifecycle guide](docs/lifecycle.md)
- [Architecture guide](docs/architecture.md)
- [Non-goals guide](docs/non-goals.md)

Read these first when the change touches performance workflow:

- [Performance overview](docs/performance/README.md)
- [Benchmark methodology](docs/performance/methodology.md)
- [Benchmark matrix](docs/performance/benchmark-matrix.md)
- [Interpretation guide](docs/performance/interpretation-guide.md)

## Branch model

### `main`

- The only long-lived branch.
- The default branch.
- The protected branch.
- The target for all normal pull requests.
- The source of stable SemVer release tags.

Direct pushes to `main` are not allowed by policy, even if GitHub settings
temporarily permit them. Repository rulesets should enforce the same policy.

### Topic branches

- Created from `main`.
- Used for all normal work.
- Deleted after merge.

### No permanent `next`

This repository does not use a permanent `next` branch. For a small library, a
separate integration branch adds process overhead without improving the normal
review and release flow.

If a remote `next` branch still exists from an older workflow model, treat it
as obsolete. Maintainers may delete it manually after confirming that it
contains no unique changes.

If release stabilization is ever needed later, maintainers may create a
temporary branch such as `release/v0.2.0`. That is an exception, not the normal
workflow.

## Creating a working branch

Create normal work branches from `main`:

```bash
git fetch origin
git switch main
git pull --ff-only origin main
git switch -c ci/simplify-branch-policy
git push -u origin ci/simplify-branch-policy
```

Then open a pull request into `main`.

Do not create normal work branches from an obsolete `next` branch. Do not
create them from an old release tag. Use current `main` unless maintainers
explicitly approved a different temporary base for exceptional release
stabilization work.

## Branch naming convention

Use short, lower-case, hyphen-separated branch names.

Preferred format:

```text
<type>/<short-description>
```

Allowed prefixes:

- `feat/` for new API-level or user-visible functionality
- `fix/` for bug fixes
- `refactor/` for internal restructuring without behavior changes
- `perf/` for performance improvements
- `docs/` for documentation-only changes
- `test/` for tests and test infrastructure
- `build/` for build system, module, or toolchain changes
- `ci/` for GitHub Actions and automation changes
- `chore/` for maintenance that does not fit another category
- `revert/` for reverting previous changes

Examples:

- `feat/typed-pool-options`
- `fix/pool-put-nil-value`
- `refactor/backend-lifecycle`
- `perf/syncpool-allocation-path`
- `docs/performance-methodology`
- `test/pool-concurrency-cases`
- `build/go-toolchain-policy`
- `ci/simplify-branch-policy`
- `chore/dependabot-main-target`
- `revert/remove-invalid-release-change`

Avoid vague names:

- `fix`
- `update`
- `changes`
- `work`
- `wip`
- `solve`

Do not use `solve/` as a branch prefix. It is too vague and does not describe
the change type.

## Commit message convention

Use Conventional Commits:

```text
type(scope): summary
```

Allowed types:

- `feat`
- `fix`
- `refactor`
- `perf`
- `docs`
- `test`
- `build`
- `ci`
- `chore`
- `revert`

Rules:

- use lower-case type;
- use a meaningful scope when possible;
- keep the summary concise;
- prefer imperative mood;
- do not end the summary with a period.

Examples:

- `fix(pool): reject nil values on Put`
- `ci(github): simplify workflow branch targets`
- `docs(contributing): document protected main workflow`
- `test(pool): cover Put and Get lifecycle cases`

The same header shape is expected for pull request titles because squash merges
commonly use the PR title as the final commit message.

## Pull request flow

Normal flow:

```text
topic branch -> PR into main -> merge -> optional release tag from main
```

Pull requests must:

- target `main`;
- use a Conventional Commit-style title;
- describe the motivation;
- describe the change;
- list local checks run;
- mention documentation impact;
- mention release impact if relevant;
- link related issues if relevant.

Do not open normal pull requests into `next`. Do not document or use a
`next -> main` promotion step for ordinary work.

Use the repository PR template:

- [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md)

## Local validation

Use the raw Go commands that match this repository:

```bash
go test ./...
go test -race ./...
go vet ./...
```

For benchmark smoke validation, use the lightweight repository command only
when the change affects benchmark code or benchmark tooling:

```bash
go test -run '^$' -bench . -benchtime=100ms ./...
```

For documentation changes, also run the repository docs smoke script:

```bash
python3 scripts/check_docs_smoke.py --repo-root .
```

For benchmark or benchmark-script changes, also run the affected suite or
script locally. Typical repository commands include:

```bash
go test ./internal/backend -run '^$' -bench '^BenchmarkSyncPool_' -benchmem -count 1
go test . -run '^$' -bench '^BenchmarkPaths_' -benchmem -count 1
bench/scripts/run_benchmarks.sh --help
bench/scripts/compare_benchmarks.sh --help
bench/scripts/profile_benchmarks.sh --help
python3 bench/scripts/plot_benchmarks.py --help
```

If the change makes a performance claim, keep the evidence aligned with the
repository performance docs:

- [Performance overview](docs/performance/README.md)
- [Benchmark methodology](docs/performance/methodology.md)

## GitHub Actions policy

The repository separates fast quality gates, PR-only dependency checks,
security scans, posture scans, and release-only workflows.

### All branch pushes

Expected fast checks:

- `CI`
- `Lint`
- `Docs Smoke`
- `Benchmark Smoke` when it remains lightweight

### Pull requests into `main`

Expected pull-request checks:

- `CI`
- `Lint`
- `Docs Smoke`
- `Benchmark Smoke`
- `Commit Lint`
- `Dependency Review`
- `Govulncheck`
- `CodeQL`

### Pushes to `main`

Protected branch pushes are also expected to run:

- `CI`
- `Lint`
- `Docs Smoke`
- `Benchmark Smoke`
- `Govulncheck`
- `CodeQL`

### Tags

Stable SemVer tags trigger:

- `Release`
- `Attest`

### Schedule or manual dispatch

Used for:

- `Govulncheck`
- `CodeQL`
- `Scorecards`
- manual reruns of release-only or smoke workflows when appropriate

`Scorecards` is repository-posture tooling, not a normal required pull-request
check.

## Required branch protection / rulesets

These settings cannot be fully enforced from files alone. Maintainers should
configure them in GitHub.

### For `main`

- require pull requests before merging;
- require required status checks;
- require branches to be up to date before merging if that is part of
  repository policy;
- require conversation resolution if that is part of repository policy;
- disallow bypass where possible;
- disallow force pushes;
- disallow deletions;
- optionally require linear history.

### Recommended required checks

For `main`:

- `test (ubuntu-latest)`
- `test (macos-latest)`
- `test (windows-latest)`
- `race-and-vet`
- `ci-summary`
- `golangci-lint`
- `docs-smoke`
- `benchmark-smoke`
- `commitlint`

`dependency-review`, `govulncheck`, and `CodeQL` may also be required if
maintainers want stricter security gating on `main`.

Maintainers should also keep the GitHub default branch set to `main`.

## Release policy

- Stable releases are created from SemVer tags on `main`.
- Release tags use `vX.Y.Z`.
- Examples:
  - `v0.1.1` for bug fixes;
  - `v0.2.0` for compatible new functionality;
  - `v1.0.0` for the first stable API release.
- Stable release workflows must not publish from feature branches.
- Stable release workflows must not publish from an obsolete `next` branch.
- Stable tags must point to commits reachable from `main`.
- This repository does not use a permanent prerelease branch or a permanent
  `next` prerelease flow.

If temporary release stabilization is needed later, maintainers may use a
branch such as `release/v0.2.0`. That is exceptional and should not replace
the normal `topic branch -> PR into main -> tag from main` flow.

## Dependabot policy

- Dependabot version update pull requests should target `main`.
- Dependency update pull requests must pass the same checks as ordinary changes.
- Dependency update pull requests must not bypass protected-branch policy.
- Security update routing can still depend on GitHub repository settings
  outside this repository.

## Documentation and benchmark expectations

Documentation is part of repository quality. If a change affects lifecycle,
ownership, benchmark workflow, or repository navigation, update the relevant
docs in the same change.

Start with:

- [README](README.md)
- [Docs index](docs/index.md)
- [Lifecycle guide](docs/lifecycle.md)
- [Architecture guide](docs/architecture.md)
- [Performance overview](docs/performance/README.md)

Performance claims should be backed by repository evidence, not only by a
single local number. Use the maintained benchmark workflow and keep raw
artifacts, comparison outputs, and reports aligned with the documentation.

## Security reporting

Do not open a public issue first for a potentially security-sensitive problem.
Use the repository security reporting path described in:

- [SECURITY.md](SECURITY.md)

For this repository, security-relevant issues can include:

- stale data retention across pooled reuse;
- ownership confusion after `Put`;
- concurrency misuse caused by incorrect runtime behavior or guidance;
- documentation that could lead users to unsafe assumptions.

## Summary

A strong contribution to `arcoris.dev/pool` is:

- based on current `main`;
- small and typed clearly;
- validated locally before review;
- opened as a pull request into the protected branch;
- documented when behavior or workflow changes;
- and released only from SemVer tags on `main`.

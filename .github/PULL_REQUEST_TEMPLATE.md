<!--
Copyright 2026 The ARCORIS Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# Before you submit

Use this template to explain the change clearly to maintainers and reviewers.

Guidelines:
- Replace the guidance text with actual content.
- If a section does not apply, write `None`.
- Be concrete: include commands, file paths, benchmark artifacts, and exact behavior changes.
- If the PR changes runtime semantics, lifecycle ordering, ownership expectations, benchmark methodology, or documentation, describe that explicitly.
- If the PR makes a performance claim, attach the relevant benchmark evidence.

# Summary

What changed in this PR?

Keep it short and concrete.
Preferred size: 3-7 bullets or 1-3 short paragraphs.

Good examples:
- Clarified `Put` callback ordering in package docs and lifecycle tests.
- Added shape-specific benchmark coverage for large value types.
- Refined snapshot chart generation to group by benchmark family and metric.

Avoid:
- Improved the package.
- Fixed various things.

- 
- 
- 

## Linked work

What should reviewers look at together with this PR?

- Issue(s):
- Design / proposal:
- Report / benchmark reference:
- Auto-close directive (`Closes #123`, `Fixes #123`, etc.):

## Change classification

Select all items that apply.

- [ ] Bug fix
- [ ] Feature
- [ ] Refactoring with no intended behavior change
- [ ] Performance or allocation improvement
- [ ] Benchmark / chart / report tooling change
- [ ] Documentation change
- [ ] Build / CI / release tooling change
- [ ] Security / ownership / isolation clarification
- [ ] Breaking change
- [ ] Mechanical change only

## Affected areas

Select all repository areas materially affected by this PR.

- [ ] Public runtime (`Pool[T]`, `Get`, `Put`)
- [ ] Lifecycle policy (`Options[T]`, `Reset`, `Reuse`, `OnDrop`)
- [ ] Backend (`internal/backend`)
- [ ] Ownership / concurrency contract
- [ ] Public package docs / Go doc
- [ ] Unit tests
- [ ] Benchmark source files
- [ ] Benchmark scripts (`bench/scripts`)
- [ ] Charts / reports / performance docs
- [ ] Root README or repository docs
- [ ] CI / release / repository automation

## Why this change

What problem existed before this PR?
Why is this change needed now?

- Problem statement:
- Why now:
- Who benefits (caller, maintainer, contributor, benchmark/report workflow, etc.):

## Reviewer guidance

Tell reviewers where to focus.

Examples:
- Review focus: `Put` ordering and ownership wording.
- Key invariants: admission happens before reset; rejected values are not stored.
- Files worth focused review: `lifecycle.go`, `lifecycle_test.go`, `docs/lifecycle.md`.

- Review focus:
- Key invariants or contracts:
- Files / flows worth focused review:
- Explicitly out of scope:

## Behavioral and contract impact

Describe what changes in package behavior, documentation, or repository workflow.

If a section does not apply, write `None`.

- Public API impact:
- Lifecycle semantic impact:
- Ownership or concurrency impact:
- Backend or storage impact:
- Benchmark or report workflow impact:
- Documentation or example impact:
- Failure mode / misuse sensitivity:

## Validation

Prefer copy-pasteable commands and precise evidence.

### Validation steps

1.
2.
3.

### Validation evidence

- Commands / suites:
- Environment:
- Evidence summary:
- Remaining validation gaps:

### Tests executed

Select what actually ran for this PR.

- [ ] Unit tests
- [ ] Race detector
- [ ] Backend tests
- [ ] Benchmark compilation / smoke validation
- [ ] Benchmarks / benchmark scripts
- [ ] Chart generation validation
- [ ] Documentation validation
- [ ] Lint / static analysis
- [ ] Manual validation
- [ ] Not run, explained above

## Performance evidence

Complete this section if the PR makes any performance, allocation, or chart/report claim.
If not applicable, write `None`.

- Relevant benchmark family:
- Raw artifact(s):
- Compare artifact(s):
- Chart or report reference:
- Claim being made:
- Limits of that evidence:

## Compatibility and release impact

Be explicit even when there is no impact.

- Breaking change: [ ] No  [ ] Yes
- Migration required: [ ] No  [ ] Yes
- Public API changed: [ ] No  [ ] Yes
- Lifecycle / ownership semantics changed: [ ] No  [ ] Yes
- Benchmark methodology or chart/report expectations changed: [ ] No  [ ] Yes
- Default behavior changed: [ ] No  [ ] Yes

### Migration or upgrade notes

What should maintainers or users do when adopting this change?

- 

## Security and data-safety considerations

For this package, security-relevant changes may include:
- object reuse causing stale data retention;
- ownership confusion after `Put`;
- concurrency misuse surfaces;
- documentation that could cause unsafe caller assumptions.

If not applicable, write `None`.

- Security / ownership / data-retention impact:
- New trust assumptions or misuse risks:
- Third-party material copied or adapted:
- License / attribution follow-up:

## Known limitations and follow-up

- Known limitations:
- Follow-up work:

## Author checklist

Select all items that are true for this PR.

- [ ] I reviewed the diff myself before requesting review.
- [ ] I removed accidental secrets, tokens, personal data, debug-only artifacts, and temporary benchmark outputs.
- [ ] I added or updated tests where needed, or explained why they were not run.
- [ ] I updated documentation, examples, or comments where needed.
- [ ] I described lifecycle, ownership, compatibility, and behavioral impact where relevant.
- [ ] I attached benchmark/report evidence for any performance claim.
- [ ] I reviewed benchmark, chart, or report implications where relevant.
- [ ] CI is green, or failing / non-required jobs are explained.

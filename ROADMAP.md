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

# Roadmap

## Current focus

The current focus is to stabilize the lifecycle contract of `arcoris.dev/pool`, keep the public API small, and make repository automation predictable enough for conservative reuse in production-facing Go code.

## Near-term work

- Preserve the module path `arcoris.dev/pool`.
- Improve fuzz and property-style coverage for lifecycle and reuse invariants.
- Keep release provenance and repository automation explicit and reviewable.
- Maintain a coherent benchmark taxonomy across backend, baseline, paths, shapes, parallel, and metrics suites.

## Stabilization path

The package should continue evolving conservatively until API behavior and lifecycle semantics are stable enough for a future `v1`. Stability matters more than adding surface area quickly.

## Security and supply-chain maturity

Security work should continue tightening pinned dependencies, provenance, review workflows, and repository policy without inventing governance signals that do not yet exist. Repository settings and review rules should mature alongside the code.

## Benchmark and documentation maturity

Benchmark documentation should stay aligned with the maintained benchmark taxonomy, and contributor-facing docs should remain explicit about lifecycle semantics, non-goals, and release expectations.

## Non-goals

The roadmap does not assume external adoption metrics, fixed release dates, or a broad framework surface. The package should remain a small Go library rather than turning into a general object-lifecycle manager.

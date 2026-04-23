<div align="center">

# arcoris.dev/pool Docs

**Documentation entry point for the typed temporary-object reuse runtime.**

[![README](https://img.shields.io/badge/Landing-README-0F766E?style=flat)](../README.md)
[![Package Contract](https://img.shields.io/badge/Package-doc.go-1D4ED8?style=flat)](../doc.go)
[![Lifecycle](https://img.shields.io/badge/Contract-Lifecycle-0F172A?style=flat)](./lifecycle.md)
[![Performance](https://img.shields.io/badge/Evidence-Performance-B45309?style=flat)](./performance/README.md)

[README](../README.md) · [Package Contract](../doc.go) · [Architecture](./architecture.md) · [Lifecycle](./lifecycle.md) · [Non-goals](./non-goals.md) · [Performance](./performance/README.md)

Temporary-object reuse · Explicit lifecycle policy · Narrow public runtime · Evidence-backed benchmarking

**Start:** [Package overview](#package-overview) · [Read by goal](#read-by-goal) · [Document map](#document-map) · [Source of truth](#source-of-truth)

</div>

## Package overview

`arcoris.dev/pool` is a small, policy-driven Go package for typed reuse of
temporary mutable values. The package keeps the runtime surface narrow, makes
ownership transfer explicit, and preserves a fixed return-path order for reuse
admission, drop observation, reset, and storage.

Use this page as the repository-facing map. Use [`../doc.go`](../doc.go) as
the Go-facing package contract.

## Read by goal

| If you want to... | Read first | Then continue with |
| --- | --- | --- |
| evaluate the package quickly | [README](../README.md) | [Lifecycle](./lifecycle.md) |
| understand exact runtime semantics | [Lifecycle](./lifecycle.md) | [Non-goals](./non-goals.md) |
| understand repository structure and layering | [Architecture](./architecture.md) | [Performance overview](./performance/README.md) |
| review scope boundaries before changing the API | [Non-goals](./non-goals.md) | [Architecture](./architecture.md) |
| inspect benchmarks, charts, and reports | [Performance overview](./performance/README.md) | [Initial baseline report](./performance/reports/2026-04-21-initial-baseline.md) |

## Document map

| Document | Role |
| --- | --- |
| [README](../README.md) | public landing page with quick orientation, example usage, and top-level links |
| [doc.go](../doc.go) | package contract for Go users and `pkg.go.dev` |
| [Architecture](./architecture.md) | repository layout, dependency direction, and layering boundaries |
| [Lifecycle](./lifecycle.md) | normative ownership, acquisition, and return-path semantics |
| [Non-goals](./non-goals.md) | explicit scope boundaries and exclusion rules |
| [Performance overview](./performance/README.md) | entry point to benchmarks, charts, methodology, and reports |
| [Benchmark methodology](./performance/methodology.md) | canonical benchmark collection and artifact workflow |
| [Benchmark matrix](./performance/benchmark-matrix.md) | benchmark inventory and suite structure |
| [Interpretation guide](./performance/interpretation-guide.md) | rules for reading benchmark evidence and avoiding overclaiming |
| [Reports contract](./performance/reports/README.md) | what committed performance reports must contain |

## Source of truth

- [`../doc.go`](../doc.go) defines the package-facing contract and public model.
- [`./lifecycle.md`](./lifecycle.md) defines the normative lifecycle and ownership rules.
- [`./non-goals.md`](./non-goals.md) defines product scope boundaries and proposal limits.
- [`./architecture.md`](./architecture.md) explains repository structure and dependency direction.
- [`./performance/README.md`](./performance/README.md) is the navigation root for benchmark evidence.

## Repository map

| Area | Purpose |
| --- | --- |
| `.` | public package code, tests, and benchmarks |
| `internal/backend/` | storage implementation behind the public runtime |
| `internal/testutil/` | shared helpers for tests and benchmarks |
| `docs/` | repository-facing documentation set |
| `bench/scripts/` | benchmark collection, comparison, profiling, and plotting automation |
| `bench/charts/` | generated SVG charts used by reports and README surfaces |

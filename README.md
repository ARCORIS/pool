<div align="center">

# arcoris.dev/pool

**Typed temporary-object reuse for Go hot paths.**

[![Start Here](https://img.shields.io/badge/Start-Docs%20Index-0F766E?style=flat)](docs/index.md)
[![Go](https://img.shields.io/badge/Go-1.25+-1D4ED8?style=flat)](go.mod)
[![Package Contract](https://img.shields.io/badge/Package-doc.go-0F172A?style=flat)](doc.go)
[![Performance](https://img.shields.io/badge/Evidence-Benchmarks%20%26%20Reports-B45309?style=flat)](docs/performance/README.md)

[Quick Start](#quick-start) · [Core Model](#core-model) · [Intended Use](#intended-use) · [Docs Index](docs/index.md) · [Performance](docs/performance/README.md)

Policy-driven reuse · Explicit ownership transfer · Canonical return-path semantics · Benchmark-first engineering

**Read by goal:** [Start here](#start-here) · [Lifecycle semantics](docs/lifecycle.md) · [Architecture](docs/architecture.md) · [Non-goals](docs/non-goals.md) · [Performance evidence](docs/performance/README.md)

</div>

`README.md` is the public landing page. [`docs/index.md`](docs/index.md) is the
repository documentation index, and [`doc.go`](doc.go) remains the package
contract for Go users and `pkg.go.dev`.

`arcoris.dev/pool` is a small Go package for typed reuse of temporary mutable
values. It stays close to the `sync.Pool` mental model, but keeps construction,
reset, reuse admission, and drop observation explicit as policy callbacks
instead of turning temporary-object reuse into a larger framework.

## Start here

| If you want to... | Read |
| --- | --- |
| understand the package in one screen | [Quick start](#quick-start) and [Core model](#core-model) |
| learn the exact ownership and return-path contract | [Lifecycle guide](docs/lifecycle.md) |
| understand repository structure and boundaries | [Architecture guide](docs/architecture.md) |
| see what the package intentionally excludes | [Non-goals guide](docs/non-goals.md) |
| inspect benchmarks, charts, and curated reports | [Performance overview](docs/performance/README.md) |

## Why it exists

In hot code paths, temporary mutable state usually ends up in one of two
shapes:

- repeated fresh allocation of scratch objects; or
- direct `sync.Pool` usage scattered through the codebase, with reset,
  admission, and drop logic repeated manually.

`arcoris.dev/pool` keeps the temporary-object model close to `sync.Pool`, but
makes the lifecycle policy explicit in one place:

- `New` defines how values are constructed on a miss;
- `Reset` defines how accepted values are cleaned before storage;
- `Reuse` decides whether a returned value is worth keeping;
- `OnDrop` observes explicit rejection by policy.

## Quick start

```go
package main

import "arcoris.dev/pool"

type Builder struct {
	buf []byte
}

func main() {
	p := pool.New(pool.Options[*Builder]{
		New: func() *Builder {
			return &Builder{
				buf: make([]byte, 0, 1024),
			}
		},
		Reset: func(b *Builder) {
			b.buf = b.buf[:0]
		},
		Reuse: func(b *Builder) bool {
			return cap(b.buf) <= 1024
		},
	})

	v := p.Get()
	v.buf = append(v.buf, "hello"...)

	// Ownership ends here. The caller must not use v after Put returns.
	p.Put(v)
}
```

## Core model

The package is built around three layers:

- `Options[T]` defines lifecycle policy;
- `Pool[T]` exposes the public `Get`/`Put` runtime;
- an internal backend stores already-clean reusable values.

The return path is canonical:

1. evaluate reuse admission;
2. if reuse is denied, invoke `OnDrop` and stop;
3. if reuse is allowed, invoke `Reset`;
4. store the cleaned value in the backend.

This ordering is part of the package contract. Admission runs before reset so
policy can inspect the real post-use state. Reset runs before backend storage
so retained values stay clean while idle.

## Ownership and concurrency

Ownership is explicit:

- after `Get`, the caller owns the value;
- after `Put`, the caller must treat the value as no longer owned.

A value must not be used after `Put` returns.

`Pool[T]` itself is safe for concurrent use by multiple goroutines. That does
not make the borrowed value automatically safe for concurrent use. Any
concurrency properties of `T` remain the responsibility of the caller and the
type.

## Choosing `T`

The package usually works best with pointer-like mutable values.

Why:

- the runtime avoids copying larger state around;
- reset can happen in place;
- the usage model aligns well with temporary-object pooling in Go;
- performance benefits are easier to justify.

Value types are supported, but large or frequently copied values should be
benchmarked carefully. The repository benchmark layer already includes explicit
pointer-like versus value-shape coverage.

## Intended use

This package is a good fit for:

- parser or decoder state;
- request-scoped scratch structures;
- builders, envelopes, and reusable work records;
- pointer-like temporary objects on hot paths.

It is usually worth considering when:

- values are created frequently;
- one borrowed value belongs to one logical operation at a time;
- reset is cheaper than repeated reconstruction;
- reuse policy benefits from being explicit and local.

## Non-goals

`arcoris.dev/pool` is intentionally not:

- a general resource manager;
- a borrow or lease tracker;
- a stable object inventory or cache;
- a queue, scheduler, or semaphore;
- a validation framework;
- a specialized memory or buffer pool;
- a rich commons-pool-style framework.

The package also does not promise:

- stable retention of returned values;
- borrow tracking in the default runtime;
- zero allocations for every `T`;
- that pooling is beneficial for all shapes of `T`.

## Documentation

| Document | Use when | Focus |
| --- | --- | --- |
| [Docs index](docs/index.md) | you want the repository map first | the best entry point into the documentation set |
| [Package contract](doc.go) | you want the Go-facing package contract | exported API intent and runtime model |
| [Architecture guide](docs/architecture.md) | you want the structural view | layers, boundaries, repository layout |
| [Lifecycle guide](docs/lifecycle.md) | you need precise runtime semantics | ownership, acquisition, return-path invariants |
| [Non-goals guide](docs/non-goals.md) | you are evaluating scope | explicit exclusions and product boundaries |
| [Performance overview](docs/performance/README.md) | you want the evidence trail | benchmarks, charts, methodology, reports |
| [Initial baseline report](docs/performance/reports/2026-04-21-initial-baseline.md) | you want the current curated snapshot | current benchmark interpretation and chart links |

## Performance overview

The repository keeps benchmark, profile, chart, and report layers as
first-class engineering artifacts. Snapshot charts are presentation summaries
over raw benchmark evidence, not replacements for the underlying artifacts.

[![Initial baseline package baseline time chart](bench/charts/initial-baseline-baselines-time-op.svg)](docs/performance/reports/2026-04-21-initial-baseline.md)

[![Initial baseline package baseline allocs chart](bench/charts/initial-baseline-baselines-allocs-op.svg)](docs/performance/reports/2026-04-21-initial-baseline.md)

Use the [performance overview](docs/performance/README.md) for the full
artifact workflow and the
[initial baseline report](docs/performance/reports/2026-04-21-initial-baseline.md)
for the current curated snapshot.

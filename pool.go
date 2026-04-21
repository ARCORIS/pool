/*
   Copyright 2026 The ARCORIS Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package pool

import "arcoris.dev/pool/internal/backend"

// Pool provides typed reuse of temporary values.
//
// Pool is the primary public runtime type of this package. It combines three
// internal responsibilities into one externally simple API:
//
//  1. value construction on the slow path, via [Options.New];
//  2. lifecycle policy on return, via [Options.Reset], [Options.Reuse], and
//     [Options.OnDrop];
//  3. low-level storage and retrieval, via an internal sync.Pool-backed
//     backend.
//
// In practical terms, Pool exists so that callers can work with the following
// extremely small surface:
//
//	v := p.Get()
//	... use v as a temporary object ...
//	p.Put(v)
//
// without having to manually repeat reset, reuse admission, and backend
// interaction logic throughout the codebase.
//
// # Intended use
//
// Pool is intended for temporary reusable values that are frequently created,
// mutated within a single logical operation, and then returned for possible
// reuse. Typical examples include:
//
//   - parser or decoder state objects;
//   - request-scoped scratch structures;
//   - reusable builders, envelopes, or temporary frames;
//   - short-lived mutable helper structs used on hot paths.
//
// Pool is usually not a good fit for:
//
//   - long-lived domain entities whose ownership escapes the current operation;
//   - values that must remain reachable after Put returns;
//   - objects requiring stable inventory guarantees or bounded-capacity borrow
//     semantics;
//   - lifecycle models that need validation-on-borrow, idle eviction,
//     reference counting, or blocking acquisition.
//
// # Design model
//
// Pool deliberately follows the temporary-object model of sync.Pool rather than
// the richer semantics of a full object-lifecycle manager. In particular:
//
//   - objects returned to the pool may later disappear from the backend without
//     notice;
//   - the pool does not promise stable retention of previously returned values;
//   - the pool does not track borrow state or detect double Put misuse;
//   - the pool does not impose a mandatory interface on T.
//
// The package instead keeps lifecycle policy explicit through [Options]. This
// allows the same type to be pooled under different reuse rules without forcing
// that type to embed reuse semantics directly into its own definition.
//
// # Ownership
//
// The caller owns a value obtained from [Pool.Get] until that value is passed
// to [Pool.Put]. After Put returns, the caller MUST treat the value as no
// longer owned. It must not be used, mutated, shared, or published as if it
// still belonged to the caller.
//
// Pool does not attempt to enforce this rule at runtime in the default build.
// Correct ownership remains the responsibility of the caller.
//
// # Concurrency
//
// Pool is safe for concurrent use by multiple goroutines. Backend storage is
// delegated to sync.Pool through an internal adapter, and lifecycle policy is
// immutable after construction.
//
// What is and is not concurrency-safe:
//
//   - concurrent calls to Get and Put on the same *Pool are supported;
//   - the value returned by Get belongs to one logical owner until Put;
//   - a borrowed value must not be concurrently mutated unless the value type T
//     provides its own synchronization.
//
// # Zero value
//
// The zero value of Pool is not ready for use. Construct a pool with [New].
// This is intentional because a valid pool requires a construction policy and a
// backend assembled from that policy.
//
// # Copying
//
// Pool values should be treated as configuration-bearing runtime handles and
// used through *Pool. New returns *Pool for this reason.
//
// Although Pool mostly contains immutable policy references after construction,
// callers SHOULD NOT copy Pool values around by value. Doing so provides no
// benefit and obscures ownership of the runtime handle.
//
// # Performance notes
//
// The package is optimized for clarity of lifecycle and stable hot path shape,
// not for exotic specialization. The fast path of Pool.Get is a backend get.
// The fast path of Pool.Put is:
//
//  1. evaluate reuse policy;
//  2. optionally observe drop and return;
//  3. reset accepted value;
//  4. store accepted value.
//
// There are no repeated nil-hook checks on the hot path because [Options] are
// resolved once during construction.
//
// T may be any type, but pointer-like temporary values are usually the best
// fit. They avoid copying large mutable state and align with the intended use
// of pooling in Go.
//
// Example
//
//	type ParserState struct {
//		Input  []byte
//		Offset int
//		Tokens []Token
//		Err    error
//	}
//
//	states := pool.New(pool.Options[*ParserState]{
//		New: func() *ParserState {
//			return &ParserState{
//				Tokens: make([]Token, 0, 64),
//			}
//		},
//		Reset: func(s *ParserState) {
//			s.Input = nil
//			s.Offset = 0
//			s.Tokens = s.Tokens[:0]
//			s.Err = nil
//		},
//		Reuse: func(s *ParserState) bool {
//			return cap(s.Tokens) <= 4_096
//		},
//	})
//
//	state := states.Get()
//	defer states.Put(state)
//
//	// use state within the current operation only
//
// # Internal structure
//
// Pool intentionally keeps its internal structure simple:
//
//   - backend owns only storage and retrieval;
//   - lifecycle owns only return-path semantics;
//   - Pool itself owns public orchestration.
//
// This separation keeps the public runtime readable while still allowing the
// project to evolve internal implementation details conservatively.
//
// The type is expected to change rarely once stabilized. As a result, comments
// in this file are intentionally explicit and normative.
type Pool[T any] struct {
	backend   *backend.SyncPool[T]
	lifecycle lifecycle[T]
}

// New constructs a typed Pool from the supplied lifecycle policy.
//
// New is the only supported constructor for Pool. It validates and resolves the
// public [Options] value, assembles the internal lifecycle controller, and
// creates the internal sync.Pool-backed backend.
//
// # Construction steps
//
// New performs the following work in order:
//
//  1. validate and normalize [Options] via Options.resolve;
//  2. construct the internal backend using the resolved New policy;
//  3. construct the lifecycle controller using the resolved reset/reuse/drop
//     policies;
//  4. return the assembled *Pool.
//
// Because optional hooks are normalized during construction, the returned Pool
// does not need to perform nil checks for Reset, Reuse, or OnDrop on every Put.
//
// # Panics
//
// New panics if [Options.New] is nil. This is a construction-time contract
// violation because the pool cannot materialize a value when the backend is
// empty without a creation policy.
//
// New may also panic if the internal backend constructor contract is violated,
// though under normal use that cannot happen independently of options
// validation because the same resolved constructor is forwarded to the backend.
//
// Example
//
//	p := pool.New(pool.Options[*Builder]{
//		New: func() *Builder {
//			return &Builder{}
//		},
//		Reset: func(b *Builder) {
//			b.Reset()
//		},
//	})
//
// The returned pool is ready for concurrent Get/Put use immediately.
func New[T any](options Options[T]) *Pool[T] {
	resolved := options.resolve()

	return &Pool[T]{
		backend:   backend.NewSyncPool(resolved.newFn),
		lifecycle: newLifecycle(resolved),
	}
}

// Get returns a temporary reusable value of type T.
//
// # Behaviour
//
// Get asks the internal backend for a previously returned reusable value. If no
// such value is currently available, the backend constructs one using the
// resolved [NewFunc] captured when the pool was created.
//
// Get performs no lifecycle work beyond acquisition. In particular, Get does
// not:
//
//   - reset the value;
//   - validate that the value should still exist in the backend;
//   - decide whether the value should later be kept;
//   - attach ownership tracking metadata.
//
// The value returned by Get is logically owned by the caller until Put is
// called.
//
// # State guarantees
//
// A value returned by Get is either:
//
//   - a newly constructed value produced by [Options.New]; or
//   - a previously returned value that was accepted for reuse and reset before
//     being stored back into the backend.
//
// In both cases, callers should reason about the value as ready for immediate
// use according to the invariants defined by their construction and reset
// policies.
//
// # Panics
//
// Get panics if called on a nil *Pool. This is treated as a hard misuse of the
// public runtime handle.
//
// Example
//
//	v := p.Get()
//	// mutate v within the current operation
//	p.Put(v)
func (p *Pool[T]) Get() T {
	if p == nil {
		panic("pool: Get called on nil Pool")
	}

	return p.backend.Get()
}

// Put returns a value to the pool according to the configured lifecycle policy.
//
// # Canonical return path
//
// Put is the public entry point for the package's return-path semantics. It
// delegates the detailed order of operations to the internal lifecycle
// controller, which performs the following steps:
//
//  1. evaluate the configured [ReuseFunc];
//  2. if reuse is denied, invoke [DropFunc] and stop;
//  3. if reuse is allowed, invoke [ResetFunc];
//  4. store the cleaned value in the backend for possible future reuse.
//
// This order is intentional and should not be changed casually. In particular,
// reuse admission happens before reset so that the admission policy can inspect
// the value in the state it actually accumulated during use.
//
// # Ownership
//
// After Put returns, the caller MUST treat the value as no longer owned,
// regardless of whether the value was retained or dropped. Put represents the
// end of the caller's lifecycle responsibility for that borrowed value.
//
// # What Put does not do
//
// Put does not:
//
//   - guarantee that the value will be retained indefinitely;
//   - detect double Put misuse in the default runtime;
//   - provide destruction/finalization semantics for dropped or later-evicted
//     values;
//   - make the value safe for post-Put inspection by the caller.
//
// # Panics
//
// Put panics if called on a nil *Pool. This is treated as misuse of the public
// runtime handle.
//
// Example
//
//	state := p.Get()
//	defer p.Put(state)
//
//	// use state during the current operation only
func (p *Pool[T]) Put(value T) {
	if p == nil {
		panic("pool: Put called on nil Pool")
	}

	p.lifecycle.Release(p.backend, value)
}

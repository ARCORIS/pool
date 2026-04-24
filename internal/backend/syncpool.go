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

// Package backend contains internal storage backends used by the public pool
// runtime.
//
// The package is intentionally not exported because backend choice is an
// implementation detail of arcoris.dev/pool. Callers should reason in terms of
// lifecycle policy exposed by pool.Options and pool.Pool, not in terms of the
// concrete low-level reuse mechanism.
package backend

import (
	"fmt"
	"sync"
)

// SyncPool is a typed internal adapter over [sync.Pool].
//
// # Purpose
//
// The public package-level Pool[T] is responsible for lifecycle policy:
// creation, reset, reuse admission, and explicit drop handling. None of that
// policy belongs in the low-level storage backend. This type exists to isolate
// the backend-specific details of storing and retrieving reusable values via
// [sync.Pool] while keeping the public runtime small and policy-oriented.
//
// In practice, SyncPool is the boundary that converts an untyped any-based
// backend into a typed generic backend suitable for the rest of the package.
// It centralizes the only place where values cross the boundary between the
// generic public API and the underlying [sync.Pool] API.
//
// Why this type exists instead of using [sync.Pool] directly everywhere
//
//   - It keeps all [sync.Pool]-specific mechanics in one internal location.
//   - It prevents the public pool runtime from repeating any <-> T conversion
//     logic.
//   - It makes future backend replacement or debug instrumentation local.
//   - It keeps package-level architecture explicit: policy above, storage
//     backend below.
//
// Contract
//
//   - SyncPool does not implement reset, validation, admission, or drop
//     policy. Those decisions are made by the public Pool[T].
//   - SyncPool only stores and retrieves values of type T.
//   - NewSyncPool requires a non-nil constructor.
//   - The zero value of SyncPool is not ready for use.
//   - SyncPool values must not be copied after first use because the type owns
//     an embedded [sync.Pool].
//
// # Concurrency
//
// SyncPool is safe for concurrent use because it delegates storage and reuse
// mechanics to [sync.Pool], which is itself designed for concurrent access.
//
// # Performance notes
//
// The adapter intentionally stays minimal:
//
//   - no extra indirection beyond the typed wrapper itself;
//   - no runtime hook dispatch inside Get/Put;
//   - no statistics or ownership tracking in the hot path;
//   - only one typed assertion site on Get.
//
// For the intended use of arcoris.dev/pool, this keeps the backend thin enough
// to remain an implementation detail rather than becoming a second runtime
// layer.
//
// T is typically expected to be a pointer-like reusable value, such as
// *ParserState or *RequestContext, but the adapter itself is generic and does
// not impose that restriction.
//
// # Internal invariant
//
// Every value stored in the underlying [sync.Pool] must have dynamic type T.
// Any violation of that invariant indicates an internal programming error in
// this repository, not a recoverable user-space condition.
//
// The type is deliberately small and stable. If the project later adds debug
// instrumentation, alternate backends, or build-tag-specific behaviour, those
// changes should continue to preserve this file as the narrow typed bridge over
// [sync.Pool].
type SyncPool[T any] struct {
	pool sync.Pool
}

// NewSyncPool constructs a typed [sync.Pool]-backed backend.
//
// newFn is mandatory. It is used as the slow-path constructor whenever the
// underlying [sync.Pool] cannot provide a reusable value.
//
// NewSyncPool panics if newFn is nil. Although higher layers already validate
// this condition when resolving public Options, the backend defends its own
// contract explicitly so that it remains correct and self-contained when read
// or tested in isolation.
//
// The constructor installed into [sync.Pool.New] must return values whose dynamic
// type is exactly T. That invariant is required for Get to remain type-safe.
func NewSyncPool[T any](newFn func() T) *SyncPool[T] {
	if newFn == nil {
		panic("pool: newFn must not be nil")
	}

	p := &SyncPool[T]{}
	p.pool.New = func() any {
		return newFn()
	}
	return p
}

// Get returns a reusable value of type T from the backend.
//
// Behaviour
//
//   - If [sync.Pool] has a previously stored value, that value is returned.
//   - Otherwise [sync.Pool] invokes the constructor installed by NewSyncPool.
//   - The result is asserted back to T and returned to the caller.
//
// This method performs no lifecycle work beyond retrieval. In particular, Get
// does not reset the value, validate it, or decide whether it should have been
// retained. Those responsibilities belong to the public pool runtime.
//
// # Panics
//
// Get panics if the backend returns a value whose dynamic type is not T. Such a
// panic represents an internal invariant violation: something in the package
// stored the wrong value into the backend. This is intentionally treated as a
// hard failure rather than a recoverable error because continuing after type
// corruption would make the runtime semantics unsound.
func (p *SyncPool[T]) Get() T {
	if p == nil {
		panic("pool: Get called on nil SyncPool")
	}

	return typedPoolValue[T](p.pool.Get())
}

// Put stores a value for future reuse.
//
// Put intentionally performs no reset or policy checks. The caller is expected
// to have already decided that the value is eligible for reuse and to have
// brought it into a clean state.
//
// In other words, Put is the final low-level storage step of the return path,
// not the place where lifecycle policy is enforced.
//
// Put panics on a nil receiver because using an uninitialized backend is an
// internal programming error.
func (p *SyncPool[T]) Put(value T) {
	if p == nil {
		panic("pool: Put called on nil SyncPool")
	}

	p.pool.Put(value)
}

// typedPoolValue converts a raw backend value into the expected generic type.
//
// Keeping the assertion logic in a small helper makes the backend fast path
// readable and gives tests a deterministic boundary for impossible type-mismatch
// scenarios. The helper does not attempt recovery: a mismatched value means the
// internal backend invariant has already been violated.
func typedPoolValue[T any](value any) T {
	typed, ok := value.(T)
	if !ok {
		panic(unexpectedTypePanic[T](value))
	}
	return typed
}

// unexpectedTypePanic formats a stable panic message for impossible backend
// type mismatches.
//
// A dedicated helper keeps the hot path readable and makes the invariant
// violation explicit in one place. The message intentionally reports both the
// received dynamic type and the expected generic type when it can be inferred
// from a zero value.
func unexpectedTypePanic[T any](value any) string {
	var expected T
	return fmt.Sprintf(
		"pool: sync.Pool returned unexpected value of type %T; expected %T",
		value,
		expected,
	)
}

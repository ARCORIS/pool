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

// Options defines the lifecycle policy for a typed [Pool].
//
// A pool built from Options follows this return path:
//
//  1. [Pool.Put] receives a value.
//  2. [ReuseFunc] decides whether the value should be retained.
//  3. If reuse is denied, [DropFunc] is called and the value is discarded.
//  4. If reuse is allowed, [ResetFunc] prepares the value for the next user.
//  5. The clean value is stored in the backend for future [Pool.Get] calls.
//
// The acquisition path is intentionally simpler:
//
//  1. [Pool.Get] asks the backend for a reusable value.
//  2. If none is available, [NewFunc] constructs one.
//  3. The value is returned to the caller as-is.
//
// Design notes:
//
//   - Options makes lifecycle policy explicit instead of encoding it into a
//     mandatory interface implemented by T.
//   - This keeps domain types decoupled from the pool package.
//   - Different pools may apply different reuse policies to the same type.
//
// Options values are configuration, not runtime state. They are consumed by
// [New] and resolved into an internal immutable policy set.
//
// Zero value:
//
// The zero value of Options is invalid because [NewFunc] is required. Callers
// MUST provide at least the [Options.New] field.
//
// Recommended shape:
//
// In performance-sensitive code, T SHOULD usually be a pointer-like type. This
// keeps Get/Put cheap, avoids copying large mutable state, and matches the
// common object-pooling pattern in Go.
//
// Minimal example:
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
// Example with an explicit reuse policy:
//
//	p := pool.New(pool.Options[*RequestScratch]{
//		New: func() *RequestScratch {
//			return &RequestScratch{
//				Fields: make([]Field, 0, 32),
//			}
//		},
//		Reset: func(s *RequestScratch) {
//			s.Path = ""
//			s.Fields = s.Fields[:0]
//			s.Payload = s.Payload[:0]
//		},
//		Reuse: func(s *RequestScratch) bool {
//			return cap(s.Payload) <= 64<<10
//		},
//		OnDrop: func(s *RequestScratch) {
//			metrics.RequestScratchDrops.Add(1)
//		},
//	})
//
// Example showing why callbacks are preferred over a mandatory interface on T:
//
//	// The same type can be reused with different retention policies.
//	fastPath := pool.New(pool.Options[*Frame]{
//		New: newFrame,
//		Reset: resetFrame,
//		Reuse: func(f *Frame) bool { return cap(f.Body) <= 8<<10 },
//	})
//
//	batchPath := pool.New(pool.Options[*Frame]{
//		New: newFrame,
//		Reset: resetFrame,
//		Reuse: func(f *Frame) bool { return cap(f.Body) <= 128<<10 },
//	})
//
// These two pools can coexist even though they manage the same concrete type.
// If reuse logic were embedded directly in Frame, this separation would be
// much harder to express cleanly.
type Options[T any] struct {
	// New constructs a fresh value when the backend has no reusable instance
	// available.
	//
	// This field is required. [New] panics if Options.New is nil.
	//
	// New SHOULD return a value that is immediately valid for the caller of
	// [Pool.Get]. For pointer-like T, this usually means allocating the object
	// and initializing any slices, maps, or nested reusable fields to their
	// desired initial state.
	New NewFunc[T]

	// Reset prepares a value for reuse before it is stored back into the pool.
	//
	// If Reset is nil, the pool uses a no-op reset policy.
	//
	// Reset is called only when the reuse policy accepts the value.
	Reset ResetFunc[T]

	// Reuse decides whether a value should be retained for future reuse.
	//
	// If Reuse is nil, the pool uses an "always reuse" policy.
	//
	// Reuse is evaluated before Reset. When Reuse returns false, the value is
	// discarded and Reset is skipped.
	Reuse ReuseFunc[T]

	// OnDrop observes values that are explicitly rejected by Reuse.
	//
	// If OnDrop is nil, no drop callback is executed.
	//
	// OnDrop is not a general destruction hook. It is only invoked on the
	// explicit "rejected for reuse" path in [Pool.Put].
	OnDrop DropFunc[T]
}

// NewFunc constructs a new value for a [Pool] when the backend has no reusable
// instance available.
//
// NewFunc is the only mandatory policy hook in [Options]. Without it, the pool
// has no way to materialize a value on the slow path.
//
// The function MUST return a value that is immediately ready for use by the
// caller of [Pool.Get]. In other words, callers of Get SHOULD NOT be required
// to perform any additional initialization merely to make the returned value
// safe to touch.
//
// Semantics:
//
//   - NewFunc is called lazily, only when the pool backend cannot provide a
//     previously returned reusable value.
//   - The returned value becomes owned by the caller of [Pool.Get].
//   - The value returned by NewFunc is not automatically reset by the pool;
//     NewFunc itself is responsible for producing a valid initial state.
//
// Performance guidance:
//
// For most real-world use cases, T SHOULD be a pointer-like type such as
// *MyState, *RequestContext, or *Builder. Pointer-like values are typically the
// best fit for object pooling because they avoid copying large mutable state
// and align well with the intended usage pattern of sync.Pool-backed designs.
//
// Example:
//
//	opts := pool.Options[*ParserState]{
//		New: func() *ParserState {
//			return &ParserState{
//				Tokens: make([]Token, 0, 64),
//			}
//		},
//	}
//
// Example for a value type (supported, but usually less desirable for hot
// paths due to copying):
//
//	opts := pool.Options[SmallValue]{
//		New: func() SmallValue {
//			return SmallValue{}
//		},
//	}
type NewFunc[T any] func() T

// ResetFunc prepares a value for future reuse before it is placed back into
// the pool.
//
// ResetFunc is called by [Pool.Put] only after the pool has decided that the
// value is eligible for reuse. It is never called during [Pool.Get]. This is a
// deliberate design choice:
//
//   - objects retained by the pool remain in a clean state,
//   - references can be released as early as possible,
//   - the cost of reset is paid only on the return path,
//   - the acquisition path stays minimal and predictable.
//
// ResetFunc SHOULD restore the value to the same logical state that callers
// would expect from a freshly created instance returned by [NewFunc].
//
// Typical reset work includes:
//
//   - clearing slices via s = s[:0],
//   - zeroing scalar fields,
//   - dropping transient references,
//   - resetting nested builders or temporary buffers,
//   - restoring invariants required by the next user of the object.
//
// ResetFunc MUST NOT:
//
//   - return the value to another pool,
//   - publish the value to other goroutines,
//   - assume that it will never be called more than once over the lifetime of
//     a reusable instance,
//   - perform ownership-sensitive logic that belongs to application code.
//
// ResetFunc MAY be nil in [Options]. In that case the pool uses a no-op reset
// policy.
//
// Example:
//
//	Reset: func(s *ParserState) {
//		s.Input = nil
//		s.Offset = 0
//		s.Tokens = s.Tokens[:0]
//		s.Err = nil
//	}
type ResetFunc[T any] func(T)

// ReuseFunc decides whether a value is eligible to be stored for reuse.
//
// ReuseFunc is evaluated by [Pool.Put] before [ResetFunc] is executed. If it
// returns false, the value is dropped instead of being reset and stored.
//
// This hook exists because not every temporary object should be retained.
// Common examples include:
//
//   - a scratch structure that grew too large,
//   - an object that holds an oversized internal slice or map,
//   - an instance that entered a poisoned or non-reusable state,
//   - an object whose retained memory would be more expensive than allocating a
//     fresh replacement later.
//
// ReuseFunc SHOULD be:
//
//   - deterministic for a given object state,
//   - fast enough for the return path,
//   - free of side effects beyond reading the value.
//
// ReuseFunc MUST NOT mutate the object. Mutation belongs in [ResetFunc].
// Separating admission from reset keeps the control flow explicit and avoids
// subtle policy coupling.
//
// ReuseFunc MAY be nil in [Options]. In that case the pool uses an "always
// reuse" policy.
//
// Example:
//
//	Reuse: func(s *ParserState) bool {
//		return cap(s.Tokens) <= 4_096
//	}
//
// Example for a request-scoped scratch object with a temporary byte field:
//
//	Reuse: func(r *RequestContext) bool {
//		return cap(r.Scratch) <= 64<<10
//	}
type ReuseFunc[T any] func(T) bool

// DropFunc observes values that were rejected for reuse.
//
// DropFunc is invoked by [Pool.Put] only when [ReuseFunc] returns false.
// Typical uses are limited and low-level:
//
//   - optional metrics,
//   - debug-only accounting,
//   - release of external non-memory resources that do not belong in the
//     ordinary reset path,
//   - tracing why objects are discarded.
//
// DropFunc MUST be treated as a best-effort callback. It is not a lifecycle
// guarantee that a value will always pass through this hook before becoming
// unreachable. In particular, values obtained from a sync.Pool-style backend
// may disappear without an explicit callback once stored in the backend.
//
// DropFunc SHOULD remain lightweight. Heavy logging, allocation, blocking I/O,
// or cross-goroutine coordination in this hook will often negate the benefit
// of pooling.
//
// DropFunc MAY be nil in [Options]. In that case the pool performs no action
// on dropped values.
//
// Example:
//
//	OnDrop: func(s *ParserState) {
//		metrics.ParserStateDrops.Add(1)
//	}
type DropFunc[T any] func(T)

// resolvedOptions is the fully populated internal policy set used by Pool
// after construction.
//
// This type exists to separate public configuration from runtime policy
// objects. After resolution, the pool no longer needs to check for nil hooks
// on the hot path.
type resolvedOptions[T any] struct {
	newFn   NewFunc[T]
	resetFn ResetFunc[T]
	reuseFn ReuseFunc[T]
	dropFn  DropFunc[T]
}

// resolve validates the public Options value and replaces all optional nil
// hooks with stable internal defaults.
//
// New is mandatory because the backend must be able to construct a fresh value
// whenever no reusable instance is available. Optional hooks are normalized to
// no-op or always-true policies so that the runtime path can remain simple.
func (o Options[T]) resolve() resolvedOptions[T] {
	if o.New == nil {
		panic("pool: Options.New must not be nil")
	}

	resetFn := o.Reset
	if resetFn == nil {
		resetFn = noopReset[T]
	}

	reuseFn := o.Reuse
	if reuseFn == nil {
		reuseFn = alwaysReuse[T]
	}

	dropFn := o.OnDrop
	if dropFn == nil {
		dropFn = noopDrop[T]
	}

	return resolvedOptions[T]{
		newFn:   o.New,
		resetFn: resetFn,
		reuseFn: reuseFn,
		dropFn:  dropFn,
	}
}

// noopReset is the default reset policy used when Options.Reset is nil.
func noopReset[T any](T) {}

// alwaysReuse is the default admission policy used when Options.Reuse is nil.
func alwaysReuse[T any](T) bool {
	return true
}

// noopDrop is the default drop callback used when Options.OnDrop is nil.
func noopDrop[T any](T) {}

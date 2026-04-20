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

// lifecycleSink is the narrow storage capability required by the lifecycle
// layer in order to complete the return path of a reusable value.
//
// # Why this interface exists
//
// The public pool runtime has two fundamentally different responsibilities:
//
//  1. lifecycle policy — deciding whether a value may be reused, resetting it,
//     and observing rejected values;
//  2. storage backend interaction — physically storing an already accepted,
//     already reset value for future reuse.
//
// These concerns should not be collapsed into one monolithic implementation
// block. The lifecycle layer should only depend on the minimal capability it
// needs from the backend: the ability to accept a value that is already ready
// to be stored.
//
// Keeping the sink contract this small has several benefits:
//
//   - lifecycle code remains independent of sync.Pool-specific details;
//   - the public runtime can be structured as policy above, backend below;
//   - alternate internal backends or debug wrappers can be introduced without
//     rewriting lifecycle semantics;
//   - the canonical order of operations for Put stays centralized in one place.
//
// This interface is intentionally unexported. It is an internal assembly point
// for the package runtime, not a public extension mechanism.
//
// # Contract
//
// Implementations MUST treat Put as the final storage step for a value that has
// already been accepted for reuse and fully reset. In particular, implementations
// of lifecycleSink MUST NOT assume responsibility for:
//
//   - deciding whether the value should be reused;
//   - resetting the value;
//   - invoking drop callbacks;
//   - tracking logical ownership.
//
// Those responsibilities belong to the lifecycle controller defined in this
// file and to the public pool runtime that uses it.
//
// In the current architecture, the primary implementation is
// internal/backend.SyncPool[T].
type lifecycleSink[T any] interface {
	Put(T)
}

// lifecycle encapsulates the canonical return-path semantics for values managed
// by a typed Pool.
//
// # Scope
//
// This type exists to own the semantics of what happens when a caller returns a
// temporary reusable value. It is deliberately separate from both:
//
//   - public configuration ([Options]), which describes policy declaratively;
//   - backend storage ([lifecycleSink]), which stores values mechanically.
//
// The purpose of lifecycle is therefore not to be a second public abstraction,
// but to serve as the internal semantic engine of the package runtime.
//
// # Canonical return path
//
// For a value v returned by a caller, lifecycle follows this strict order:
//
//  1. evaluate the reuse admission policy;
//  2. if reuse is denied, invoke the drop callback and stop;
//  3. if reuse is allowed, reset the value into a clean reusable state;
//  4. pass the clean value to the backend sink for storage.
//
// That order is intentional and should remain stable.
//
// # Why admission happens before reset
//
// Reuse admission is evaluated before reset so that the package can decide
// whether a value should be retained based on the state it actually accumulated
// while in use. Typical examples include:
//
//   - a temporary object whose internal slice grew beyond an acceptable bound;
//   - an instance that entered a poisoned or non-reusable state;
//   - a scratch structure whose retained memory would be too expensive to keep.
//
// If reset were performed first, policy code would lose visibility into that
// state and could no longer make accurate retention decisions.
//
// # Why reset happens before backend storage
//
// Reset is applied before the value re-enters backend storage so that:
//
//   - retained values remain in a clean state while idle in the backend;
//   - references are dropped as early as possible;
//   - the acquisition path stays lean and predictable;
//   - callers of Pool.Get observe values that are already safe to use.
//
// # Non-goals
//
// lifecycle intentionally does not:
//
//   - track ownership or detect double Put operations;
//   - recover from misuse or invalid application-level object state;
//   - coordinate between goroutines;
//   - provide validation-on-borrow or validation-on-return hooks;
//   - decide how values are physically stored or reclaimed by the backend.
//
// Those concerns either belong to application code, future debug-only tooling,
// or the backend itself.
//
// # Stability
//
// The project is intended to change conservatively after stabilization. For
// that reason, this type is deliberately narrow and explicit. The lifecycle
// semantics embodied here should be treated as part of the package's core
// internal contract.
type lifecycle[T any] struct {
	reset  ResetFunc[T]
	reuse  ReuseFunc[T]
	onDrop DropFunc[T]
}

// newLifecycle constructs the internal lifecycle controller from already
// resolved options.
//
// resolvedOptions guarantees that all policy hooks are non-nil. This means the
// lifecycle controller can execute its hot-path logic without repeatedly
// checking for missing callbacks.
//
// The constructor does not perform additional validation because validation and
// defaulting belong in [Options.resolve]. By the time execution reaches this
// layer, policy should already be normalized and immutable.
//
// This separation is intentional:
//
//   - Options.resolve owns configuration correctness;
//   - newLifecycle owns semantic assembly;
//   - the public Pool runtime owns orchestration.
func newLifecycle[T any](resolved resolvedOptions[T]) lifecycle[T] {
	return lifecycle[T]{
		reset:  resolved.resetFn,
		reuse:  resolved.reuseFn,
		onDrop: resolved.dropFn,
	}
}

// AllowReuse reports whether the value is eligible to be retained for future
// reuse.
//
// This method is a thin semantic wrapper over the configured [ReuseFunc]. It is
// intentionally kept separate from Reset and Release so that the runtime can,
// when necessary, reason about admission as an explicit step rather than a
// hidden side effect.
//
// In the normal public runtime, callers will most often use [lifecycle.Release]
// instead of invoking AllowReuse directly. The method exists because making the
// individual steps addressable keeps the lifecycle model readable and testable.
//
// AllowReuse must be free of side effects beyond whatever the configured policy
// itself performs. The package expects the configured [ReuseFunc] to behave as
// a read-only admission decision.
func (l lifecycle[T]) AllowReuse(value T) bool {
	return l.reuse(value)
}

// ResetForReuse prepares a value for backend storage after admission has
// already succeeded.
//
// The method intentionally does not perform a reuse check. Callers must only
// invoke it after deciding that the value should be retained.
//
// Separating admission and reset clarifies the lifecycle model and avoids
// conflating two different concerns:
//
//   - admission answers "should this value be kept?";
//   - reset answers "how should this value be cleaned before storage?".
//
// In the default runtime, this method is primarily exercised through
// [lifecycle.Release]. It remains separately exposed to package-internal code so
// that tests, alternate runtime assemblies, or debug paths can drive the steps
// explicitly if needed.
func (l lifecycle[T]) ResetForReuse(value T) {
	l.reset(value)
}

// ObserveDrop handles a value that was explicitly rejected for reuse.
//
// ObserveDrop does not attempt to reset or retain the value. Its only role is
// to run the configured [DropFunc], if any, so that low-cost best-effort
// observation can take place.
//
// The method intentionally does not guarantee that every ultimately discarded
// object in the system will pass through this code path. Once a value has been
// accepted into a sync.Pool-style backend, that backend may later release it
// without further callbacks. ObserveDrop therefore represents explicit
// rejection-by-policy, not a universal finalizer mechanism.
func (l lifecycle[T]) ObserveDrop(value T) {
	l.onDrop(value)
}

// Release executes the canonical return path for a value and, if admitted,
// stores it in the provided backend sink.
//
// # Behaviour
//
// Release performs the following steps in order:
//
//  1. evaluate the reuse policy via [lifecycle.AllowReuse];
//  2. if reuse is denied, invoke [lifecycle.ObserveDrop] and return;
//  3. reset the value via [lifecycle.ResetForReuse];
//  4. forward the clean value to the backend sink.
//
// This method is intended to be the normal implementation of Pool.Put in the
// public runtime.
//
// # Ownership
//
// Callers must treat the value as no longer owned after Release returns,
// regardless of whether the value was stored or dropped. Release represents the
// end of the caller's lifecycle responsibility for the current borrowed value.
//
// # Sink requirements
//
// sink must not be nil. A nil sink indicates an internal programming error in
// the package runtime assembly and results in panic.
//
// # Panics
//
// Release panics if sink is nil. This is treated as a hard internal invariant
// violation because the lifecycle layer cannot complete its contract without a
// valid storage destination.
//
// Example conceptual flow:
//
//	value := p.backend.Get()
//	... caller uses value ...
//	p.lifecycle.Release(p.backend, value)
//
// The actual public Pool implementation may structure its code differently, but
// the semantic order of operations should remain identical.
func (l lifecycle[T]) Release(sink lifecycleSink[T], value T) {
	if sink == nil {
		panic("pool: nil lifecycle sink")
	}

	if !l.AllowReuse(value) {
		l.ObserveDrop(value)
		return
	}

	l.ResetForReuse(value)
	sink.Put(value)
}

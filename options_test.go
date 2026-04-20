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

import (
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

// optionsHookCalls records which resolved hooks were actually executed.
//
// The tests use it both to prove that resolve() itself stays side-effect free
// and to verify that resolved callbacks still dispatch to the original hooks.
type optionsHookCalls struct {
	new   int
	reset int
	reuse int
	drop  int
}

// optionsTestObject models a pointer-like pooled value.
//
// Tests mutate its fields through Reset and OnDrop hooks so they can verify
// that resolve() preserves hook behaviour and that default hooks stay inert.
type optionsTestObject struct {
	count     int
	payload   []byte
	resetSeen bool
	dropSeen  bool
}

// optionsTestValue exercises Options with a value-typed T.
//
// Keeping a non-pointer fixture in the suite makes the default-policy tests
// prove that value instantiations behave correctly as well.
type optionsTestValue struct {
	N int
}

func TestOptionsResolve(t *testing.T) {
	t.Run("panics when New is nil", func(t *testing.T) {
		testutil.AssertPanicMessage(
			t,
			"Options.resolve() with nil New",
			func() {
				var opts Options[*optionsTestObject]
				_ = opts.resolve()
			},
			"pool: Options.New must not be nil",
		)
	})

	t.Run("does not invoke hooks during resolution", func(t *testing.T) {
		calls := optionsHookCalls{}
		opts := Options[*optionsTestObject]{
			New: func() *optionsTestObject {
				calls.new++
				return &optionsTestObject{}
			},
			Reset: func(*optionsTestObject) {
				calls.reset++
			},
			Reuse: func(*optionsTestObject) bool {
				calls.reuse++
				return true
			},
			OnDrop: func(*optionsTestObject) {
				calls.drop++
			},
		}

		_ = opts.resolve()

		if calls != (optionsHookCalls{}) {
			t.Fatalf("resolve() invoked hooks eagerly: got %+v, want all counters to remain zero", calls)
		}
	})

	t.Run("installs defaults for value type", func(t *testing.T) {
		resolved := Options[optionsTestValue]{
			New: func() optionsTestValue {
				return optionsTestValue{N: 41}
			},
		}.resolve()

		if got := resolved.newFn(); got != (optionsTestValue{N: 41}) {
			t.Fatalf("resolved newFn() = %+v, want %+v", got, optionsTestValue{N: 41})
		}

		value := optionsTestValue{N: 99}
		resolved.resetFn(value)
		if value != (optionsTestValue{N: 99}) {
			t.Fatalf("default resetFn mutated value-typed input: got %+v, want %+v", value, optionsTestValue{N: 99})
		}

		if !resolved.reuseFn(optionsTestValue{N: -1}) {
			t.Fatal("default reuseFn returned false for value-typed input, want true")
		}

		// The default drop policy is intentionally just "do nothing and do not
		// panic"; there is no additional state to assert here.
		resolved.dropFn(optionsTestValue{N: 7})
	})

	t.Run("installs defaults for pointer type", func(t *testing.T) {
		expected := &optionsTestObject{count: 5, payload: []byte("abc")}
		resolved := Options[*optionsTestObject]{
			New: func() *optionsTestObject {
				return expected
			},
		}.resolve()

		if got := resolved.newFn(); got != expected {
			t.Fatalf("resolved newFn() pointer = %p, want %p", got, expected)
		}

		object := &optionsTestObject{count: 77, payload: []byte("payload")}
		beforeCount := object.count
		beforePayload := string(object.payload)
		// The default reset policy must remain a true no-op for pointer-backed T:
		// resolve() should install noopReset, not synthesize cleanup.
		resolved.resetFn(object)
		if object.count != beforeCount || string(object.payload) != beforePayload {
			t.Fatalf(
				"default resetFn mutated pointer-backed input: got count=%d payload=%q, want count=%d payload=%q",
				object.count,
				string(object.payload),
				beforeCount,
				beforePayload,
			)
		}

		if !resolved.reuseFn(object) {
			t.Fatal("default reuseFn returned false for pointer-typed input, want true")
		}

		resolved.dropFn(object)
	})

	t.Run("preserves custom hooks", func(t *testing.T) {
		calls := optionsHookCalls{}
		marker := &optionsTestObject{count: 10, payload: []byte("marker")}

		resolved := Options[*optionsTestObject]{
			New: func() *optionsTestObject {
				calls.new++
				return marker
			},
			Reset: func(v *optionsTestObject) {
				calls.reset++
				v.count = 0
				v.payload = v.payload[:0]
				v.resetSeen = true
			},
			Reuse: func(v *optionsTestObject) bool {
				calls.reuse++
				return v.count <= 64
			},
			OnDrop: func(v *optionsTestObject) {
				calls.drop++
				v.dropSeen = true
			},
		}.resolve()

		if got := resolved.newFn(); got != marker {
			t.Fatalf("resolved newFn() pointer = %p, want %p", got, marker)
		}
		if calls.new != 1 {
			t.Fatalf("custom newFn call count = %d, want 1", calls.new)
		}

		reusable := &optionsTestObject{count: 32, payload: []byte("retain")}
		// Accepted objects must flow through Reuse and then Reset, while OnDrop
		// must remain untouched on the retained path.
		if !resolved.reuseFn(reusable) {
			t.Fatal("custom reuseFn rejected reusable object, want true")
		}
		resolved.resetFn(reusable)

		if calls.reuse != 1 {
			t.Fatalf("custom reuseFn call count after accepted path = %d, want 1", calls.reuse)
		}
		if calls.reset != 1 {
			t.Fatalf("custom resetFn call count after accepted path = %d, want 1", calls.reset)
		}
		if reusable.count != 0 {
			t.Fatalf("custom resetFn left count = %d, want 0", reusable.count)
		}
		if len(reusable.payload) != 0 {
			t.Fatalf("custom resetFn left payload length = %d, want 0", len(reusable.payload))
		}
		if !reusable.resetSeen {
			t.Fatal("custom resetFn did not mark resetSeen")
		}
		if reusable.dropSeen {
			t.Fatal("drop callback ran on accepted object, want it to remain untouched")
		}

		dropped := &optionsTestObject{count: 128, payload: []byte("drop")}
		// Rejected objects follow the opposite path: Reuse returns false and the
		// resolved drop callback remains callable with the original semantics.
		if resolved.reuseFn(dropped) {
			t.Fatal("custom reuseFn accepted dropped object, want false")
		}
		resolved.dropFn(dropped)

		if calls.reuse != 2 {
			t.Fatalf("custom reuseFn call count after dropped path = %d, want 2", calls.reuse)
		}
		if calls.drop != 1 {
			t.Fatalf("custom dropFn call count = %d, want 1", calls.drop)
		}
		if !dropped.dropSeen {
			t.Fatal("custom dropFn did not mark dropSeen")
		}
	})

	t.Run("allows mixing custom and default hooks", func(t *testing.T) {
		resetCalls := 0
		reuseCalls := 0

		resolved := Options[*optionsTestObject]{
			New: func() *optionsTestObject {
				return &optionsTestObject{}
			},
			Reset: func(v *optionsTestObject) {
				resetCalls++
				v.count = 0
			},
			Reuse: func(v *optionsTestObject) bool {
				reuseCalls++
				return v.count == 0
			},
		}.resolve()

		object := &optionsTestObject{count: 0}
		if !resolved.reuseFn(object) {
			t.Fatal("custom reuseFn rejected accepted object, want true")
		}
		resolved.resetFn(object)

		if object.count != 0 {
			t.Fatalf("custom resetFn left count = %d, want 0", object.count)
		}
		if resetCalls != 1 {
			t.Fatalf("custom resetFn call count = %d, want 1", resetCalls)
		}
		if reuseCalls != 1 {
			t.Fatalf("custom reuseFn call count = %d, want 1", reuseCalls)
		}

		// OnDrop was left nil on purpose. The resolved drop hook should therefore
		// remain the default no-op and be safely callable here.
		resolved.dropFn(&optionsTestObject{count: 99})
	})
}

func TestDefaultPolicies(t *testing.T) {
	// These tests pin the standalone helper contracts directly so regressions in
	// resolve() can be separated from regressions in the default policy helpers.
	t.Run("noopReset leaves object unchanged", func(t *testing.T) {
		object := &optionsTestObject{count: 11, payload: []byte("keep")}
		beforeCount := object.count
		beforePayload := string(object.payload)

		noopReset(object)

		if object.count != beforeCount || string(object.payload) != beforePayload {
			t.Fatalf(
				"noopReset mutated object: got count=%d payload=%q, want count=%d payload=%q",
				object.count,
				string(object.payload),
				beforeCount,
				beforePayload,
			)
		}
	})

	t.Run("alwaysReuse returns true for value and pointer inputs", func(t *testing.T) {
		if !alwaysReuse(0) {
			t.Fatal("alwaysReuse(0) = false, want true")
		}
		if !alwaysReuse(&optionsTestObject{count: 1}) {
			t.Fatal("alwaysReuse(pointer) = false, want true")
		}
	})

	t.Run("noopDrop accepts pointer and value inputs", func(t *testing.T) {
		// noopDrop has no observable state change; the contract is simply that it
		// accepts any T without panicking.
		noopDrop(&optionsTestObject{count: 1})
		noopDrop(optionsTestValue{N: 2})
	})
}

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

package backend

import (
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

// syncPoolTestObject is intentionally compact.
//
// The backend contract under test is about type preservation, constructor
// behaviour, and reuse eligibility rather than stable same-instance retention.
// A small object keeps the tests
// focused on those invariants instead of unrelated domain behaviour.
type syncPoolTestObject struct {
	ID int
}

// syncPoolOtherObject exists solely to violate the backend invariant in a
// controlled way. Using a second named type makes mismatch tests easier to read
// than relying only on built-in scalar types.
type syncPoolOtherObject struct {
	ID int
}

func TestNewSyncPool(t *testing.T) {
	t.Run("panics when constructor is nil", func(t *testing.T) {
		testutil.AssertPanicMessage(
			t,
			"NewSyncPool(nil)",
			func() {
				_ = NewSyncPool[any](nil)
			},
			"pool: newFn must not be nil",
		)
	})

	t.Run("Get uses constructor on backend miss", func(t *testing.T) {
		calls := 0
		pool := NewSyncPool(func() syncPoolTestObject {
			calls++
			return syncPoolTestObject{ID: calls}
		})

		got1 := pool.Get()
		got2 := pool.Get()

		if got1 != (syncPoolTestObject{ID: 1}) {
			t.Fatalf("first Get() result = %+v, want %+v", got1, syncPoolTestObject{ID: 1})
		}
		if got2 != (syncPoolTestObject{ID: 2}) {
			t.Fatalf("second Get() result = %+v, want %+v", got2, syncPoolTestObject{ID: 2})
		}
		if calls != 2 {
			t.Fatalf("constructor call count after two backend misses = %d, want 2", calls)
		}
	})
}

func TestSyncPoolGetAfterPut(t *testing.T) {
	t.Run("pointer type", func(t *testing.T) {
		testutil.WithControlledSteadyStatePoolRoundTrip(t, func() {
			calls := 0
			pool := NewSyncPool(func() *syncPoolTestObject {
				calls++
				return &syncPoolTestObject{ID: calls}
			})

			stored := &syncPoolTestObject{ID: 42}
			pool.Put(stored)

			got := pool.Get()
			assertPointerGetAfterPut(t, got, stored, calls)
		})
	})

	t.Run("value type", func(t *testing.T) {
		testutil.WithControlledSteadyStatePoolRoundTrip(t, func() {
			calls := 0
			pool := NewSyncPool(func() syncPoolTestObject {
				calls++
				return syncPoolTestObject{ID: calls}
			})

			stored := syncPoolTestObject{ID: 42}
			pool.Put(stored)

			got := pool.Get()
			assertValueGetAfterPut(t, got, stored, calls)
		})
	})
}

func TestSyncPoolTypedNilHandling(t *testing.T) {
	t.Run("constructor may return typed nil pointer", func(t *testing.T) {
		pool := NewSyncPool(func() *syncPoolTestObject {
			return nil
		})

		got := pool.Get()
		if got != nil {
			t.Fatalf("Get() result with constructor returning typed nil = %v, want nil", got)
		}
	})

	t.Run("stored typed nil pointer does not break backend type safety", func(t *testing.T) {
		testutil.WithControlledSteadyStatePoolRoundTrip(t, func() {
			calls := 0
			pool := NewSyncPool(func() *syncPoolTestObject {
				calls++
				return &syncPoolTestObject{ID: calls}
			})

			var stored *syncPoolTestObject
			pool.Put(stored)

			got := pool.Get()
			switch {
			case got == nil:
				if calls != 0 {
					t.Fatalf("constructor call count after nil pointer round-trip = %d, want 0 when stored nil was reused", calls)
				}
			case got.ID == 1:
				if calls != 1 {
					t.Fatalf("constructor call count after nil pointer fallback = %d, want 1", calls)
				}
			default:
				t.Fatalf("Get() result after storing typed nil pointer = %+v, want nil or fresh constructor value", got)
			}
		})
	})
}

func TestSyncPoolNilReceiverPanics(t *testing.T) {
	t.Run("Get", func(t *testing.T) {
		var pool *SyncPool[int]

		testutil.AssertPanicMessage(
			t,
			"(*SyncPool[int])(nil).Get()",
			func() {
				_ = pool.Get()
			},
			"pool: Get called on nil SyncPool",
		)
	})

	t.Run("Put", func(t *testing.T) {
		var pool *SyncPool[int]

		testutil.AssertPanicMessage(
			t,
			"(*SyncPool[int])(nil).Put(1)",
			func() {
				pool.Put(1)
			},
			"pool: Put called on nil SyncPool",
		)
	})
}

func TestTypedPoolValue(t *testing.T) {
	t.Run("matching value type", func(t *testing.T) {
		got := typedPoolValue[syncPoolTestObject](syncPoolTestObject{ID: 42})
		if got != (syncPoolTestObject{ID: 42}) {
			t.Fatalf("typedPoolValue[syncPoolTestObject](...) = %+v, want %+v", got, syncPoolTestObject{ID: 42})
		}
	})

	t.Run("matching typed nil pointer", func(t *testing.T) {
		var want *syncPoolTestObject
		got := typedPoolValue[*syncPoolTestObject](want)
		if got != nil {
			t.Fatalf("typedPoolValue[*syncPoolTestObject](typed nil) = %v, want nil", got)
		}
	})
}

func TestTypedPoolValuePanicsOnUnexpectedType(t *testing.T) {
	t.Run("wrong non-nil dynamic type", func(t *testing.T) {
		testutil.AssertPanicMessage(
			t,
			"typedPoolValue[*syncPoolTestObject](1)",
			func() {
				_ = typedPoolValue[*syncPoolTestObject](1)
			},
			unexpectedTypePanic[*syncPoolTestObject](1),
		)
	})

	t.Run("wrong typed nil dynamic type", func(t *testing.T) {
		var wrong *syncPoolOtherObject
		testutil.AssertPanicMessage(
			t,
			"typedPoolValue[*syncPoolTestObject](wrong)",
			func() {
				_ = typedPoolValue[*syncPoolTestObject](wrong)
			},
			unexpectedTypePanic[*syncPoolTestObject](wrong),
		)
	})
}

func TestUnexpectedTypePanic(t *testing.T) {
	t.Run("value type expectation", func(t *testing.T) {
		got := unexpectedTypePanic[syncPoolTestObject](123)
		want := "pool: sync.Pool returned unexpected value of type int; expected backend.syncPoolTestObject"
		if got != want {
			t.Fatalf("unexpectedTypePanic[syncPoolTestObject](123) = %q, want %q", got, want)
		}
	})

	t.Run("pointer type expectation", func(t *testing.T) {
		got := unexpectedTypePanic[*syncPoolTestObject](123)
		want := "pool: sync.Pool returned unexpected value of type int; expected *backend.syncPoolTestObject"
		if got != want {
			t.Fatalf("unexpectedTypePanic[*syncPoolTestObject](123) = %q, want %q", got, want)
		}
	})
}

func assertPointerGetAfterPut(
	t *testing.T,
	got *syncPoolTestObject,
	stored *syncPoolTestObject,
	constructorCalls int,
) {
	t.Helper()

	switch {
	case got == stored:
		if constructorCalls != 0 {
			t.Fatalf("constructor call count after pointer reuse = %d, want 0", constructorCalls)
		}
	case got != nil && got.ID == 1:
		if constructorCalls != 1 {
			t.Fatalf("constructor call count after pointer fallback construction = %d, want 1", constructorCalls)
		}
	default:
		t.Fatalf("Get() after pointer Put() = %+v, want stored pointer or fresh constructor value", got)
	}
}

func assertValueGetAfterPut(
	t *testing.T,
	got syncPoolTestObject,
	stored syncPoolTestObject,
	constructorCalls int,
) {
	t.Helper()

	switch {
	case got == stored:
		if constructorCalls != 0 {
			t.Fatalf("constructor call count after value reuse = %d, want 0", constructorCalls)
		}
	case got == (syncPoolTestObject{ID: 1}):
		if constructorCalls != 1 {
			t.Fatalf("constructor call count after value fallback construction = %d, want 1", constructorCalls)
		}
	default:
		t.Fatalf("Get() after value Put() = %+v, want stored value or fresh constructor value", got)
	}
}

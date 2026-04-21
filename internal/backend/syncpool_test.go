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
// behaviour, and round-trip reuse semantics. A small object keeps the tests
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

func TestSyncPoolGetPutRoundTrip(t *testing.T) {
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
			if got != stored {
				t.Fatalf("Get() returned pointer %p after Put(), want %p", got, stored)
			}
			if calls != 0 {
				t.Fatalf("constructor call count after pointer Put()/Get() round-trip = %d, want 0", calls)
			}
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
			if got != stored {
				t.Fatalf("Get() returned value %+v after Put(), want %+v", got, stored)
			}
			if calls != 0 {
				t.Fatalf("constructor call count after value Put()/Get() round-trip = %d, want 0", calls)
			}
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

	t.Run("backend may round-trip typed nil pointer", func(t *testing.T) {
		testutil.WithControlledSteadyStatePoolRoundTrip(t, func() {
			pool := NewSyncPool(func() *syncPoolTestObject {
				t.Fatal("constructor must not be called when a typed nil pointer was stored explicitly")
				return &syncPoolTestObject{}
			})

			var stored *syncPoolTestObject
			pool.Put(stored)

			got := pool.Get()
			if got != nil {
				t.Fatalf("Get() result after storing typed nil pointer = %v, want nil", got)
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

func TestSyncPoolGetPanicsOnUnexpectedStoredType(t *testing.T) {
	t.Run("wrong non-nil dynamic type", func(t *testing.T) {
		pool := NewSyncPool(func() *syncPoolTestObject {
			return &syncPoolTestObject{}
		})

		// This intentionally bypasses the typed API to verify the internal
		// invariant: every value stored in the embedded sync.Pool must still
		// have dynamic type T when read back.
		pool.pool.Put(1)

		testutil.AssertPanicMessage(
			t,
			"Get() with scalar value of wrong type stored in embedded sync.Pool",
			func() {
				_ = pool.Get()
			},
			unexpectedTypePanic[*syncPoolTestObject](1),
		)
	})

	t.Run("wrong typed nil dynamic type", func(t *testing.T) {
		pool := NewSyncPool(func() *syncPoolTestObject {
			return &syncPoolTestObject{}
		})

		var wrong *syncPoolOtherObject
		pool.pool.Put(wrong)

		testutil.AssertPanicMessage(
			t,
			"Get() with typed nil pointer of wrong type stored in embedded sync.Pool",
			func() {
				_ = pool.Get()
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

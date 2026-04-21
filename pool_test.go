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
	"fmt"
	"sync"
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

// poolTestObject models the kind of pointer-backed temporary value the public
// Pool API is primarily meant to manage.
//
// Tests mutate its fields between Get and Put so they can verify that accepted
// objects are reset before reuse and rejected ones are dropped unchanged.
type poolTestObject struct {
	ID        int
	State     string
	Flag      bool
	Payload   []byte
	ResetSeen bool
	DropSeen  bool
}

// poolHookCalls tracks which public Options hooks were exercised by Pool.
//
// The Pool tests care about these counts because the main public contract is
// not just "what value comes back", but also "which lifecycle phases ran".
type poolHookCalls struct {
	new   int
	reset int
	reuse int
	drop  int
}

func TestNew(t *testing.T) {
	t.Run("panics when Options.New is nil", func(t *testing.T) {
		testutil.AssertPanicMessage(
			t,
			"New(Options[*poolTestObject]{})",
			func() {
				_ = New(Options[*poolTestObject]{})
			},
			"pool: Options.New must not be nil",
		)
	})
}

func TestPoolGet(t *testing.T) {
	t.Run("constructs on backend miss without running return-path hooks", func(t *testing.T) {
		calls := poolHookCalls{}
		pool := New(Options[*poolTestObject]{
			New: func() *poolTestObject {
				calls.new++
				return &poolTestObject{
					ID:      calls.new,
					Payload: make([]byte, 0, 16),
				}
			},
			Reset: func(*poolTestObject) {
				calls.reset++
			},
			Reuse: func(*poolTestObject) bool {
				calls.reuse++
				return true
			},
			OnDrop: func(*poolTestObject) {
				calls.drop++
			},
		})

		got := pool.Get()

		if got == nil {
			t.Fatal("Get() returned nil object on backend miss")
		}
		if got.ID != 1 {
			t.Fatalf("Get() object ID on backend miss = %d, want 1", got.ID)
		}
		if calls != (poolHookCalls{new: 1}) {
			t.Fatalf("hook calls after fresh Get() = %+v, want only new=1", calls)
		}
	})

	t.Run("panics on nil receiver", func(t *testing.T) {
		var pool *Pool[*poolTestObject]

		testutil.AssertPanicMessage(
			t,
			"(*Pool[*poolTestObject])(nil).Get()",
			func() {
				_ = pool.Get()
			},
			"pool: Get called on nil Pool",
		)
	})
}

func TestPoolPut(t *testing.T) {
	t.Run("accepted pointer values are reset before reuse", func(t *testing.T) {
		testutil.WithControlledSteadyStatePoolRoundTrip(t, func() {
			calls := poolHookCalls{}
			events := make([]string, 0, 2)
			pool := New(Options[*poolTestObject]{
				New: func() *poolTestObject {
					calls.new++
					return &poolTestObject{
						ID:      calls.new,
						State:   "fresh",
						Payload: make([]byte, 0, 64),
					}
				},
				Reset: func(v *poolTestObject) {
					calls.reset++
					events = append(events, "reset")
					v.State = "clean"
					v.Flag = false
					v.Payload = v.Payload[:0]
					v.ResetSeen = true
				},
				Reuse: func(v *poolTestObject) bool {
					calls.reuse++
					events = append(events, fmt.Sprintf("reuse:%s", v.State))
					return cap(v.Payload) <= 64
				},
				OnDrop: func(v *poolTestObject) {
					calls.drop++
					events = append(events, "drop")
					v.DropSeen = true
				},
			})

			first := pool.Get()
			first.State = "dirty"
			first.Flag = true
			first.Payload = append(first.Payload, []byte("dirty-state")...)

			pool.Put(first)

			second := pool.Get()

			testutil.AssertEventSequence(
				t,
				"accepted pointer Put() path",
				events,
				[]string{"reuse:dirty", "reset"},
			)
			if second != first {
				t.Fatalf("Get() after accepted Put() returned pointer %p, want original pointer %p", second, first)
			}
			if second.State != "clean" {
				t.Fatalf("reused object state = %q, want %q", second.State, "clean")
			}
			if second.Flag {
				t.Fatal("reused object retained dirty Flag, want false after reset")
			}
			if len(second.Payload) != 0 {
				t.Fatalf("reused object payload length = %d, want 0 after reset", len(second.Payload))
			}
			if !second.ResetSeen {
				t.Fatal("reused object does not show reset marker, want reset hook to have run")
			}
			if second.DropSeen {
				t.Fatal("accepted object was marked as dropped, want drop hook to remain untouched")
			}
			if calls != (poolHookCalls{new: 1, reset: 1, reuse: 1}) {
				t.Fatalf("hook calls after accepted pointer round-trip = %+v, want new=1 reset=1 reuse=1 drop=0", calls)
			}
		})
	})

	t.Run("rejected pointer values are dropped without reset or reuse", func(t *testing.T) {
		testutil.WithControlledSteadyStatePoolRoundTrip(t, func() {
			calls := poolHookCalls{}
			events := make([]string, 0, 2)
			pool := New(Options[*poolTestObject]{
				New: func() *poolTestObject {
					calls.new++
					return &poolTestObject{
						ID:      calls.new,
						State:   "fresh",
						Payload: make([]byte, 0, 8),
					}
				},
				Reset: func(v *poolTestObject) {
					calls.reset++
					events = append(events, "reset")
					v.State = "clean"
					v.Payload = v.Payload[:0]
				},
				Reuse: func(v *poolTestObject) bool {
					calls.reuse++
					events = append(events, fmt.Sprintf("reuse:%s", v.State))
					return cap(v.Payload) <= 4
				},
				OnDrop: func(v *poolTestObject) {
					calls.drop++
					events = append(events, fmt.Sprintf("drop:%s", v.State))
					v.DropSeen = true
				},
			})

			first := pool.Get()
			first.State = "oversized"
			first.Payload = append(first.Payload, []byte("01234567")...)

			pool.Put(first)

			second := pool.Get()

			testutil.AssertEventSequence(
				t,
				"rejected pointer Put() path",
				events,
				[]string{"reuse:oversized", "drop:oversized"},
			)
			if second == first {
				t.Fatalf("Get() reused rejected pointer %p, want newly constructed instance", second)
			}
			if second.ID != 2 {
				t.Fatalf("Get() after rejected Put() returned ID %d, want 2 from fresh construction", second.ID)
			}
			if first.State != "oversized" {
				t.Fatalf("rejected object state = %q, want %q (reset must not run)", first.State, "oversized")
			}
			if len(first.Payload) != 8 {
				t.Fatalf("rejected object payload length = %d, want 8 (reset must not run)", len(first.Payload))
			}
			if !first.DropSeen {
				t.Fatal("rejected object was not marked as dropped, want drop hook to run")
			}
			if calls != (poolHookCalls{new: 2, reuse: 1, drop: 1}) {
				t.Fatalf("hook calls after rejected pointer round-trip = %+v, want new=2 reset=0 reuse=1 drop=1", calls)
			}
		})
	})

	t.Run("value-typed pools follow the same reuse and drop contract", func(t *testing.T) {
		testutil.WithControlledSteadyStatePoolRoundTrip(t, func() {
			type value struct {
				ID    int
				Dirty bool
			}

			calls := poolHookCalls{}
			events := make([]string, 0, 4)
			pool := New(Options[value]{
				New: func() value {
					calls.new++
					return value{ID: calls.new}
				},
				Reset: func(v value) {
					calls.reset++
					events = append(events, fmt.Sprintf("reset:%d", v.ID))
					// Value-typed T is copied into ResetFunc. The public contract we
					// care about here is that reset still runs on accepted values and
					// drop still runs on rejected ones.
				},
				Reuse: func(v value) bool {
					calls.reuse++
					events = append(events, fmt.Sprintf("reuse:%d:%t", v.ID, v.Dirty))
					return !v.Dirty
				},
				OnDrop: func(v value) {
					calls.drop++
					events = append(events, fmt.Sprintf("drop:%d", v.ID))
				},
			})

			first := pool.Get()
			if first.ID != 1 {
				t.Fatalf("first Get() value ID = %d, want 1", first.ID)
			}
			pool.Put(first)

			second := pool.Get()
			if second.ID != 1 {
				t.Fatalf("Get() after accepted value Put() returned ID %d, want 1", second.ID)
			}

			second.Dirty = true
			pool.Put(second)

			third := pool.Get()

			testutil.AssertEventSequence(
				t,
				"value-typed Put() paths",
				events,
				[]string{"reuse:1:false", "reset:1", "reuse:1:true", "drop:1"},
			)
			if third.ID != 2 {
				t.Fatalf("Get() after rejected dirty value returned ID %d, want 2", third.ID)
			}
			if calls != (poolHookCalls{new: 2, reset: 1, reuse: 2, drop: 1}) {
				t.Fatalf("hook calls for value-typed paths = %+v, want new=2 reset=1 reuse=2 drop=1", calls)
			}
		})
	})

	t.Run("panics on nil receiver", func(t *testing.T) {
		var pool *Pool[*poolTestObject]

		testutil.AssertPanicMessage(
			t,
			"(*Pool[*poolTestObject])(nil).Put(nil)",
			func() {
				pool.Put(nil)
			},
			"pool: Put called on nil Pool",
		)
	})
}

func TestPoolConcurrentGetPut(t *testing.T) {
	pool := New(Options[*poolTestObject]{
		New: func() *poolTestObject {
			return &poolTestObject{
				State:   "fresh",
				Payload: make([]byte, 0, 64),
			}
		},
		Reset: func(v *poolTestObject) {
			v.State = "clean"
			v.Flag = false
			v.Payload = v.Payload[:0]
		},
		Reuse: func(v *poolTestObject) bool {
			return cap(v.Payload) <= 64
		},
	})

	const (
		goroutines = 16
		iterations = 1000
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				object := pool.Get()
				if object == nil {
					t.Error("Get() returned nil object during concurrent use")
					return
				}

				object.State = "dirty"
				object.Flag = true
				object.Payload = append(object.Payload, 'x')
				pool.Put(object)
			}
		}()
	}

	wg.Wait()

	object := pool.Get()
	if object == nil {
		t.Fatal("Get() returned nil object after concurrent use")
	}
	if object.Flag {
		t.Fatal("object after concurrent round-trip retained dirty Flag, want false")
	}
	if object.State != "fresh" && object.State != "clean" {
		t.Fatalf("object state after concurrent use = %q, want %q or %q", object.State, "fresh", "clean")
	}
	if len(object.Payload) != 0 {
		t.Fatalf("object payload length after concurrent use = %d, want 0", len(object.Payload))
	}
	pool.Put(object)
}

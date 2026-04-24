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

type poolTestValue struct {
	ID    int
	Dirty bool
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

func TestPoolPutAcceptedPointerValue(t *testing.T) {
	calls, events, pool := newPointerPutTestPool(64)
	value := &poolTestObject{
		ID:      42,
		State:   "dirty",
		Flag:    true,
		Payload: append(make([]byte, 0, 64), []byte("dirty-state")...),
	}

	pool.Put(value)

	testutil.AssertEventSequence(
		t,
		"accepted pointer Put() path",
		*events,
		[]string{"reuse:dirty", "reset"},
	)
	if value.State != "clean" {
		t.Fatalf("accepted pointer state after Put() = %q, want %q", value.State, "clean")
	}
	if value.Flag {
		t.Fatal("accepted pointer retained dirty Flag after Put(), want false")
	}
	if len(value.Payload) != 0 {
		t.Fatalf("accepted pointer payload length after Put() = %d, want 0", len(value.Payload))
	}
	if !value.ResetSeen {
		t.Fatal("accepted pointer does not show reset marker after Put()")
	}
	if value.DropSeen {
		t.Fatal("accepted pointer was marked as dropped, want drop hook to remain idle")
	}
	if *calls != (poolHookCalls{reset: 1, reuse: 1}) {
		t.Fatalf("hook calls for accepted pointer Put() = %+v, want reset=1 reuse=1", *calls)
	}
}

func TestPoolPutRejectedPointerValue(t *testing.T) {
	calls, events, pool := newPointerPutTestPool(4)
	value := &poolTestObject{
		ID:      42,
		State:   "oversized",
		Payload: append(make([]byte, 0, 8), []byte("01234567")...),
	}

	pool.Put(value)

	testutil.AssertEventSequence(
		t,
		"rejected pointer Put() path",
		*events,
		[]string{"reuse:oversized", "drop:oversized"},
	)
	if value.State != "oversized" {
		t.Fatalf("rejected pointer state after Put() = %q, want %q", value.State, "oversized")
	}
	if len(value.Payload) != 8 {
		t.Fatalf("rejected pointer payload length after Put() = %d, want 8", len(value.Payload))
	}
	if !value.DropSeen {
		t.Fatal("rejected pointer was not marked as dropped")
	}
	if value.ResetSeen {
		t.Fatal("rejected pointer was marked as reset, want reset hook to stay idle")
	}
	if *calls != (poolHookCalls{reuse: 1, drop: 1}) {
		t.Fatalf("hook calls for rejected pointer Put() = %+v, want reuse=1 drop=1", *calls)
	}

	got := pool.Get()
	if got.ID != 1 {
		t.Fatalf("Get() after rejected Put() returned ID %d, want 1 from fresh construction", got.ID)
	}
	if *calls != (poolHookCalls{new: 1, reuse: 1, drop: 1}) {
		t.Fatalf("hook calls after rejected pointer Put()/Get() = %+v, want new=1 reuse=1 drop=1", *calls)
	}
}

func TestPoolPutValueTypeHonorsReuseAndDropPolicy(t *testing.T) {
	calls := poolHookCalls{}
	events := make([]string, 0, 4)
	pool := New(Options[poolTestValue]{
		New: func() poolTestValue {
			calls.new++
			return poolTestValue{ID: calls.new}
		},
		Reset: func(v poolTestValue) {
			calls.reset++
			events = append(events, fmt.Sprintf("reset:%d", v.ID))
		},
		Reuse: func(v poolTestValue) bool {
			calls.reuse++
			events = append(events, fmt.Sprintf("reuse:%d:%t", v.ID, v.Dirty))
			return !v.Dirty
		},
		OnDrop: func(v poolTestValue) {
			calls.drop++
			events = append(events, fmt.Sprintf("drop:%d", v.ID))
		},
	})

	pool.Put(poolTestValue{ID: 41})
	pool.Put(poolTestValue{ID: 42, Dirty: true})

	testutil.AssertEventSequence(
		t,
		"value-typed Put() paths",
		events,
		[]string{"reuse:41:false", "reset:41", "reuse:42:true", "drop:42"},
	)
	if calls != (poolHookCalls{reset: 1, reuse: 2, drop: 1}) {
		t.Fatalf("hook calls for value-typed Put() paths = %+v, want reset=1 reuse=2 drop=1", calls)
	}
}

func TestPoolPutNilReceiverPanics(t *testing.T) {
	var pool *Pool[*poolTestObject]

	testutil.AssertPanicMessage(
		t,
		"(*Pool[*poolTestObject])(nil).Put(nil)",
		func() {
			pool.Put(nil)
		},
		"pool: Put called on nil Pool",
	)
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

	for range goroutines {
		go func() {
			defer wg.Done()

			for range iterations {
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

func newPointerPutTestPool(reusePayloadCap int) (*poolHookCalls, *[]string, *Pool[*poolTestObject]) {
	calls := &poolHookCalls{}
	events := &[]string{}
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
			*events = append(*events, "reset")
			v.State = "clean"
			v.Flag = false
			v.Payload = v.Payload[:0]
			v.ResetSeen = true
		},
		Reuse: func(v *poolTestObject) bool {
			calls.reuse++
			*events = append(*events, fmt.Sprintf("reuse:%s", v.State))
			return cap(v.Payload) <= reusePayloadCap
		},
		OnDrop: func(v *poolTestObject) {
			calls.drop++
			*events = append(*events, fmt.Sprintf("drop:%s", v.State))
			v.DropSeen = true
		},
	})
	return calls, events, pool
}

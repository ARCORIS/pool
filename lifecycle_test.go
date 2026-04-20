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
	"slices"
	"testing"
)

// lifecycleTestObject is intentionally mutable.
//
// The lifecycle tests need to observe whether Release resets accepted objects,
// leaves rejected objects untouched, and forwards the expected state into the
// final sink Put call.
type lifecycleTestObject struct {
	ID     int
	State  string
	Buffer []byte
}

// recordingSink captures values that made it all the way through lifecycle
// release semantics into backend storage.
//
// Tests also optionally attach an event log so the final Put step can be
// asserted alongside reuse, reset, and drop callbacks.
type recordingSink[T any] struct {
	events *[]string
	puts   []T
}

func (s *recordingSink[T]) Put(value T) {
	if s.events != nil {
		*s.events = append(*s.events, "put")
	}
	s.puts = append(s.puts, value)
}

func TestNewLifecycle(t *testing.T) {
	t.Run("wires resolved hooks into the semantic controller", func(t *testing.T) {
		events := make([]string, 0, 3)
		object := &lifecycleTestObject{ID: 7, State: "dirty", Buffer: []byte("payload")}

		// newLifecycle is fed resolvedOptions directly because Options.resolve is
		// the layer that already guarantees non-nil hook normalization.
		lifecycle := newLifecycle(resolvedOptions[*lifecycleTestObject]{
			resetFn: func(v *lifecycleTestObject) {
				events = append(events, "reset")
				v.State = "clean"
				v.Buffer = v.Buffer[:0]
			},
			reuseFn: func(v *lifecycleTestObject) bool {
				events = append(events, "reuse")
				return v.ID == 7
			},
			dropFn: func(*lifecycleTestObject) {
				events = append(events, "drop")
			},
		})

		if got := lifecycle.AllowReuse(object); !got {
			t.Fatal("AllowReuse() = false, want true")
		}
		lifecycle.ResetForReuse(object)
		lifecycle.ObserveDrop(object)

		assertEventSequence(t, "newLifecycle hook wiring", events, []string{"reuse", "reset", "drop"})
		if object.State != "clean" {
			t.Fatalf("object state after ResetForReuse() = %q, want %q", object.State, "clean")
		}
		if len(object.Buffer) != 0 {
			t.Fatalf("buffer length after ResetForReuse() = %d, want 0", len(object.Buffer))
		}
	})
}

func TestLifecycleAllowReuse(t *testing.T) {
	t.Run("returns the configured admission decision", func(t *testing.T) {
		calls := 0
		lifecycle := lifecycle[int]{
			reuse: func(value int) bool {
				calls++
				return value%2 == 0
			},
			reset:  noopReset[int],
			onDrop: noopDrop[int],
		}

		if got := lifecycle.AllowReuse(2); !got {
			t.Fatal("AllowReuse(2) = false, want true")
		}
		if got := lifecycle.AllowReuse(3); got {
			t.Fatal("AllowReuse(3) = true, want false")
		}
		if calls != 2 {
			t.Fatalf("reuse hook call count = %d, want 2", calls)
		}
	})
}

func TestLifecycleResetForReuse(t *testing.T) {
	t.Run("delegates only to the configured reset hook", func(t *testing.T) {
		events := make([]string, 0, 1)
		object := &lifecycleTestObject{State: "dirty", Buffer: []byte("payload")}
		lifecycle := lifecycle[*lifecycleTestObject]{
			reset: func(v *lifecycleTestObject) {
				events = append(events, "reset")
				v.State = "clean"
				v.Buffer = v.Buffer[:0]
			},
			reuse: func(*lifecycleTestObject) bool {
				t.Fatal("reuse hook must not be called by ResetForReuse")
				return true
			},
			onDrop: func(*lifecycleTestObject) {
				t.Fatal("drop hook must not be called by ResetForReuse")
			},
		}

		lifecycle.ResetForReuse(object)

		assertEventSequence(t, "ResetForReuse()", events, []string{"reset"})
		if object.State != "clean" {
			t.Fatalf("object state after ResetForReuse() = %q, want %q", object.State, "clean")
		}
		if len(object.Buffer) != 0 {
			t.Fatalf("buffer length after ResetForReuse() = %d, want 0", len(object.Buffer))
		}
	})
}

func TestLifecycleObserveDrop(t *testing.T) {
	t.Run("delegates only to the configured drop hook", func(t *testing.T) {
		events := make([]string, 0, 1)
		object := &lifecycleTestObject{ID: 11}
		lifecycle := lifecycle[*lifecycleTestObject]{
			reset: func(*lifecycleTestObject) {
				t.Fatal("reset hook must not be called by ObserveDrop")
			},
			reuse: func(*lifecycleTestObject) bool {
				t.Fatal("reuse hook must not be called by ObserveDrop")
				return false
			},
			onDrop: func(v *lifecycleTestObject) {
				events = append(events, fmt.Sprintf("drop:%d", v.ID))
			},
		}

		lifecycle.ObserveDrop(object)

		assertEventSequence(t, "ObserveDrop()", events, []string{"drop:11"})
	})
}

func TestLifecycleRelease(t *testing.T) {
	t.Run("panics when sink is nil", func(t *testing.T) {
		lifecycle := lifecycle[int]{
			reset:  noopReset[int],
			reuse:  alwaysReuse[int],
			onDrop: noopDrop[int],
		}

		assertPanicMessage(
			t,
			"Release(nil, 1)",
			func() {
				lifecycle.Release(nil, 1)
			},
			"pool: nil lifecycle sink",
		)
	})

	t.Run("denied values are dropped without reset or storage", func(t *testing.T) {
		events := make([]string, 0, 2)
		sink := &recordingSink[*lifecycleTestObject]{events: &events}
		object := &lifecycleTestObject{ID: 1, State: "oversized", Buffer: make([]byte, 0, 128<<10)}

		lifecycle := lifecycle[*lifecycleTestObject]{
			reuse: func(v *lifecycleTestObject) bool {
				events = append(events, fmt.Sprintf("reuse:%s", v.State))
				return false
			},
			reset: func(v *lifecycleTestObject) {
				events = append(events, "reset")
				v.State = "clean"
				v.Buffer = v.Buffer[:0]
			},
			onDrop: func(v *lifecycleTestObject) {
				events = append(events, fmt.Sprintf("drop:%s", v.State))
			},
		}

		lifecycle.Release(sink, object)

		// Admission must observe the pre-reset state. Once reuse is denied,
		// Release must drop and stop without cleaning or storing the object.
		assertEventSequence(
			t,
			"Release() denied path",
			events,
			[]string{"reuse:oversized", "drop:oversized"},
		)
		if len(sink.puts) != 0 {
			t.Fatalf("sink put count after denied Release() = %d, want 0", len(sink.puts))
		}
		if object.State != "oversized" {
			t.Fatalf("object state after denied Release() = %q, want %q", object.State, "oversized")
		}
		if len(object.Buffer) != 0 {
			t.Fatalf("buffer length after denied Release() = %d, want 0 (reset must not run)", len(object.Buffer))
		}
	})

	t.Run("accepted values are reset before storage and are not dropped", func(t *testing.T) {
		events := make([]string, 0, 3)
		sink := &recordingSink[*lifecycleTestObject]{events: &events}
		object := &lifecycleTestObject{ID: 2, State: "dirty", Buffer: []byte("payload")}

		lifecycle := lifecycle[*lifecycleTestObject]{
			reuse: func(v *lifecycleTestObject) bool {
				events = append(events, fmt.Sprintf("reuse:%s", v.State))
				return true
			},
			reset: func(v *lifecycleTestObject) {
				events = append(events, "reset")
				v.State = "clean"
				v.Buffer = v.Buffer[:0]
			},
			onDrop: func(*lifecycleTestObject) {
				events = append(events, "drop")
			},
		}

		lifecycle.Release(sink, object)

		assertEventSequence(
			t,
			"Release() accepted pointer path",
			events,
			[]string{"reuse:dirty", "reset", "put"},
		)
		if len(sink.puts) != 1 {
			t.Fatalf("sink put count after accepted Release() = %d, want 1", len(sink.puts))
		}
		if sink.puts[0] != object {
			t.Fatalf("sink stored pointer %p, want original pointer %p", sink.puts[0], object)
		}
		if object.State != "clean" {
			t.Fatalf("object state after accepted Release() = %q, want %q", object.State, "clean")
		}
		if len(object.Buffer) != 0 {
			t.Fatalf("buffer length after accepted Release() = %d, want 0", len(object.Buffer))
		}
	})

	t.Run("accepted values preserve order for value types", func(t *testing.T) {
		type value struct {
			State string
		}

		events := make([]string, 0, 3)
		sink := &recordingSink[value]{events: &events}
		lifecycle := lifecycle[value]{
			reuse: func(v value) bool {
				events = append(events, fmt.Sprintf("reuse:%s", v.State))
				return true
			},
			reset: func(v value) {
				events = append(events, fmt.Sprintf("reset:%s", v.State))
				// Value-typed T is copied into ResetFunc and Put. This test
				// documents that lifecycle still preserves semantic order even
				// when reset cannot mutate the caller-visible object in place.
			},
			onDrop: func(value) {
				events = append(events, "drop")
			},
		}

		lifecycle.Release(sink, value{State: "dirty"})

		assertEventSequence(
			t,
			"Release() accepted value path",
			events,
			[]string{"reuse:dirty", "reset:dirty", "put"},
		)
		if len(sink.puts) != 1 {
			t.Fatalf("sink put count for value-typed Release() = %d, want 1", len(sink.puts))
		}
		if sink.puts[0].State != "dirty" {
			t.Fatalf("stored value state = %q, want %q", sink.puts[0].State, "dirty")
		}
	})
}

func assertEventSequence(t *testing.T, scenario string, got []string, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("%s event sequence = %v, want %v", scenario, got, want)
	}
}

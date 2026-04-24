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
	"bytes"
	"fmt"
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

type fuzzPoolObject struct {
	State     string
	Payload   []byte
	ResetSeen bool
	DropSeen  bool
}

func FuzzPoolLifecycleOrder(f *testing.F) {
	f.Add(true, []byte("payload"))
	f.Add(false, []byte("oversized"))
	f.Add(true, []byte{})

	f.Fuzz(func(t *testing.T, allowReuse bool, payload []byte) {
		payload = fuzzTrimBytes(payload, 32)

		events := make([]string, 0, 3)
		resetCalls := 0
		dropCalls := 0
		reuseCalls := 0
		sink := &testutil.RecordingSink[*fuzzPoolObject]{Events: &events}
		value := &fuzzPoolObject{
			State:   "dirty",
			Payload: append([]byte(nil), payload...),
		}

		lifecycle := lifecycle[*fuzzPoolObject]{
			reuse: func(v *fuzzPoolObject) bool {
				reuseCalls++
				events = append(events, fmt.Sprintf("reuse:%d", len(v.Payload)))
				return allowReuse
			},
			reset: func(v *fuzzPoolObject) {
				resetCalls++
				events = append(events, "reset")
				v.State = "clean"
				v.Payload = v.Payload[:0]
				v.ResetSeen = true
			},
			onDrop: func(v *fuzzPoolObject) {
				dropCalls++
				events = append(events, "drop")
				v.DropSeen = true
			},
		}

		lifecycle.Release(sink, value)

		if reuseCalls != 1 {
			t.Fatalf("reuse call count = %d, want 1", reuseCalls)
		}

		if allowReuse {
			testutil.AssertEventSequence(
				t,
				"accepted fuzz lifecycle order",
				events,
				[]string{fmt.Sprintf("reuse:%d", len(payload)), "reset", "put"},
			)
			if resetCalls != 1 {
				t.Fatalf("reset call count on accepted path = %d, want 1", resetCalls)
			}
			if dropCalls != 0 {
				t.Fatalf("drop call count on accepted path = %d, want 0", dropCalls)
			}
			if len(sink.Puts) != 1 {
				t.Fatalf("sink put count on accepted path = %d, want 1", len(sink.Puts))
			}
		} else {
			testutil.AssertEventSequence(
				t,
				"rejected fuzz lifecycle order",
				events,
				[]string{fmt.Sprintf("reuse:%d", len(payload)), "drop"},
			)
			if resetCalls != 0 {
				t.Fatalf("reset call count on rejected path = %d, want 0", resetCalls)
			}
			if dropCalls != 1 {
				t.Fatalf("drop call count on rejected path = %d, want 1", dropCalls)
			}
			if len(sink.Puts) != 0 {
				t.Fatalf("sink put count on rejected path = %d, want 0", len(sink.Puts))
			}
		}
	})
}

func FuzzPoolAcceptedValueIsResetBeforeReuse(f *testing.F) {
	f.Add([]byte("dirty"))
	f.Add([]byte("with-more-bytes"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, payload []byte) {
		payload = fuzzTrimBytes(payload, 32)

		resetCalls := 0
		newCalls := 0
		pool := New(Options[*fuzzPoolObject]{
			New: func() *fuzzPoolObject {
				newCalls++
				return &fuzzPoolObject{
					State:   "fresh",
					Payload: make([]byte, 0, 32),
				}
			},
			Reset: func(v *fuzzPoolObject) {
				resetCalls++
				v.State = "clean"
				v.Payload = v.Payload[:0]
				v.ResetSeen = true
			},
			Reuse: func(*fuzzPoolObject) bool {
				return true
			},
		})

		value := &fuzzPoolObject{
			State:   "dirty",
			Payload: append([]byte(nil), payload...),
		}

		pool.Put(value)

		if resetCalls != 1 {
			t.Fatalf("reset call count after accepted Put() = %d, want 1", resetCalls)
		}
		if value.State != "clean" {
			t.Fatalf("accepted value state after Put() = %q, want %q", value.State, "clean")
		}
		if len(value.Payload) != 0 {
			t.Fatalf("accepted value payload length after Put() = %d, want 0", len(value.Payload))
		}
		if !value.ResetSeen {
			t.Fatal("accepted value does not show reset marker after Put()")
		}

		got := pool.Get()
		if got == value {
			if got.State != "clean" {
				t.Fatalf("reused value state = %q, want %q", got.State, "clean")
			}
			if len(got.Payload) != 0 {
				t.Fatalf("reused value payload length = %d, want 0", len(got.Payload))
			}
		} else {
			if newCalls != 1 {
				t.Fatalf("new call count after fallback Get() = %d, want 1", newCalls)
			}
		}
		pool.Put(got)
	})
}

func FuzzPoolRejectedValueIsNotStored(f *testing.F) {
	f.Add([]byte("oversized"))
	f.Add([]byte("abc"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, payload []byte) {
		payload = fuzzTrimBytes(payload, 32)

		newCalls := 0
		resetCalls := 0
		dropCalls := 0
		pool := New(Options[*fuzzPoolObject]{
			New: func() *fuzzPoolObject {
				newCalls++
				return &fuzzPoolObject{
					State:   "fresh",
					Payload: make([]byte, 0, 32),
				}
			},
			Reset: func(v *fuzzPoolObject) {
				resetCalls++
				v.State = "clean"
				v.Payload = v.Payload[:0]
				v.ResetSeen = true
			},
			Reuse: func(*fuzzPoolObject) bool {
				return false
			},
			OnDrop: func(v *fuzzPoolObject) {
				dropCalls++
				v.DropSeen = true
			},
		})

		value := &fuzzPoolObject{
			State:   "dirty",
			Payload: append([]byte(nil), payload...),
		}

		pool.Put(value)

		if resetCalls != 0 {
			t.Fatalf("reset call count after rejected Put() = %d, want 0", resetCalls)
		}
		if dropCalls != 1 {
			t.Fatalf("drop call count after rejected Put() = %d, want 1", dropCalls)
		}
		if !value.DropSeen {
			t.Fatal("rejected value does not show drop marker after Put()")
		}
		if value.ResetSeen {
			t.Fatal("rejected value was reset even though reuse was denied")
		}
		if !bytes.Equal(value.Payload, payload) {
			t.Fatalf("rejected value payload changed from %q to %q", payload, value.Payload)
		}

		got := pool.Get()
		if got == value {
			t.Fatal("rejected value was returned by Get(), want a fresh allocation")
		}
		if newCalls != 1 {
			t.Fatalf("new call count after rejected Put()/Get() = %d, want 1", newCalls)
		}
		pool.Put(got)
	})
}

func fuzzTrimBytes(payload []byte, limit int) []byte {
	if len(payload) > limit {
		payload = payload[:limit]
	}
	return append([]byte(nil), payload...)
}

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

package testutil

import "testing"

func TestRecordingSinkPut(t *testing.T) {
	t.Run("records event and stored values", func(t *testing.T) {
		events := []string{}
		sink := &RecordingSink[int]{Events: &events}

		// The sink is used in lifecycle tests that assert both the final stored
		// values and the fact that the backend put step happened at all.
		sink.Put(1)
		sink.Put(2)

		AssertEventSequence(t, "RecordingSink event log", events, []string{"put", "put"})
		if len(sink.Puts) != 2 || sink.Puts[0] != 1 || sink.Puts[1] != 2 {
			t.Fatalf("RecordingSink stored values = %v, want [1 2]", sink.Puts)
		}
	})

	t.Run("stores values without event log", func(t *testing.T) {
		sink := &RecordingSink[string]{}

		sink.Put("a")

		if len(sink.Puts) != 1 || sink.Puts[0] != "a" {
			t.Fatalf("RecordingSink stored values without event log = %v, want [a]", sink.Puts)
		}
	})
}

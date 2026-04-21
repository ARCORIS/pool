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

func TestMustPanic(t *testing.T) {
	t.Run("returns panic message", func(t *testing.T) {
		// The helper is used to keep panic-based contract tests concise. This
		// case proves it preserves the recovered message instead of only checking
		// that a panic happened.
		got := MustPanic(t, "panic case", func() {
			panic("boom")
		})

		if got != "boom" {
			t.Fatalf("MustPanic returned %q, want %q", got, "boom")
		}
	})
}

func TestAssertPanicMessage(t *testing.T) {
	t.Run("accepts exact message", func(t *testing.T) {
		AssertPanicMessage(t, "exact match", func() {
			panic("boom")
		}, "boom")
	})
}

func TestAssertEventSequence(t *testing.T) {
	t.Run("accepts equal slices", func(t *testing.T) {
		// Lifecycle tests care about strict event order, not just set equality.
		// A simple equal-slice case keeps that contract explicit.
		AssertEventSequence(t, "equal sequence", []string{"a", "b"}, []string{"a", "b"})
	})
}

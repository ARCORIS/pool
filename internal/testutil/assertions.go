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

// Package testutil contains shared helpers for repository-local tests and
// benchmarks.
//
// The package intentionally lives under internal/ so helpers can be reused
// across arcoris.dev/pool test packages without becoming part of the public
// module API.
package testutil

import (
	"fmt"
	"slices"
	"testing"
)

// AssertPanicMessage verifies that fn panics and that the recovered panic value
// stringifies to the exact expected message.
//
// The helper is used when panic text is part of the contract under test. It
// delegates panic capture to MustPanic so callers can keep panic assertions
// short while still producing a scenario-specific diagnostic on failure.
func AssertPanicMessage(tb testing.TB, scenario string, fn func(), want string) {
	tb.Helper()

	got := MustPanic(tb, scenario, fn)
	if got != want {
		tb.Fatalf("%s panic message = %q, want %q", scenario, got, want)
	}
}

// MustPanic runs fn and returns the recovered panic value formatted with
// fmt.Sprint.
//
// If fn does not panic, the helper fails the test immediately. This keeps
// panic-based contract checks concise while preserving the scenario name in the
// failure output.
func MustPanic(tb testing.TB, scenario string, fn func()) string {
	tb.Helper()

	var panicValue any

	func() {
		defer func() {
			panicValue = recover()
		}()
		fn()
	}()

	if panicValue == nil {
		tb.Fatalf("%s: expected panic, got none", scenario)
	}

	return fmt.Sprint(panicValue)
}

// AssertEventSequence verifies that an observed event log matches the expected
// sequence exactly.
//
// Lifecycle-oriented tests use this helper to keep ordering assertions concise
// and to produce a stable diagnostic message when a semantic step moves or is
// omitted.
func AssertEventSequence(tb testing.TB, scenario string, got []string, want []string) {
	tb.Helper()

	if !slices.Equal(got, want) {
		tb.Fatalf("%s event sequence = %v, want %v", scenario, got, want)
	}
}

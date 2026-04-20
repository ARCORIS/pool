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

// Package testutil contains shared helpers for package-local tests and
// benchmarks.
//
// The package intentionally lives under internal/ so it can be reused across
// arcoris.dev/pool test packages without becoming part of the public module
// API.
package testutil

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"testing"
)

// RecordingSink is a generic Put recorder for tests that need to observe the
// final storage step of a value.
//
// When Events is non-nil, Put appends a literal "put" marker to that event
// log before storing the value in Puts.
type RecordingSink[T any] struct {
	Events *[]string
	Puts   []T
}

func (s *RecordingSink[T]) Put(value T) {
	if s.Events != nil {
		*s.Events = append(*s.Events, "put")
	}
	s.Puts = append(s.Puts, value)
}

func AssertPanicMessage(tb testing.TB, scenario string, fn func(), want string) {
	tb.Helper()

	got := MustPanic(tb, scenario, fn)
	if got != want {
		tb.Fatalf("%s panic message = %q, want %q", scenario, got, want)
	}
}

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

func AssertEventSequence(tb testing.TB, scenario string, got []string, want []string) {
	tb.Helper()

	if !slices.Equal(got, want) {
		tb.Fatalf("%s event sequence = %v, want %v", scenario, got, want)
	}
}

func WithSingleP(tb testing.TB, fn func()) {
	tb.Helper()

	previous := runtime.GOMAXPROCS(1)
	tb.Cleanup(func() {
		runtime.GOMAXPROCS(previous)
	})

	fn()
}

func WithGCDisabled(tb testing.TB, fn func()) {
	tb.Helper()

	previous := debug.SetGCPercent(-1)
	tb.Cleanup(func() {
		debug.SetGCPercent(previous)
	})

	fn()
}

// WithStablePoolRoundTrip makes immediate Put/Get assertions deterministic
// enough for tests and benchmarks that rely on sync.Pool local-cache reuse.
//
// Pinning execution to one P avoids per-P handoff surprises, and disabling GC
// prevents cached values from being discarded between Put and Get.
func WithStablePoolRoundTrip(tb testing.TB, fn func()) {
	tb.Helper()

	WithGCDisabled(tb, func() {
		WithSingleP(tb, fn)
	})
}

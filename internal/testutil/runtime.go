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

import (
	"runtime"
	"runtime/debug"
	"testing"
)

// WithSingleP runs fn with GOMAXPROCS forced to 1 and restores the previous
// setting before returning.
//
// The helper is intended for tests and benchmarks that need deterministic
// single-P behaviour, especially around sync.Pool local-cache semantics. The
// restoration is scoped to the callback itself rather than deferred to test
// cleanup so later assertions in the same test see the original runtime state.
func WithSingleP(tb testing.TB, fn func()) {
	tb.Helper()

	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)

	fn()
}

// WithGCDisabled runs fn with automatic GC disabled and restores the previous
// GC target before returning.
//
// This helper is useful when a test or benchmark needs to prevent transient GC
// cycles from discarding sync.Pool state between tightly-coupled Put/Get steps.
// As with WithSingleP, restoration happens immediately after fn returns so the
// helper composes safely inside larger tests.
func WithGCDisabled(tb testing.TB, fn func()) {
	tb.Helper()

	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)

	fn()
}

// WithControlledSteadyStatePoolRoundTrip makes immediate Put/Get assertions
// deterministic enough for controlled tests and benchmarks that rely on
// sync.Pool local-cache reuse.
//
// Pinning execution to one P avoids per-P handoff surprises, and disabling GC
// prevents cached values from being discarded between Put and Get. The helper
// restores both runtime settings before it returns to the caller.
//
// This helper intentionally creates an idealized local steady-state execution
// mode. It is appropriate for controlled hot path measurements and for unit
// tests that need deterministic immediate reuse. It MUST NOT be described as a
// general runtime mode in performance reports.
func WithControlledSteadyStatePoolRoundTrip(tb testing.TB, fn func()) {
	tb.Helper()

	WithGCDisabled(tb, func() {
		WithSingleP(tb, fn)
	})
}

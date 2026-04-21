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

import "runtime"

const parallelWarmFactor = 16

// PrimePoolValue executes one Get/Put-style round trip before a benchmark loop
// starts.
//
// Serial steady-state benchmarks use this helper to ensure the timed loop
// measures the reuse path rather than the first constructor miss.
func PrimePoolValue[T any](get func() T, put func(T)) {
	value := get()
	put(value)
}

// PrefillPool stores count freshly constructed values into a pool-like sink.
//
// Parallel benchmarks use this helper to avoid measuring an entirely cold pool
// when the question is concurrent steady-state behaviour.
func PrefillPool[T any](count int, newFn func() T, put func(T)) {
	for i := 0; i < count; i++ {
		put(newFn())
	}
}

// ParallelWarmCount returns the default repository warm-up size for parallel
// pool benchmarks.
//
// The value scales with the current GOMAXPROCS setting so the prefill step is
// proportional to the number of active Ps that may participate in sync.Pool
// local-cache distribution.
func ParallelWarmCount() int {
	return runtime.GOMAXPROCS(0) * parallelWarmFactor
}

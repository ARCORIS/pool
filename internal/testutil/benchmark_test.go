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
	"slices"
	"testing"
)

func TestPrimePoolValue(t *testing.T) {
	t.Run("executes one get put round trip", func(t *testing.T) {
		getCalls := 0
		gotPut := 0

		// The helper is intentionally tiny, but benchmark stability depends on
		// it doing exactly one acquisition and one return before timing starts.
		PrimePoolValue(func() int {
			getCalls++
			return 42
		}, func(value int) {
			gotPut = value
		})

		if getCalls != 1 {
			t.Fatalf("get call count = %d, want 1", getCalls)
		}
		if gotPut != 42 {
			t.Fatalf("put value = %d, want 42", gotPut)
		}
	})
}

func TestPrefillPool(t *testing.T) {
	t.Run("stores the requested number of constructed values", func(t *testing.T) {
		next := 0
		puts := make([]int, 0, 4)

		// The helper is meant to preserve constructor order while preloading a
		// pool-like sink. Using incrementing integers makes that contract
		// observable without depending on any specific pool implementation.
		PrefillPool(4, func() int {
			next++
			return next
		}, func(value int) {
			puts = append(puts, value)
		})

		if next != 4 {
			t.Fatalf("constructor call count = %d, want 4", next)
		}
		if len(puts) != 4 {
			t.Fatalf("stored value count = %d, want 4", len(puts))
		}
		if want := []int{1, 2, 3, 4}; !slices.Equal(puts, want) {
			t.Fatalf("stored values = %v, want %v", puts, want)
		}
	})
}

func TestParallelWarmCount(t *testing.T) {
	t.Run("scales with current GOMAXPROCS", func(t *testing.T) {
		// The benchmark suite uses a fixed factor per active P so prefill volume
		// grows with the amount of parallelism the runtime may expose.
		want := runtime.GOMAXPROCS(0) * 16
		if got := ParallelWarmCount(); got != want {
			t.Fatalf("ParallelWarmCount() = %d, want %d", got, want)
		}
	})
}

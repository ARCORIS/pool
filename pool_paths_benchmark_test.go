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
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

// This file contains the package-level lifecycle-path benchmarks for
// arcoris.dev/pool.
//
// Its responsibility is intentionally narrow. These benchmarks do not compare
// the package against plain allocation or direct sync.Pool usage; that belongs
// to pool_baseline_benchmark_test.go. They also do not explore broader type
// shape sensitivity or parallel scaling; those concerns belong to the shapes
// and parallel benchmark suites.
//
// The purpose of this file is to make the major return-path branches of Pool[T]
// visible and measurable in isolation:
//   - the controlled accepted steady-state path;
//   - the realistic serial accepted path;
//   - the realistic serial rejected path with cheap observation;
//   - the controlled accepted path with intentionally more expensive reset work;
//   - the realistic serial rejected path with deliberately non-trivial OnDrop
//     work.
//
// Mutation work is deliberate and category-specific:
// accepted-path cases keep object work light, while reset-heavy and
// drop-observed cases intentionally make those lifecycle phases visible.

// pathAccepted is the canonical pointer-like value used for accepted lifecycle
// benchmarks.
//
// The shape is intentionally small but not trivial: it includes both scalar
// fields and a reusable scratch slice so that the benchmark resembles a normal
// temporary mutable object rather than an empty shell.
type pathAccepted struct {
	ID      int
	Flags   uint64
	Scratch []byte
}

// pathHeavyReset is used by the reset-heavy benchmark.
//
// The type intentionally carries multiple slices and reference-bearing state so
// that Reset can do real work. The goal is not to model a specific domain type,
// but to make reset cost large enough to be visible independently of backend
// cost.
type pathHeavyReset struct {
	Bytes  []byte
	Tokens []pathToken
	Refs   []*pathToken
	Counts []int
}

// pathToken is reused by pathHeavyReset to keep the benchmark focused on reset
// cost rather than per-iteration heap churn from temporary token allocation.
type pathToken struct {
	Value int
}

var (
	pathAcceptedSink   *pathAccepted
	pathHeavyResetSink *pathHeavyReset
	pathDropSink       *pathAccepted
	pathDropWorkSink   uint64
)

// newPathAccepted constructs the steady-state accepted-path benchmark shape.
//
// The scratch buffer is preallocated so accepted-path benchmarks can reuse the
// same backing storage instead of re-measuring slice growth on every iteration.
func newPathAccepted() *pathAccepted {
	return &pathAccepted{
		Scratch: make([]byte, 0, 256),
	}
}

// useAcceptedPath applies a compact, repeatable mutation to the accepted-path
// shape.
//
// The work is intentionally light so accepted-path benchmarks keep the focus
// on lifecycle and backend cost rather than on synthetic object processing.
func useAcceptedPath(v *pathAccepted, i int) {
	v.ID = i
	v.Flags ^= uint64(i + 1)
	v.Scratch = append(v.Scratch[:0], byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
}

// resetAcceptedPath returns the accepted-path object to the same logical state
// expected from newPathAccepted.
func resetAcceptedPath(v *pathAccepted) {
	v.ID = 0
	v.Flags = 0
	v.Scratch = v.Scratch[:0]
}

// newPathHeavyReset constructs the shape used by the reset-dominated path
// benchmark.
//
// The object carries retained byte, pointer, and counter state so Reset has
// meaningful cleanup work to perform before the value can be reused safely.
func newPathHeavyReset() *pathHeavyReset {
	return &pathHeavyReset{
		Bytes:  make([]byte, 0, 4096),
		Tokens: make([]pathToken, 128),
		Refs:   make([]*pathToken, 0, 256),
		Counts: make([]int, 0, 256),
	}
}

// useHeavyResetPath fills the reset-heavy shape with enough state that the
// subsequent Reset call performs visible work.
//
// Tokens are reused in place so the benchmark studies reset cost and retained
// state cleanup rather than measuring an unrelated stream of per-iteration
// heap allocations for token creation.
func useHeavyResetPath(v *pathHeavyReset, i int) {
	v.Bytes = v.Bytes[:cap(v.Bytes)]
	for j := range v.Bytes {
		v.Bytes[j] = byte(i + j)
	}

	for j := range v.Tokens {
		v.Tokens[j].Value = i + j
	}

	v.Refs = v.Refs[:0]
	v.Counts = v.Counts[:0]
	for j := range v.Tokens {
		v.Refs = append(v.Refs, &v.Tokens[j])
		v.Counts = append(v.Counts, i+j)
	}
}

// heavyResetPath explicitly clears all retained state of the reset-heavy
// benchmark shape.
//
// The reset work is intentionally broader than the accepted-path reset above:
// this benchmark exists to make expensive cleanup visible as its own category.
func heavyResetPath(v *pathHeavyReset) {
	for i := range v.Bytes {
		v.Bytes[i] = 0
	}
	v.Bytes = v.Bytes[:0]

	for i := range v.Tokens {
		v.Tokens[i].Value = 0
	}

	for i := range v.Refs {
		v.Refs[i] = nil
	}
	v.Refs = v.Refs[:0]

	for i := range v.Counts {
		v.Counts[i] = 0
	}
	v.Counts = v.Counts[:0]
}

// benchmarkPathsControlledAccepted implements the controlled accepted-path
// benchmark so it can be reused from the compare suite without calling a
// benchmark entrypoint from another benchmark.
func benchmarkPathsControlledAccepted(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := New(Options[*pathAccepted]{
			New: func() *pathAccepted {
				news++
				return newPathAccepted()
			},
			Reset: resetAcceptedPath,
			Reuse: func(*pathAccepted) bool { return true },
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			useAcceptedPath(v, i)
			pathAcceptedSink = v
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// benchmarkPathsRealisticAccepted implements the realistic serial accepted-path
// benchmark.
//
// The benchmark primes one reusable value before timing starts so the timed
// body measures a warm accepted path under the ordinary runtime rather than a
// cold constructor miss. Unlike the controlled companion above, it does not
// force single-P execution or disable GC.
func benchmarkPathsRealisticAccepted(b *testing.B) {
	var news uint64

	p := New(Options[*pathAccepted]{
		New: func() *pathAccepted {
			news++
			return newPathAccepted()
		},
		Reset: resetAcceptedPath,
		Reuse: func(*pathAccepted) bool { return true },
	})

	testutil.PrimePoolValue(p.Get, p.Put)
	news = 0

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := p.Get()
		useAcceptedPath(v, i)
		pathAcceptedSink = v
		p.Put(v)
	}

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
}

// benchmarkPathsRealisticRejected implements the realistic serial rejected-path
// benchmark.
func benchmarkPathsRealisticRejected(b *testing.B) {
	var news uint64
	var drops uint64
	var denials uint64

	p := New(Options[*pathAccepted]{
		New: func() *pathAccepted {
			news++
			return newPathAccepted()
		},
		Reset: resetAcceptedPath,
		Reuse: func(*pathAccepted) bool {
			denials++
			return false
		},
		OnDrop: func(v *pathAccepted) {
			drops++
			pathDropSink = v
		},
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := p.Get()
		useAcceptedPath(v, i)
		p.Put(v)
	}

	b.StopTimer()
	testutil.ReportLifecycleMetrics(b, news, drops, denials)
}

// benchmarkPathsControlledResetHeavy implements the controlled accepted-path
// benchmark whose reset work intentionally dominates the measured cost.
func benchmarkPathsControlledResetHeavy(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := New(Options[*pathHeavyReset]{
			New: func() *pathHeavyReset {
				news++
				return newPathHeavyReset()
			},
			Reset: heavyResetPath,
			Reuse: func(*pathHeavyReset) bool { return true },
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			useHeavyResetPath(v, i)
			pathHeavyResetSink = v
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// benchmarkPathsRealisticDropObserved implements the realistic serial rejected-path
// benchmark with deliberately non-trivial OnDrop work.
func benchmarkPathsRealisticDropObserved(b *testing.B) {
	var news uint64
	var drops uint64
	var denials uint64
	var dropWork uint64

	p := New(Options[*pathAccepted]{
		New: func() *pathAccepted {
			news++
			return newPathAccepted()
		},
		Reset: resetAcceptedPath,
		Reuse: func(v *pathAccepted) bool {
			_ = len(v.Scratch)
			denials++
			return false
		},
		OnDrop: func(v *pathAccepted) {
			drops++
			for i := 0; i < len(v.Scratch); i++ {
				dropWork += uint64(v.Scratch[i])
			}
			pathDropSink = v
		},
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := p.Get()
		useAcceptedPath(v, i)
		// Make the drop callback do measurable work on the accumulated state.
		v.Scratch = testutil.AppendSamplePayload(v.Scratch)
		p.Put(v)
	}

	b.StopTimer()
	pathDropWorkSink = dropWork
	testutil.ReportLifecycleMetrics(b, news, drops, denials)
}

// BenchmarkPaths_ControlledAccepted measures the controlled accepted return
// path.
//
// The benchmark exercises the steady-state path where:
//   - Reuse always accepts the value;
//   - Reset is cheap;
//   - OnDrop is never used.
//
// The harness forces a local steady-state reuse mode so the result is a
// controlled hot path serial measurement, not a whole-package serial average.
func BenchmarkPaths_ControlledAccepted(b *testing.B) {
	benchmarkPathsControlledAccepted(b)
}

// BenchmarkPaths_RealisticAccepted measures the accepted return path under the
// ordinary serial runtime.
//
// This benchmark keeps the same lightweight representative mutation as the
// controlled accepted-path case but allows incidental constructor misses caused
// by the normal runtime to remain visible through news/op.
func BenchmarkPaths_RealisticAccepted(b *testing.B) {
	benchmarkPathsRealisticAccepted(b)
}

// BenchmarkPaths_RealisticRejected measures the realistic serial rejected path
// with cheap drop observation.
func BenchmarkPaths_RealisticRejected(b *testing.B) {
	benchmarkPathsRealisticRejected(b)
}

// BenchmarkPaths_ControlledResetHeavy measures a controlled accepted path whose
// cost is dominated by deliberate reset work rather than by backend reuse
// mechanics.
func BenchmarkPaths_ControlledResetHeavy(b *testing.B) {
	benchmarkPathsControlledResetHeavy(b)
}

// BenchmarkPaths_RealisticDropObserved measures a realistic serial rejected
// path with intentionally more expensive OnDrop work.
func BenchmarkPaths_RealisticDropObserved(b *testing.B) {
	benchmarkPathsRealisticDropObserved(b)
}

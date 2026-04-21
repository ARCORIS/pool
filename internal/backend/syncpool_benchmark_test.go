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

package backend

import (
	"sync/atomic"
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

// The backend benchmark suite exists to measure only the low-level typed
// adapter over sync.Pool.
//
// These benchmarks intentionally avoid lifecycle policy concerns such as:
//   - reset cost;
//   - reuse admission logic;
//   - drop-path logic;
//   - public runtime orchestration.
//
// Those concerns belong to package-level benchmarks in the repository root.
// This file exists only to answer the backend questions defined by the
// benchmark matrix:
//   - what does a constructor miss cost;
//   - what does a controlled steady-state Get/Put round trip cost;
//   - how does pointer-like T compare to value T;
//   - what does the backend look like under realistic parallel access.
//
// Benchmark results from this file must therefore be interpreted only as a
// lower bound for public runtime cost. They are not substitutes for package
// baselines or lifecycle-path benchmarks.
//
// The steady-state loops still apply a small unit of representative mutation
// before Put. These are backend-focused workload benchmarks, not pure
// zero-work API-call measurements.

// syncPoolBenchmarkPointer is the canonical pointer-like benchmark shape for
// backend baselines.
//
// The object is intentionally small and mutable. The benchmark is interested in
// backend retrieval and return costs, not in modelling a complex domain object.
// A single byte array is enough to make the type non-empty and reduce the risk
// of unrealistic zero-field behaviour dominating the result.
type syncPoolBenchmarkPointer struct {
	ID   int
	Data [16]byte
}

// syncPoolBenchmarkValue is the canonical value-type benchmark shape for
// backend baselines.
//
// The type is intentionally copied by value so that the benchmark can expose
// the difference between the intended pointer-like path and a by-value path.
// This matches the package documentation, which treats pointer-like reusable
// values as the expected primary use case for arcoris.dev/pool.
type syncPoolBenchmarkValue struct {
	A uint64
	B uint64
	C uint64
	D uint64
}

var (
	syncPoolBenchmarkPointerSink *syncPoolBenchmarkPointer
	syncPoolBenchmarkValueSink   syncPoolBenchmarkValue
)

// BenchmarkSyncPool_GetMiss measures the pure backend miss path.
//
// The benchmark deliberately never returns values to the backend. As a result,
// every Get call must fall through to the constructor installed in
// sync.Pool.New.
//
// This benchmark answers two questions:
//   - what is the cost of a constructor miss at the backend layer;
//   - how many fresh values are materialized per iteration.
//
// The benchmark reports a custom news/op metric so reports can state not only
// that the path is slower, but also that the slowdown corresponds to forced
// construction rather than to reuse mechanics.
func BenchmarkSyncPool_GetMiss(b *testing.B) {
	var news uint64

	p := NewSyncPool(func() *syncPoolBenchmarkPointer {
		news++
		return &syncPoolBenchmarkPointer{}
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := p.Get()
		v.ID = i
		syncPoolBenchmarkPointerSink = v
	}

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
}

// BenchmarkSyncPool_ControlledGetPut_Pointer measures the controlled
// steady-state backend round trip for the intended pointer-like value shape.
//
// The benchmark uses a controlled steady-state helper that:
//   - pins execution to one P;
//   - disables GC for the duration of the benchmark body.
//
// This is intentional. The benchmark is not trying to simulate arbitrary
// scheduler-shaped reuse. It is trying to measure the minimum backend cost of a
// hot Get/Put loop when a reusable pointer-like value is available.
//
// A small warm-up step preloads one value into the backend before the timer is
// started so the timed loop measures the reuse path rather than the first miss.
func BenchmarkSyncPool_ControlledGetPut_Pointer(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := NewSyncPool(func() *syncPoolBenchmarkPointer {
			news++
			return &syncPoolBenchmarkPointer{}
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			v.ID = i
			v.Data[0] = byte(i)
			p.Put(v)
			syncPoolBenchmarkPointerSink = v
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// BenchmarkSyncPool_ControlledGetPut_Value measures the same controlled
// steady-state round trip for a by-value T.
//
// This benchmark exists because the backend is generic while the package is
// architecturally tuned for pointer-like temporary values. Comparing this case
// with the pointer benchmark shows how much shape alone changes backend cost.
//
// As in the pointer benchmark, the backend is preseeded before timing begins so
// the loop measures the reuse path instead of including first-miss cost.
func BenchmarkSyncPool_ControlledGetPut_Value(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := NewSyncPool(func() syncPoolBenchmarkValue {
			news++
			return syncPoolBenchmarkValue{}
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			v.A = uint64(i)
			v.B = uint64(i * 2)
			p.Put(v)
			syncPoolBenchmarkValueSink = v
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// BenchmarkSyncPool_RealisticParallel measures backend behaviour under
// realistic concurrent Get/Put traffic.
//
// This benchmark is intentionally pointer-like and reuse-oriented. Its purpose
// is to show what the backend costs under concurrent clients before public
// lifecycle policy is layered on top.
//
// The benchmark reports news/op so that reports can distinguish between:
//   - constructor misses caused by parallel pressure or cache distribution;
//   - pure round-trip cost on already reusable values.
//
// A small prefill step is performed before timing begins. The goal is not to
// guarantee zero misses across all Ps, which sync.Pool does not promise, but to
// avoid measuring an entirely cold backend when the question is concurrent
// steady-state behaviour.
func BenchmarkSyncPool_RealisticParallel(b *testing.B) {
	var news atomic.Uint64

	p := NewSyncPool(func() *syncPoolBenchmarkPointer {
		news.Add(1)
		return &syncPoolBenchmarkPointer{}
	})

	testutil.PrefillPool(testutil.ParallelWarmCount(), func() *syncPoolBenchmarkPointer {
		return &syncPoolBenchmarkPointer{}
	}, p.Put)

	news.Store(0)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v := p.Get()
			v.ID++
			v.Data[0]++
			p.Put(v)
		}
	})

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news.Load(), testutil.MetricNewsPerOp)
}

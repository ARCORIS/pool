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

// This file contains metric-oriented benchmarks.
//
// These benchmarks exist to surface lifecycle-observable counters that standard
// benchmark output does not capture directly. They complement the baseline and
// path suites by making constructor pressure, drop frequency, and reuse-denial
// rates first-class metrics in their own right.
//
// The file intentionally contains both controlled and realistic serial cases:
//   - one controlled accepted-path benchmark for warm steady-state counters;
//   - realistic serial rejected and mixed workloads where counters describe the
//     policy outcome of each iteration.

// metricsShape is the canonical pointer-backed value used by the metrics suite.
//
// The type is intentionally simple because these benchmarks are about emitted
// pool-specific counters, not about stress-testing type-shape complexity.
type metricsShape struct {
	Counter int
	Payload []byte
}

var metricsShapeSink *metricsShape

func benchmarkMetricsControlledAcceptedWarmPath(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := New(Options[*metricsShape]{
			New: func() *metricsShape {
				news++
				return &metricsShape{Payload: make([]byte, 0, 256)}
			},
			Reset: func(v *metricsShape) {
				v.Counter = 0
				v.Payload = v.Payload[:0]
			},
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			v.Counter = i
			v.Payload = testutil.AppendSamplePayload(v.Payload[:0])
			metricsShapeSink = v
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// BenchmarkMetrics_ControlledAcceptedWarmPath measures the controlled
// steady-state accepted path with an explicit news/op counter.
func BenchmarkMetrics_ControlledAcceptedWarmPath(b *testing.B) {
	benchmarkMetricsControlledAcceptedWarmPath(b)
}

// BenchmarkMetrics_RealisticRejectedSteadyState measures the always-rejected
// serial path with explicit news/op, drops/op, and reuse_denials/op reporting.
func benchmarkMetricsRealisticRejectedSteadyState(b *testing.B) {
	var news uint64
	var drops uint64
	var denials uint64

	p := New(Options[*metricsShape]{
		New: func() *metricsShape {
			news++
			return &metricsShape{Payload: make([]byte, 0, 256)}
		},
		Reset: func(v *metricsShape) {
			v.Counter = 0
			v.Payload = v.Payload[:0]
		},
		Reuse: func(*metricsShape) bool {
			denials++
			return false
		},
		OnDrop: func(v *metricsShape) {
			drops++
			metricsShapeSink = v
		},
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := p.Get()
		v.Counter = i
		v.Payload = testutil.AppendSamplePayload(v.Payload[:0])
		p.Put(v)
	}

	b.StopTimer()
	testutil.ReportLifecycleMetrics(b, news, drops, denials)
}

// BenchmarkMetrics_RealisticRejectedSteadyState measures the always-rejected
// serial path with explicit news/op, drops/op, and reuse_denials/op reporting.
func BenchmarkMetrics_RealisticRejectedSteadyState(b *testing.B) {
	benchmarkMetricsRealisticRejectedSteadyState(b)
}

// BenchmarkMetrics_RealisticMixedReuse measures a serial workload with mostly
// accepted reuse and periodic oversized values that must be denied.
//
// The benchmark primes one reusable value before timing starts, but otherwise
// runs under the ordinary serial runtime. Additional constructor misses are
// therefore part of the observed realistic-path behaviour.
func benchmarkMetricsRealisticMixedReuse(b *testing.B) {
	var news uint64
	var drops uint64
	var denials uint64

	p := New(Options[*metricsShape]{
		New: func() *metricsShape {
			news++
			return &metricsShape{Payload: make([]byte, 0, 512)}
		},
		Reset: func(v *metricsShape) {
			v.Counter = 0
			v.Payload = v.Payload[:0]
		},
		Reuse: func(v *metricsShape) bool {
			if cap(v.Payload) > 4096 {
				denials++
				return false
			}
			return true
		},
		OnDrop: func(v *metricsShape) {
			drops++
			metricsShapeSink = v
		},
	})

	testutil.PrimePoolValue(p.Get, p.Put)
	news = 0

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := p.Get()
		v.Counter = i
		if i%8 == 0 {
			v.Payload = testutil.AppendOversizedPayload(v.Payload[:0])
		} else {
			v.Payload = testutil.AppendSamplePayload(v.Payload[:0])
		}
		p.Put(v)
	}

	b.StopTimer()
	testutil.ReportLifecycleMetrics(b, news, drops, denials)
}

func BenchmarkMetrics_RealisticMixedReuse(b *testing.B) {
	benchmarkMetricsRealisticMixedReuse(b)
}

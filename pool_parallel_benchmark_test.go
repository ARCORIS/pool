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
	"sync"
	"sync/atomic"
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

// This file implements the parallel benchmark suite for arcoris.dev/pool.
//
// The suite exists to make concurrent behaviour visible separately from the
// serial benchmarks. Serial steady-state results are useful, but they do not
// answer how the runtime behaves under concurrent Get/Put traffic or how much
// extra cost comes from package orchestration relative to direct sync.Pool
// usage.
//
// Unlike the controlled serial hot path benchmarks elsewhere in the suite,
// these cases run under the ordinary scheduler and garbage collector. They are
// therefore the repository's realistic parallel view, even though a small
// prefill step is still used to avoid measuring a completely cold pool.
//
// Each loop includes a minimal representative object mutation before Put.
// These benchmarks therefore measure concurrent pooling plus lightweight
// object work, not a zero-work API-call microbenchmark.

// parallelAccepted is the pointer-backed shape used by the accepted and
// rejected public-runtime parallel benchmarks.
type parallelAccepted struct {
	ID      uint64
	Scratch []byte
}

// parallelCompare is the compact pointer-backed shape shared by the raw
// sync.Pool and public-runtime parallel comparison baselines.
type parallelCompare struct {
	ID   uint64
	Data [16]byte
}

func benchmarkRealisticParallelAccepted(b *testing.B) {
	var news atomic.Uint64

	p := New(Options[*parallelAccepted]{
		New: func() *parallelAccepted {
			news.Add(1)
			return &parallelAccepted{Scratch: make([]byte, 0, 256)}
		},
		Reset: func(v *parallelAccepted) {
			v.ID = 0
			v.Scratch = v.Scratch[:0]
		},
		Reuse: func(*parallelAccepted) bool { return true },
	})

	testutil.PrefillPool(testutil.ParallelWarmCount(), func() *parallelAccepted {
		return &parallelAccepted{Scratch: make([]byte, 0, 256)}
	}, p.Put)
	news.Store(0)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v := p.Get()
			v.ID++
			v.Scratch = testutil.AppendSamplePayload(v.Scratch[:0])
			p.Put(v)
		}
	})

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news.Load(), testutil.MetricNewsPerOp)
}

// BenchmarkParallel_RealisticAccepted measures the concurrent accepted path of
// the public runtime.
func BenchmarkParallel_RealisticAccepted(b *testing.B) {
	benchmarkRealisticParallelAccepted(b)
}

func benchmarkRealisticParallelRejected(b *testing.B) {
	var news atomic.Uint64
	var drops atomic.Uint64
	var denials atomic.Uint64

	p := New(Options[*parallelAccepted]{
		New: func() *parallelAccepted {
			news.Add(1)
			return &parallelAccepted{Scratch: make([]byte, 0, 256)}
		},
		Reset: func(v *parallelAccepted) {
			v.ID = 0
			v.Scratch = v.Scratch[:0]
		},
		Reuse: func(*parallelAccepted) bool {
			denials.Add(1)
			return false
		},
		OnDrop: func(*parallelAccepted) {
			drops.Add(1)
		},
	})

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v := p.Get()
			v.ID++
			v.Scratch = testutil.AppendSamplePayload(v.Scratch[:0])
			p.Put(v)
		}
	})

	b.StopTimer()
	testutil.ReportLifecycleMetrics(b, news.Load(), drops.Load(), denials.Load())
}

// BenchmarkParallel_RealisticRejected measures the concurrent rejected path of
// the public runtime.
func BenchmarkParallel_RealisticRejected(b *testing.B) {
	benchmarkRealisticParallelRejected(b)
}

// benchmarkRealisticParallelRawSyncPool measures concurrent direct sync.Pool
// usage as the closest low-level external baseline for the package runtime.
func benchmarkRealisticParallelRawSyncPool(b *testing.B) {
	var news atomic.Uint64

	var p sync.Pool
	p.New = func() any {
		news.Add(1)
		return &parallelCompare{}
	}

	testutil.PrefillPool(testutil.ParallelWarmCount(), func() *parallelCompare {
		return &parallelCompare{}
	}, func(v *parallelCompare) {
		p.Put(v)
	})
	news.Store(0)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v := p.Get().(*parallelCompare)
			v.ID++
			v.Data[0]++
			v.Data[1] = byte(v.ID)
			v.ID = 0
			v.Data = [16]byte{}
			p.Put(v)
		}
	})

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news.Load(), testutil.MetricNewsPerOp)
}

// BenchmarkParallel_RealisticRawSyncPool measures concurrent direct sync.Pool
// usage as the closest low-level external baseline for the package runtime.
func BenchmarkParallel_RealisticRawSyncPool(b *testing.B) {
	benchmarkRealisticParallelRawSyncPool(b)
}

// benchmarkRealisticParallelARCORISPool measures concurrent public-runtime
// usage for the same object shape as
// BenchmarkParallel_RealisticRawSyncPool.
func benchmarkRealisticParallelARCORISPool(b *testing.B) {
	var news atomic.Uint64

	p := New(Options[*parallelCompare]{
		New: func() *parallelCompare {
			news.Add(1)
			return &parallelCompare{}
		},
		Reset: func(v *parallelCompare) {
			v.ID = 0
			v.Data = [16]byte{}
		},
	})

	testutil.PrefillPool(testutil.ParallelWarmCount(), func() *parallelCompare {
		return &parallelCompare{}
	}, p.Put)
	news.Store(0)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v := p.Get()
			v.ID++
			v.Data[0]++
			v.Data[1] = byte(v.ID)
			p.Put(v)
		}
	})

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news.Load(), testutil.MetricNewsPerOp)
}

// BenchmarkParallel_RealisticARCORISPool measures concurrent public-runtime
// usage for the same object shape as BenchmarkParallel_RealisticRawSyncPool.
func BenchmarkParallel_RealisticARCORISPool(b *testing.B) {
	benchmarkRealisticParallelARCORISPool(b)
}

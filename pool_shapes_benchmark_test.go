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

// This file implements the value-shape benchmark suite for arcoris.dev/pool.
//
// The purpose of this suite is to isolate how the shape of T changes the cost
// profile of the public runtime. These benchmarks do not compare against plain
// allocation or direct sync.Pool usage. Those questions belong to the baseline
// suite. They also do not focus on parallelism, which belongs to the parallel
// suite.
//
// The shape suite exists because the package is generic while its intended
// primary use case is not: pointer-like temporary objects with explicit reset
// and reuse policy. Different shapes can materially change copying behaviour,
// reset cost, retained memory, and reuse usefulness. The suite therefore makes
// those differences visible instead of allowing one benchmark shape to stand in
// for all possible T.
//
// Most cases in this file are controlled accepted-path serial benchmarks so the
// effect of T itself is not drowned out by scheduler or GC variability. The
// final oversized-rejection case is intentionally different: it is a realistic
// serial path that models values which are always too large to retain.

type shapePointerSmall struct {
	A uint64
	B uint64
}

type shapePointerWithSlices struct {
	Bytes  []byte
	Tokens []shapeToken
}

type shapeToken struct {
	Kind  int
	Value int
}

type shapeValueSmall struct {
	A uint64
	B uint64
	C uint64
	D uint64
}

type shapeValueLarge struct {
	A uint64
	B uint64
	C uint64
	D uint64
	E uint64
	F uint64
	G uint64
	H uint64
	I uint64
	J uint64
	K uint64
	L uint64
	M uint64
	N uint64
	O uint64
	P uint64
}

type shapeOversizedRejected struct {
	Payload []byte
}

var (
	shapePointerSmallSink      *shapePointerSmall
	shapePointerWithSlicesSink *shapePointerWithSlices
	shapeValueSmallSink        shapeValueSmall
	shapeValueLargeSink        shapeValueLarge
	shapeOversizedRejectedSink *shapeOversizedRejected
)

// benchmarkAcceptedShape implements the stable accepted-path shape benchmark
// pattern used by the four reuse-oriented shape cases in this file.
//
// The helper keeps warm-up, timer control, and per-op news reporting identical
// across shape variants so comparisons stay about the shape of T rather than
// about benchmark harness drift.
func benchmarkAcceptedShape[T any](
	b *testing.B,
	newFn func() T,
	reset ResetFunc[T],
	use func(T, int) T,
	store func(T),
	news *uint64,
) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		p := New(Options[T]{
			New:   newFn,
			Reset: reset,
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		*news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			v = use(v, i)
			store(v)
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, *news, testutil.MetricNewsPerOp)
	})
}

func benchmarkControlledShapePointerSmall(b *testing.B) {
	var news uint64

	benchmarkAcceptedShape(
		b,
		func() *shapePointerSmall {
			news++
			return &shapePointerSmall{}
		},
		func(v *shapePointerSmall) {
			v.A = 0
			v.B = 0
		},
		func(v *shapePointerSmall, i int) *shapePointerSmall {
			v.A = uint64(i)
			v.B = uint64(i * 3)
			return v
		},
		func(v *shapePointerSmall) {
			shapePointerSmallSink = v
		},
		&news,
	)
}

// BenchmarkShapes_ControlledPointerSmall measures the most compact pointer-like
// shape in the suite under a controlled accepted-path serial harness.
func BenchmarkShapes_ControlledPointerSmall(b *testing.B) {
	benchmarkControlledShapePointerSmall(b)
}

func benchmarkControlledShapePointerWithSlices(b *testing.B) {
	var news uint64

	benchmarkAcceptedShape(
		b,
		func() *shapePointerWithSlices {
			news++
			return &shapePointerWithSlices{
				Bytes:  make([]byte, 0, 512),
				Tokens: make([]shapeToken, 0, 64),
			}
		},
		func(v *shapePointerWithSlices) {
			v.Bytes = v.Bytes[:0]
			v.Tokens = v.Tokens[:0]
		},
		func(v *shapePointerWithSlices, i int) *shapePointerWithSlices {
			v.Bytes = append(v.Bytes[:0], byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
			for j := 0; j < 16; j++ {
				v.Tokens = append(v.Tokens, shapeToken{Kind: j, Value: i + j})
			}
			return v
		},
		func(v *shapePointerWithSlices) {
			shapePointerWithSlicesSink = v
		},
		&news,
	)
}

// BenchmarkShapes_ControlledPointerWithSlices measures a more realistic
// pointer-like shape with retained scratch slices under a controlled accepted-
// path serial harness.
func BenchmarkShapes_ControlledPointerWithSlices(b *testing.B) {
	benchmarkControlledShapePointerWithSlices(b)
}

func benchmarkControlledShapeValueSmall(b *testing.B) {
	var news uint64

	benchmarkAcceptedShape(
		b,
		func() shapeValueSmall {
			news++
			return shapeValueSmall{}
		},
		func(v shapeValueSmall) {
			_ = v
		},
		func(v shapeValueSmall, i int) shapeValueSmall {
			v.A = uint64(i)
			v.B = uint64(i * 2)
			v.C = uint64(i * 3)
			v.D = uint64(i * 4)
			return v
		},
		func(v shapeValueSmall) {
			shapeValueSmallSink = v
		},
		&news,
	)
}

// BenchmarkShapes_ControlledValueSmall measures a small by-value shape as an
// explicit contrast case to the intended pointer-like primary path.
func BenchmarkShapes_ControlledValueSmall(b *testing.B) {
	benchmarkControlledShapeValueSmall(b)
}

func benchmarkControlledShapeValueLarge(b *testing.B) {
	var news uint64

	benchmarkAcceptedShape(
		b,
		func() shapeValueLarge {
			news++
			return shapeValueLarge{}
		},
		func(v shapeValueLarge) {
			_ = v
		},
		func(v shapeValueLarge, i int) shapeValueLarge {
			v.A = uint64(i)
			v.B = uint64(i + 1)
			v.C = uint64(i + 2)
			v.D = uint64(i + 3)
			v.E = uint64(i + 4)
			v.F = uint64(i + 5)
			v.G = uint64(i + 6)
			v.H = uint64(i + 7)
			v.I = uint64(i + 8)
			v.J = uint64(i + 9)
			v.K = uint64(i + 10)
			v.L = uint64(i + 11)
			v.M = uint64(i + 12)
			v.N = uint64(i + 13)
			v.O = uint64(i + 14)
			v.P = uint64(i + 15)
			return v
		},
		func(v shapeValueLarge) {
			shapeValueLargeSink = v
		},
		&news,
	)
}

// BenchmarkShapes_ControlledValueLarge measures a larger by-value shape whose
// copy cost is expected to be more visible than in
// BenchmarkShapes_ControlledValueSmall.
func BenchmarkShapes_ControlledValueLarge(b *testing.B) {
	benchmarkControlledShapeValueLarge(b)
}

func benchmarkShapeAlwaysOversizedRejected(b *testing.B) {
	var news uint64
	var drops uint64
	var denials uint64

	p := New(Options[*shapeOversizedRejected]{
		New: func() *shapeOversizedRejected {
			news++
			return &shapeOversizedRejected{Payload: make([]byte, 0, 1024)}
		},
		Reset: func(v *shapeOversizedRejected) {
			v.Payload = v.Payload[:0]
		},
		Reuse: func(v *shapeOversizedRejected) bool {
			if cap(v.Payload) > 4096 {
				denials++
				return false
			}
			return true
		},
		OnDrop: func(v *shapeOversizedRejected) {
			drops++
			shapeOversizedRejectedSink = v
		},
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := p.Get()
		// This benchmark is intentionally "always oversized", not mixed-size.
		// Every iteration crosses the reuse threshold before Put.
		v.Payload = testutil.AppendOversizedPayload(v.Payload[:0])
		p.Put(v)
	}

	b.StopTimer()
	testutil.ReportLifecycleMetrics(b, news, drops, denials)
}

// BenchmarkShapes_AlwaysOversizedRejected measures a realistic serial workload
// where every returned value is oversized and must be rejected.
//
// Unlike the controlled accepted-path cases above, this benchmark models a
// clear policy outcome rather than a hot reuse loop: each iteration grows the
// payload beyond the configured reuse threshold before Put is called.
func BenchmarkShapes_AlwaysOversizedRejected(b *testing.B) {
	benchmarkShapeAlwaysOversizedRejected(b)
}

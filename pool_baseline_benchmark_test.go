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
	"testing"

	"arcoris.dev/pool/internal/testutil"
)

// This file contains the package-level baseline benchmarks for arcoris.dev/pool.
//
// The responsibility of this file is intentionally narrow. These benchmarks
// answer only the baseline comparison questions defined by the benchmark
// matrix:
//   - what plain allocation looks like for a temporary-object shape;
//   - what direct sync.Pool usage looks like for the same shape;
//   - what arcoris.dev/pool adds on top of that baseline.
//
// The suite mixes two serial baseline modes on purpose:
//   - direct construction baselines, which show serial fresh-value cost;
//   - controlled steady-state reuse baselines, which use a single-P,
//     GC-disabled harness to measure hot reuse paths explicitly.
//
// This file does not attempt to isolate lifecycle-path branches in detail,
// measure broader type-shape sensitivity, or study parallel scaling. Those
// concerns belong to other benchmark files.
//
// The baseline suite intentionally uses two benchmark shapes:
//   - a pointer-like temporary object, which represents the intended primary
//     use case of the package;
//   - a value type, which acts as a contrast case and makes the cost of
//     boxing, copying, and value-oriented usage visible.
//
// None of these cases are pure API-call microbenchmarks. Each iteration
// includes a small, explicit unit of representative object mutation so the
// baselines remain closer to plausible temporary-object usage.

// baselinePointer is the canonical pointer-like shape used by the baseline
// comparison suite.
//
// The object intentionally carries both scalar state and a reusable scratch
// slice. This makes it representative of the kinds of temporary mutable values
// pooling is usually meant for: parser states, request scratch objects,
// builders, envelopes, or reusable work records.
type baselinePointer struct {
	ID      int
	Flags   uint64
	Scratch []byte
}

// baselineValue is the canonical value-type shape used by the baseline suite.
//
// The type is intentionally copied by value and contains enough scalar state to
// make copying visible without turning the benchmark into a pathological large
// struct exercise. It exists to show how the generic runtime behaves when T is
// not pointer-like.
type baselineValue struct {
	A uint64
	B uint64
	C uint64
	D uint64
	E uint64
	F uint64
}

var (
	baselinePointerSink *baselinePointer
	baselineValueSink   baselineValue
)

// newBaselinePointer constructs a fresh pointer-like benchmark value with a
// stable reusable scratch slice.
//
// The constructor shape is shared by all pointer baseline variants so the
// three strategies differ in pooling semantics, not in object definition.
func newBaselinePointer() *baselinePointer {
	return &baselinePointer{
		Scratch: make([]byte, 0, 256),
	}
}

// resetBaselinePointer restores a pointer benchmark value to the same logical
// state expected from newBaselinePointer.
func resetBaselinePointer(v *baselinePointer) {
	v.ID = 0
	v.Flags = 0
	v.Scratch = v.Scratch[:0]
}

// exerciseBaselinePointer applies a small, stable unit of mutation to the
// pointer benchmark value.
//
// The mutation is intentionally light. The baseline suite is meant to expose
// allocation and pooling cost, not to bury those signals under heavy object
// work.
func exerciseBaselinePointer(v *baselinePointer, i int) {
	v.ID = i
	v.Flags ^= uint64(i + 1)
	v.Scratch = append(v.Scratch[:0], byte(i), byte(i>>1), byte(i>>2), byte(i>>3))
}

// exerciseBaselineValue applies a small, stable unit of mutation to the value
// benchmark shape.
//
// The function overwrites all fields it touches so the benchmark does not rely
// on reset semantics to produce deterministic work for value paths.
func exerciseBaselineValue(v baselineValue, i int) baselineValue {
	v.A = uint64(i)
	v.B = uint64(i * 2)
	v.C = v.A + v.B
	v.D = uint64(i + 7)
	v.E = v.C + v.D
	v.F = v.E ^ 0xA5A5A5A5A5A5A5A5
	return v
}

// benchmarkBaselineAllocOnlyPointer implements the plain-allocation pointer
// baseline so it can be reused both by the canonical benchmark and the grouped
// compare suite.
func benchmarkBaselineAllocOnlyPointer(b *testing.B) {
	var news uint64

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		news++
		v := newBaselinePointer()
		exerciseBaselinePointer(v, i)
		baselinePointerSink = v
	}

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
}

// benchmarkBaselineControlledRawSyncPoolPointer implements the controlled steady-state
// direct sync.Pool pointer baseline used by both the canonical benchmark and
// the compare suite.
func benchmarkBaselineControlledRawSyncPoolPointer(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := &sync.Pool{
			New: func() any {
				news++
				return newBaselinePointer()
			},
		}

		testutil.PrimePoolValue(func() *baselinePointer {
			return p.Get().(*baselinePointer)
		}, func(v *baselinePointer) {
			p.Put(v)
		})
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get().(*baselinePointer)
			exerciseBaselinePointer(v, i)
			baselinePointerSink = v
			resetBaselinePointer(v)
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// benchmarkBaselineControlledPoolPointer implements the controlled steady-state
// public-runtime pointer baseline.
func benchmarkBaselineControlledPoolPointer(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := New(Options[*baselinePointer]{
			New: func() *baselinePointer {
				news++
				return newBaselinePointer()
			},
			Reset: resetBaselinePointer,
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			exerciseBaselinePointer(v, i)
			baselinePointerSink = v
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// benchmarkBaselineAllocOnlyValue implements the plain-allocation value
// baseline.
func benchmarkBaselineAllocOnlyValue(b *testing.B) {
	var news uint64

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		news++
		baselineValueSink = exerciseBaselineValue(baselineValue{}, i)
	}

	b.StopTimer()
	testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
}

// benchmarkBaselineControlledRawSyncPoolValue implements the controlled steady-state
// direct sync.Pool value baseline.
func benchmarkBaselineControlledRawSyncPoolValue(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := &sync.Pool{
			New: func() any {
				news++
				return baselineValue{}
			},
		}

		testutil.PrimePoolValue(func() baselineValue {
			return p.Get().(baselineValue)
		}, func(v baselineValue) {
			p.Put(v)
		})
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get().(baselineValue)
			v = exerciseBaselineValue(v, i)
			baselineValueSink = v
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// benchmarkBaselineControlledPoolValue implements the controlled steady-state
// public-runtime value baseline.
func benchmarkBaselineControlledPoolValue(b *testing.B) {
	testutil.WithControlledSteadyStatePoolRoundTrip(b, func() {
		var news uint64

		p := New(Options[baselineValue]{
			New: func() baselineValue {
				news++
				return baselineValue{}
			},
		})

		testutil.PrimePoolValue(p.Get, p.Put)
		news = 0

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			v := p.Get()
			v = exerciseBaselineValue(v, i)
			baselineValueSink = v
			p.Put(v)
		}

		b.StopTimer()
		testutil.ReportPerOpMetric(b, news, testutil.MetricNewsPerOp)
	})
}

// BenchmarkBaseline_AllocOnly_Pointer measures plain allocation for the
// pointer-like temporary-object shape.
//
// This benchmark is the non-pooled baseline. It exists to answer the most basic
// question in the repository's performance story: what does ordinary fresh
// allocation cost for the intended primary object shape.
func BenchmarkBaseline_AllocOnly_Pointer(b *testing.B) {
	benchmarkBaselineAllocOnlyPointer(b)
}

// BenchmarkBaseline_Controlled_RawSyncPool_Pointer measures the controlled
// steady-state direct sync.Pool reuse path for the pointer-like temporary
// object shape.
//
// The harness pins execution to one P, disables GC for the timed body, and
// primes the pool before timing starts. The result is therefore a hot path
// upper-bound serial reuse benchmark, not a claim about general runtime
// behaviour.
func BenchmarkBaseline_Controlled_RawSyncPool_Pointer(b *testing.B) {
	benchmarkBaselineControlledRawSyncPoolPointer(b)
}

// BenchmarkBaseline_Controlled_ARCORISPool_Pointer measures the controlled
// steady-state public runtime for the same pointer-like temporary-object shape.
//
// Like the raw sync.Pool companion, this is an upper-bound hot path serial
// reuse benchmark. It is useful for localizing public runtime overhead relative
// to the closest low-level baseline, but it is not a general package
// performance claim on its own.
func BenchmarkBaseline_Controlled_ARCORISPool_Pointer(b *testing.B) {
	benchmarkBaselineControlledPoolPointer(b)
}

// BenchmarkBaseline_AllocOnly_Value measures plain construction and discard for
// the canonical value-type benchmark shape.
func BenchmarkBaseline_AllocOnly_Value(b *testing.B) {
	benchmarkBaselineAllocOnlyValue(b)
}

// BenchmarkBaseline_Controlled_RawSyncPool_Value measures the controlled
// steady-state direct sync.Pool reuse path for the canonical value-type
// benchmark shape.
func BenchmarkBaseline_Controlled_RawSyncPool_Value(b *testing.B) {
	benchmarkBaselineControlledRawSyncPoolValue(b)
}

// BenchmarkBaseline_Controlled_ARCORISPool_Value measures the controlled
// steady-state public runtime for the canonical value-type benchmark shape.
func BenchmarkBaseline_Controlled_ARCORISPool_Value(b *testing.B) {
	benchmarkBaselineControlledPoolValue(b)
}

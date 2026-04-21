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

import "testing"

// This file contains grouped comparison benchmarks.
//
// The compare suite exists to produce stable, human-readable sub-benchmark
// blocks that make it easy to generate focused before/after reports. It does
// not replace the more explicit baseline, path, shape, or parallel files.
// Instead, it groups related strategies under one top-level benchmark name so
// benchmark output is easier to compare and chart.
//
// This file intentionally contains no independent benchmark logic. Every group
// is a thin surface over benchmark bodies defined in the owning benchmark file.

// BenchmarkCompare_PointerBaselines groups the three pointer-like baseline
// strategies under one top-level benchmark name.
func BenchmarkCompare_PointerBaselines(b *testing.B) {
	b.Run("realistic-alloc-only", benchmarkBaselineAllocOnlyPointer)
	b.Run("controlled-raw-sync-pool", benchmarkBaselineControlledRawSyncPoolPointer)
	b.Run("controlled-arcoris-pool", benchmarkBaselineControlledPoolPointer)
}

// BenchmarkCompare_ValueBaselines groups the three value-oriented baseline
// strategies under one top-level benchmark name.
func BenchmarkCompare_ValueBaselines(b *testing.B) {
	b.Run("realistic-alloc-only", benchmarkBaselineAllocOnlyValue)
	b.Run("controlled-raw-sync-pool", benchmarkBaselineControlledRawSyncPoolValue)
	b.Run("controlled-arcoris-pool", benchmarkBaselineControlledPoolValue)
}

// BenchmarkCompare_LifecyclePaths groups the controlled and realistic serial
// lifecycle-path categories under one top-level benchmark name.
func BenchmarkCompare_LifecyclePaths(b *testing.B) {
	b.Run("controlled-accepted", benchmarkPathsControlledAccepted)
	b.Run("realistic-accepted", benchmarkPathsRealisticAccepted)
	b.Run("realistic-rejected", benchmarkPathsRealisticRejected)
	b.Run("controlled-reset-heavy", benchmarkPathsControlledResetHeavy)
	b.Run("realistic-drop-observed", benchmarkPathsRealisticDropObserved)
}

// BenchmarkCompare_Shapes groups the canonical shape-sensitivity benchmarks.
func BenchmarkCompare_Shapes(b *testing.B) {
	b.Run("controlled-pointer-small", benchmarkControlledShapePointerSmall)
	b.Run("controlled-pointer-with-slices", benchmarkControlledShapePointerWithSlices)
	b.Run("controlled-value-small", benchmarkControlledShapeValueSmall)
	b.Run("controlled-value-large", benchmarkControlledShapeValueLarge)
	b.Run("realistic-always-oversized-rejected", benchmarkShapeAlwaysOversizedRejected)
}

// BenchmarkCompare_Parallel groups the realistic parallel benchmark family.
func BenchmarkCompare_Parallel(b *testing.B) {
	b.Run("realistic-accepted", benchmarkRealisticParallelAccepted)
	b.Run("realistic-rejected", benchmarkRealisticParallelRejected)
	b.Run("realistic-raw-sync-pool", benchmarkRealisticParallelRawSyncPool)
	b.Run("realistic-arcoris-pool", benchmarkRealisticParallelARCORISPool)
}

// BenchmarkCompare_Metrics groups the controlled and realistic metric-oriented
// serial benchmarks.
func BenchmarkCompare_Metrics(b *testing.B) {
	b.Run("controlled-accepted-warm-path", benchmarkMetricsControlledAcceptedWarmPath)
	b.Run("realistic-rejected-steady-state", benchmarkMetricsRealisticRejectedSteadyState)
	b.Run("realistic-mixed-reuse", benchmarkMetricsRealisticMixedReuse)
}

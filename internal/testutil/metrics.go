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

import "testing"

const (
	// MetricNewsPerOp is the canonical custom metric name for constructor events
	// observed per benchmark iteration.
	MetricNewsPerOp = "news/op"

	// MetricDropsPerOp is the canonical custom metric name for explicit
	// reuse-denial drop events observed per benchmark iteration.
	MetricDropsPerOp = "drops/op"

	// MetricReuseDenialsPerOp is the canonical custom metric name for reuse
	// policy denials observed per benchmark iteration.
	MetricReuseDenialsPerOp = "reuse_denials/op"
)

// ReportPerOpMetric publishes a total counter as a per-iteration benchmark
// metric.
//
// The helper keeps custom metric formatting consistent across the benchmark
// suite. It intentionally does nothing for empty benchmark runs.
func ReportPerOpMetric(b *testing.B, total uint64, unit string) {
	b.Helper()
	if b.N == 0 {
		return
	}
	b.ReportMetric(float64(total)/float64(b.N), unit)
}

// ReportLifecycleMetrics publishes the repository's three canonical pool
// counters for one benchmark run.
//
// Use this helper when a benchmark intentionally observes constructor events,
// explicit drops, and reuse denials together. It keeps both metric names and
// reporting order stable across the suite and across generated reports.
func ReportLifecycleMetrics(b *testing.B, news uint64, drops uint64, reuseDenials uint64) {
	b.Helper()

	ReportPerOpMetric(b, news, MetricNewsPerOp)
	ReportPerOpMetric(b, drops, MetricDropsPerOp)
	ReportPerOpMetric(b, reuseDenials, MetricReuseDenialsPerOp)
}

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

func TestReportPerOpMetric(t *testing.T) {
	t.Run("publishes a stable per operation value", func(t *testing.T) {
		result := testing.Benchmark(func(b *testing.B) {
			// Make the total proportional to b.N so the expected reported value
			// stays constant regardless of how many iterations testing chooses.
			ReportPerOpMetric(b, uint64(2*b.N), MetricNewsPerOp)
		})

		got, ok := result.Extra[MetricNewsPerOp]
		if !ok {
			t.Fatalf("ReportPerOpMetric did not publish %q", MetricNewsPerOp)
		}
		if got != 2 {
			t.Fatalf("reported %q metric = %v, want 2", MetricNewsPerOp, got)
		}
	})
}

func TestReportLifecycleMetrics(t *testing.T) {
	t.Run("publishes the canonical lifecycle metrics together", func(t *testing.T) {
		result := testing.Benchmark(func(b *testing.B) {
			// Each counter is scaled from b.N so the expected /op result is exact
			// and independent of the harness-selected iteration count.
			ReportLifecycleMetrics(b, uint64(b.N), uint64(2*b.N), uint64(3*b.N))
		})

		if got := result.Extra[MetricNewsPerOp]; got != 1 {
			t.Fatalf("reported %q metric = %v, want 1", MetricNewsPerOp, got)
		}
		if got := result.Extra[MetricDropsPerOp]; got != 2 {
			t.Fatalf("reported %q metric = %v, want 2", MetricDropsPerOp, got)
		}
		if got := result.Extra[MetricReuseDenialsPerOp]; got != 3 {
			t.Fatalf("reported %q metric = %v, want 3", MetricReuseDenialsPerOp, got)
		}
	})
}

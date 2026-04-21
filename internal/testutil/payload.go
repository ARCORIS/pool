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

var (
	benchmarkSamplePayload    = [...]byte{1, 2, 3, 4}
	benchmarkOversizedPayload = [8192]byte{}
)

// AppendSamplePayload appends the repository's canonical small benchmark
// payload to dst.
//
// Several benchmark families mutate a pooled byte slice with a short,
// allocation-free payload. Centralizing that payload keeps the suite
// consistent and removes repeated literal byte sequences from multiple files.
func AppendSamplePayload(dst []byte) []byte {
	return append(dst, benchmarkSamplePayload[:]...)
}

// AppendOversizedPayload appends the canonical oversized benchmark payload to
// dst.
//
// The helper intentionally uses a shared zero-value array rather than
// allocating a temporary source slice on every call. This keeps oversized-path
// benchmarks focused on target growth and reuse policy instead of measuring
// incidental source-slice allocations.
func AppendOversizedPayload(dst []byte) []byte {
	return append(dst, benchmarkOversizedPayload[:]...)
}

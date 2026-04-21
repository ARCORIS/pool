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

import (
	"slices"
	"testing"
)

func TestAppendSamplePayload(t *testing.T) {
	t.Run("appends the canonical small payload", func(t *testing.T) {
		got := AppendSamplePayload([]byte{9})
		want := []byte{9, 1, 2, 3, 4}

		if !slices.Equal(got, want) {
			t.Fatalf("AppendSamplePayload result = %v, want %v", got, want)
		}
	})
}

func TestAppendOversizedPayload(t *testing.T) {
	t.Run("appends the canonical oversized payload without changing the prefix", func(t *testing.T) {
		got := AppendOversizedPayload([]byte{7, 8})

		if len(got) != 2+8192 {
			t.Fatalf("AppendOversizedPayload length = %d, want %d", len(got), 2+8192)
		}
		if got[0] != 7 || got[1] != 8 {
			t.Fatalf("AppendOversizedPayload prefix = %v, want [7 8]", got[:2])
		}
		for i, b := range got[2:] {
			if b != 0 {
				t.Fatalf("AppendOversizedPayload byte %d after prefix = %d, want 0", i, b)
			}
		}
	})
}

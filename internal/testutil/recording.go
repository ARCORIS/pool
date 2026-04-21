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

// RecordingSink is a generic Put recorder for tests that need to observe the
// final storage step of a value.
//
// When Events is non-nil, Put appends a literal "put" marker to that event
// log before storing the value in Puts.
type RecordingSink[T any] struct {
	Events *[]string
	Puts   []T
}

// Put records the final storage step of a value.
//
// The method intentionally mirrors the minimal sink contract used by lifecycle
// tests: append an optional "put" event marker first, then retain the value in
// Puts for later inspection.
func (s *RecordingSink[T]) Put(value T) {
	if s.Events != nil {
		*s.Events = append(*s.Events, "put")
	}
	s.Puts = append(s.Puts, value)
}

// Copyright 2015 The appc Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import "testing"

func TestCRILabelsAssertValid(t *testing.T) {
	tests := []struct {
		in   CRILabels
		werr bool
	}{
		// empty is OK
		{
			CRILabels{},
			false,
		},
		{
			CRILabels{"a": "b"},
			false,
		},
		{
			CRILabels{"a!": "b"},
			true,
		},
		{
			CRILabels{"/a": "b"},
			true,
		},
		{
			CRILabels{"a/a": "b"},
			false,
		},
	}
	for i, tt := range tests {
		err := tt.in.assertValid()
		if gerr := (err != nil); gerr != tt.werr {
			t.Errorf("#%d: gerr=%t, want %t (err=%v)", i, gerr, tt.werr, err)
		}
	}
}

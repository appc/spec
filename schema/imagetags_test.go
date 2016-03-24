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

package schema

import "testing"

func TestImageTags(t *testing.T) {
	tj := `
	{
		"aliases": {
			"latest": "3.x",
			"3.x": "3.0.x",
			"3.0.x": "3.0.1",
			"3.0.1": "3.0.1-2"
		},
		"labels": {
			"3.0.1-2" : { "version": "3.0.1", "build": "2" },
			"3.0.1-3" : { "version": "3.0.1", "build": "3" }
		}
	}
	`

	var imageTags ImageTags

	err := imageTags.UnmarshalJSON([]byte(tj))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

}

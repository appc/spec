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

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestLabels(t *testing.T) {
	tests := []struct {
		in           string
		goOS         string
		goArch       string
		goArchFlavor string
		errPrefix    string
	}{
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "amd64"}]`,
			"linux",
			"amd64",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "aarch64"}]`,
			"linux",
			"arm64",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "arm64"}]`,
			"",
			"",
			"",
			`bad arch "arm64" for linux`,
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "aarch64_be"}]`,
			"",
			"",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "arm64_be"}]`,
			"",
			"",
			"",
			`bad arch "arm64_be" for linux`,
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "arm"}]`,
			"",
			"",
			"",
			`bad arch "arm" for linux`,
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "armv6l"}]`,
			"linux",
			"arm",
			"6",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "armv7l"}]`,
			"linux",
			"arm",
			"7",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "armv7b"}]`,
			"",
			"",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "ppc64le"}]`,
			"linux",
			"ppc64le",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "ppc64l"}]`,
			"",
			"",
			"",
			`bad arch "ppc64l" for linux`,
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "ppc64"}]`,
			"linux",
			"ppc64",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "ppc64b"}]`,
			"",
			"",
			"",
			`bad arch "ppc64b" for linux`,
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "ppc64be"}]`,
			"",
			"",
			"",
			`bad arch "ppc64be" for linux`,
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "s390x"}]`,
			"linux",
			"s390x",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "arch", "value": "S390x"}]`,
			"",
			"",
			"",
			`bad arch "S390x" for linux`,
		},
		{
			`[{"name": "os", "value": "freebsd"}, {"name": "arch", "value": "amd64"}]`,
			"freebsd",
			"amd64",
			"",
			"",
		},
		{
			`[{"name": "os", "value": "OS/360"}, {"name": "arch", "value": "S/360"}]`,
			"",
			"",
			"",
			`bad os "OS/360"`,
		},
		{
			`[{"name": "os", "value": "freebsd"}, {"name": "arch", "value": "armv7b"}]`,
			"",
			"",
			"",
			`bad arch "armv7b" for freebsd`,
		},
		{
			`[{"name": "name"}]`,
			"",
			"",
			"",
			`invalid label name: "name"`,
		},
		{
			`[{"name": "os", "value": "linux"}, {"name": "os", "value": "freebsd"}]`,
			"",
			"",
			"",
			`duplicate labels of name "os"`,
		},
		{
			`[{"name": "arch", "value": "amd64"}, {"name": "os", "value": "freebsd"}, {"name": "arch", "value": "x86_64"}]`,
			"",
			"",
			"",
			`duplicate labels of name "arch"`,
		},
		{
			`[]`,
			"",
			"",
			"",
			"",
		},
	}
	for i, tt := range tests {
		var l Labels
		if err := json.Unmarshal([]byte(tt.in), &l); err != nil {
			if tt.errPrefix == "" {
				t.Errorf("#%d: got err=%v, expected no error", i, err)
			} else if !strings.HasPrefix(err.Error(), tt.errPrefix) {
				t.Errorf("#%d: got err=%v, expected prefix %#v", i, err, tt.errPrefix)
			}
		} else {
			t.Log(l)
			if tt.errPrefix != "" {
				t.Errorf("#%d: got no err, expected prefix %#v", i, tt.errPrefix)
			}
			jsonOs, _ := l.Get("os")
			jsonArch, _ := l.Get("arch")
			os, arch, flavor, _ := ToGoOSArch(jsonOs, jsonArch)
			if os != tt.goOS {
				t.Errorf("#%d: got os %q, expected os %q", i, os, tt.goOS)

			}
			if arch != tt.goArch {
				t.Errorf("#%d: got arch %q, expected arch %q", i, arch, tt.goArch)

			}
			if flavor != tt.goArchFlavor {
				t.Errorf("#%d: got flavor %q, expected flavor %q", i, flavor, tt.goArchFlavor)

			}
		}
	}
}

func TestLabelsFromMap(t *testing.T) {
	tests := []struct {
		in          map[ACIdentifier]string
		expectedOut Labels
		expectedErr error
	}{
		{
			in: map[ACIdentifier]string{
				"foo": "bar",
				"bar": "baz",
				"baz": "foo",
			},
			expectedOut: []Label{
				Label{
					Name:  "bar",
					Value: "baz",
				},
				Label{
					Name:  "baz",
					Value: "foo",
				},
				Label{
					Name:  "foo",
					Value: "bar",
				},
			},
		},
		{
			in: map[ACIdentifier]string{
				"foo": "",
			},
			expectedOut: []Label{
				Label{
					Name:  "foo",
					Value: "",
				},
			},
		},
		{
			in: map[ACIdentifier]string{
				"name": "foo",
			},
			expectedErr: errors.New(`invalid label name: "name"`),
		},
	}

	for i, test := range tests {
		out, err := LabelsFromMap(test.in)
		if err != nil {
			if err.Error() != test.expectedErr.Error() {
				t.Errorf("case %d: expected %v = %v", i, err, test.expectedErr)
			}
			continue
		}
		if test.expectedErr != nil {
			t.Errorf("case %d: expected error %v, but got none", i, test.expectedErr)
			continue
		}
		if !reflect.DeepEqual(test.expectedOut, out) {
			t.Errorf("case %d: expected %v = %v", i, out, test.expectedOut)
		}
	}
}

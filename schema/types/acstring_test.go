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
	"reflect"
	"strings"
	"testing"
)

var (
	goodStrings = []string{
		"asdf",
		"asdf !*'();@&+$/?#[]¼µß",
		// 255 char length string
		strings.Repeat("a", 255),
	}

	badStrings = []string{
		"asdf\t",
		"asdf\n",
		// 256 char length string
		strings.Repeat("a", 256),
	}
)

func TestNewACString(t *testing.T) {
	for i, in := range goodStrings {
		l, err := NewACString(in)
		if err != nil {
			t.Errorf("#%d: got err=%v, want nil", i, err)
		}
		if l == nil {
			t.Errorf("#%d: got l=nil, want non-nil", i)
		}
	}
}

func TestNewACStringBad(t *testing.T) {
	for i, in := range badStrings {
		l, err := NewACString(in)
		if l != nil {
			t.Errorf("#%d: got l=%v, want nil", i, l)
		}
		if err == nil {
			t.Errorf("#%d: got err=nil, want non-nil", i)
		}
	}
}

func TestMustACString(t *testing.T) {
	for i, in := range goodStrings {
		l := MustACString(in)
		if l == nil {
			t.Errorf("#%d: got l=nil, want non-nil", i)
		}
	}
}

func expectPanicMustACString(i int, in string, t *testing.T) {
	defer func() {
		recover()
	}()
	_ = MustACString(in)
	t.Errorf("#%d: panic expected", i)
}

func TestMustACStringBad(t *testing.T) {
	for i, in := range badStrings {
		expectPanicMustACString(i, in, t)
	}
}

func TestACStringSetGood(t *testing.T) {
	tests := map[string]ACString{
		"asdf !*'();@&+$/?#[]¼µß": ACString("asdf !*'();@&+$/?#[]¼µß"),
	}
	for in, w := range tests {
		// Ensure an empty name is set appropriately
		var a ACString
		err := a.Set(in)
		if err != nil {
			t.Errorf("%v: got err=%v, want nil", in, err)
			continue
		}
		if !reflect.DeepEqual(a, w) {
			t.Errorf("%v: a=%v, want %v", in, a, w)
		}

		// Ensure an existing name is overwritten
		var b ACString = ACString("orig")
		err = b.Set(in)
		if err != nil {
			t.Errorf("%v: got err=%v, want nil", in, err)
			continue
		}
		if !reflect.DeepEqual(b, w) {
			t.Errorf("%v: b=%v, want %v", in, b, w)
		}
	}
}

func TestACStringSetBad(t *testing.T) {
	for i, in := range badStrings {
		// Ensure an empty name stays empty
		var a ACString
		err := a.Set(in)
		if err == nil {
			t.Errorf("#%d: err=%v, want nil", i, err)
			continue
		}
		if w := ACString(""); !reflect.DeepEqual(a, w) {
			t.Errorf("%d: a=%v, want %v", i, a, w)
		}

		// Ensure an existing name is not overwritten
		var b ACString = ACString("orig")
		err = b.Set(in)
		if err == nil {
			t.Errorf("#%d: err=%v, want nil", i, err)
			continue
		}
		if w := ACString("orig"); !reflect.DeepEqual(b, w) {
			t.Errorf("%d: b=%v, want %v", i, b, w)
		}
	}
}

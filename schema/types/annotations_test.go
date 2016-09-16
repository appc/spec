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
	"testing"
)

func TestAnnotationsAssertValid(t *testing.T) {
	tests := []struct {
		in   []Annotation
		werr bool
	}{
		// duplicate names should fail
		{
			[]Annotation{
				Annotation{"foo", "bar"},
				Annotation{"foo", "baz"},
			},
			true,
		},
		// bad created should fail
		{
			[]Annotation{
				Annotation{"created", "garbage"},
			},
			true,
		},
		// bad homepage should fail
		{
			[]Annotation{
				Annotation{"homepage", "not-A$@#URL"},
			},
			true,
		},
		// bad documentation should fail
		{
			[]Annotation{
				Annotation{"documentation", "ftp://isnotallowed.com"},
			},
			true,
		},
		// good cases
		{
			[]Annotation{
				Annotation{"created", "2004-05-14T23:11:14+00:00"},
				Annotation{"documentation", "http://example.com/docs"},
			},
			false,
		},
		{
			[]Annotation{
				Annotation{"foo", "bar"},
				Annotation{"homepage", "https://homepage.com"},
			},
			false,
		},
		{
			[]Annotation{
				Annotation{"appc.io/executor/supports-systemd-notify", "false"},
			},
			false,
		},
		// empty is OK
		{
			[]Annotation{},
			false,
		},
	}
	for i, tt := range tests {
		a := Annotations(tt.in)
		err := a.assertValid()
		if gerr := (err != nil); gerr != tt.werr {
			t.Errorf("#%d: gerr=%t, want %t (err=%v)", i, gerr, tt.werr, err)
		}
	}
}

func TestAnnotationsMarshal(t *testing.T) {
	for i, tt := range []struct {
		in   []Annotation
		wb   []byte
		werr bool
	}{
		{
			[]Annotation{
				Annotation{"foo", "bar"},
				Annotation{"foo", "baz"},
				Annotation{"website", "http://example.com/anno"},
			},
			nil,
			true,
		},
		{
			[]Annotation{
				Annotation{"a", "b"},
			},
			[]byte(`[{"name":"a","value":"b"}]`),
			false,
		},
		{
			[]Annotation{
				Annotation{"foo", "bar"},
				Annotation{"website", "http://example.com/anno"},
			},
			[]byte(`[{"name":"foo","value":"bar"},{"name":"website","value":"http://example.com/anno"}]`),
			false,
		},
	} {
		a := Annotations(tt.in)
		b, err := a.MarshalJSON()
		if !reflect.DeepEqual(b, tt.wb) {
			t.Errorf("#%d: b=%s, want %s", i, b, tt.wb)
		}
		gerr := err != nil
		if gerr != tt.werr {
			t.Errorf("#%d: gerr=%t, want %t (err=%v)", i, gerr, tt.werr, err)
		}
	}
}

func TestAnnotationsUnmarshal(t *testing.T) {
	tests := []struct {
		in   string
		wann *Annotations
		werr bool
	}{
		{
			`garbage`,
			&Annotations{},
			true,
		},
		{
			`[{"name":"a","value":"b"},{"name":"a","value":"b"}]`,
			&Annotations{},
			true,
		},
		{
			`[{"name":"a","value":"b"}]`,
			&Annotations{
				Annotation{"a", "b"},
			},
			false,
		},
	}
	for i, tt := range tests {
		a := &Annotations{}
		err := a.UnmarshalJSON([]byte(tt.in))
		gerr := err != nil
		if gerr != tt.werr {
			t.Errorf("#%d: gerr=%t, want %t (err=%v)", i, gerr, tt.werr, err)
		}
		if !reflect.DeepEqual(a, tt.wann) {
			t.Errorf("#%d: ann=%#v, want %#v", i, a, tt.wann)
		}
	}

}

func TestAnnotationsGet(t *testing.T) {
	for i, tt := range []struct {
		in   string
		wval string
		wok  bool
	}{
		{"foo", "bar", true},
		{"website", "http://example.com/anno", true},
		{"baz", "", false},
		{"wuuf", "", false},
	} {
		a := Annotations{
			Annotation{"foo", "bar"},
			Annotation{"website", "http://example.com/anno"},
		}
		gval, gok := a.Get(tt.in)
		if gval != tt.wval {
			t.Errorf("#%d: val=%v, want %v", i, gval, tt.wval)
		}
		if gok != tt.wok {
			t.Errorf("#%d: ok=%t, want %t", i, gok, tt.wok)
		}
	}
}

func TestAnnotationsSet(t *testing.T) {
	a := Annotations{}

	a.Set("foo", "bar")
	w := Annotations{
		Annotation{"foo", "bar"},
	}
	if !reflect.DeepEqual(w, a) {
		t.Fatalf("want %v, got %v", w, a)
	}

	a.Set("dog", "woof")
	w = Annotations{
		Annotation{"foo", "bar"},
		Annotation{"dog", "woof"},
	}
	if !reflect.DeepEqual(w, a) {
		t.Fatalf("want %v, got %v", w, a)
	}

	a.Set("foo", "baz")
	a.Set("example.com/foo_bar", "quux")
	w = Annotations{
		Annotation{"foo", "baz"},
		Annotation{"dog", "woof"},
		Annotation{"example.com/foo_bar", "quux"},
	}
	if !reflect.DeepEqual(w, a) {
		t.Fatalf("want %v, got %v", w, a)
	}
}

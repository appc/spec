package types

import "testing"

func TestNewACName(t *testing.T) {
	tests := []string{
		"asdf",
		"foo-bar-baz",
		"database",
		"example.com/database",
		"example.com/ourapp-1.0.0",
		"sub-domain.example.com/org/product/release-1.0.0",
	}
	for i, in := range tests {
		l, err := NewACName(in)
		if err != nil {
			t.Errorf("#%d: got err=%v, want nil", i, err)
		}
		if l == nil {
			t.Errorf("#%d: got l=nil, want non-nil", i)
		}
	}
}

func TestNewACNameBad(t *testing.T) {
	tests := []string{
		"",
		"foo#",
		"EXAMPLE.com",
		"foo.com/BAR",
		"example.com/app_1",
		"/app",
		"app/",
		"-app",
		"app-",
		".app",
		"app.",
	}
	for i, in := range tests {
		l, err := NewACName(in)
		if l != nil {
			t.Errorf("#%d: got l=%v, want nil", i, l)
		}
		if err == nil {
			t.Errorf("#%d: got err=nil, want non-nil", i)
		}
	}
}

func TestSanitizeACName(t *testing.T) {
	tests := map[string]string{
		"foo#":                                             "foo",
		"EXAMPLE.com":                                      "example.com",
		"foo.com/BAR":                                      "foo.com/bar",
		"example.com/app_1":                                "example.com/app-1",
		"/app":                                             "app",
		"app/":                                             "app",
		"-app":                                             "app",
		"app-":                                             "app",
		".app":                                             "app",
		"app.":                                             "app",
		"app///":                                           "app",
		"-/.app..":                                         "app",
		"-/app.name-test/-/":                               "app.name-test",
		"sub-domain.example.com/org/product/release-1.0.0": "sub-domain.example.com/org/product/release-1.0.0",
	}
	for in, ex := range tests {
		o, err := SanitizeACName(in)
		if err != nil {
			t.Errorf("got err=%v, want nil", err)
		}
		if o != ex {
			t.Errorf("got l=%s, want %s", o, ex)
		}
	}
}

func TestSanitizeACNameBad(t *testing.T) {
	tests := []string{
		"__",
		"..",
		"//",
		"",
		".//-"}
	for i, in := range tests {
		l, err := SanitizeACName(in)
		if l != "" {
			t.Errorf("#%d: got l=%v, want nil", i, l)
		}
		if err == nil {
			t.Errorf("#%d: got err=nil, want non-nil", i)
		}
	}
}

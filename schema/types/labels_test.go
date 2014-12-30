package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLabels(t *testing.T) {
	tests := []struct {
		in        string
		errPrefix string
	}{
		{
			`[{"name": "os", "val": "linux"}, {"name": "arch", "val": "amd64"}]`,
			"",
		},
		{
			`[{"name": "os", "val": "freebsd"}, {"name": "arch", "val": "amd64"}]`,
			"",
		},
		{
			`[{"name": "os", "val": "OS/360"}, {"name": "arch", "val": "S/360"}]`,
			`bad os "OS/360"`,
		},
		{
			`[{"name": "os", "val": "linux"}, {"name": "arch", "val": "arm"}]`,
			`bad arch "arm" for linux`,
		},
		{
			`[{"name": "os", "val": "linux"}, {"name": "os", "val": "freebsd"}]`,
			`duplicate labels of name "os"`,
		},
		{
			`[{"name": "arch", "val": "amd64"}, {"name": "os", "val": "freebsd"}, {"name": "arch", "val": "x86_64"}]`,
			`duplicate labels of name "arch"`,
		},
		{
			`[]`,
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
		}
	}
}

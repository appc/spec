package types

import "testing"

func TestAppValid(t *testing.T) {
	tests := []App{
		App{
			Exec:  []string{"/bin/httpd"},
			User:  "0",
			Group: "0",
		},
		App{
			Exec:  []string{"/app"},
			User:  "0",
			Group: "0",
		},
		App{
			Exec:  []string{"/app", "arg1", "arg2"},
			User:  "0",
			Group: "0",
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err != nil {
			t.Errorf("#%d: err == %v, want nil", i, err)
		}
	}
}

func TestAppInvalid(t *testing.T) {
	tests := []App{
		App{
			Exec: nil,
		},
		App{
			Exec:  []string{},
			User:  "0",
			Group: "0",
		},
		App{
			Exec:  []string{"app"},
			User:  "0",
			Group: "0",
		},
		App{
			Exec:  []string{"bin/app", "arg1"},
			User:  "0",
			Group: "0",
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}

func TestUserGroupInvalid(t *testing.T) {
	tests := []App{
		App{
			Exec: []string{"/app"},
		},
		App{
			Exec: []string{"/app"},
			User: "0",
		},
		App{
			Exec:  []string{"app"},
			Group: "0",
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}

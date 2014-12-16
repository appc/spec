package types

import "testing"

func TestAppValid(t *testing.T) {
	tests := []App{
		App{
			Exec: []string{"/bin/httpd"},
		},
		App{
			Exec: []string{"/app"},
		},
		App{
			Exec: []string{"/app", "arg1", "arg2"},
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
			Exec: []string{},
		},
		App{
			Exec: []string{"app"},
		},
		App{
			Exec: []string{"bin/app", "arg1"},
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}

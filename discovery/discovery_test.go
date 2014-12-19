package discovery

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

func fakeHTTPGet(filename string, failures int) func(uri string) (*http.Response, error) {
	attempts := 0
	return func(uri string) (*http.Response, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}

		var resp *http.Response

		switch {
		case attempts < failures:
			resp = &http.Response{
				Status:     "404 Not Found",
				StatusCode: http.StatusNotFound,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}
		default:
			resp = &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: f,
			}
		}

		attempts = attempts + 1
		return resp, nil
	}
}

type httpgetter func(uri string) (*http.Response, error)

func TestDiscoverEndpoints(t *testing.T) {
	tests := []struct {
		get httpgetter
		expectDiscoverySuccess bool
		app App
	}{
		{
			fakeHTTPGet("myapp.html", 0),
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
		},
		{
			fakeHTTPGet("myapp.html", 1),
			true,
			App{
				Name: "example.com/myapp/foobar",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
		},
		{
			fakeHTTPGet("myapp.html", 20),
			false,
			App{
				Name: "example.com/myapp/foobar/bazzer",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
		},
	}

	for i, tt := range tests {
		httpGet = tt.get
		de, err := DiscoverEndpoints(tt.app, true)
		if err != nil && !tt.expectDiscoverySuccess {
			continue
		}
		if err != nil {
			t.Fatalf("#%d DiscoverEndpoints failed: %v", i, err)
		}

		if len(de.Sig) != 2 {
			t.Errorf("Sig array is wrong length want %d got %d", 2, len(de.Sig))
		} else {
			tor := fmt.Sprintf("https://storage.example.com/%s-%s.sig?torrent", tt.app.Name, tt.app.Labels["version"])
			if de.Sig[0] != tor {
				t.Errorf("#%d sig[0] mismatch: want %v got %v", i, tor, de.Sig[0])
			}
			hdfs := fmt.Sprintf("hdfs://storage.example.com/%s-%s.sig", tt.app.Name, tt.app.Labels["version"])
			if de.Sig[1] != hdfs {
				t.Errorf("#%d sig[1] mismatch want %v got %v", i, hdfs, de.Sig[0])
			}
		}

		if len(de.ACI) != 2 {
			t.Errorf("ACI array is wrong length")
		} else {
			tor := fmt.Sprintf("https://storage.example.com/%s-%s.aci?torrent", tt.app.Name, tt.app.Labels["version"])
			if de.ACI[0] != tor {
				t.Errorf("#%d ACI[0] mismatch want %v got %v", i, tor, de.ACI[0])
			}
			hdfs := fmt.Sprintf("hdfs://storage.example.com/%s-%s.aci", tt.app.Name, tt.app.Labels["version"])
			if de.ACI[1] != hdfs {
				t.Errorf("#%d ACI[1] mismatch want %v got %v", i, hdfs, de.ACI[1])
			}
		}

		if len(de.Keys) != 1 {
			t.Errorf("Keys array is wrong length")
		} else {
			if de.Keys[0] != "https://example.com/pubkeys.gpg" {
				t.Error("Keys[0] mismatch: ", de.Keys[0])
			}
		}
	}
}

package discovery

import (
	"bytes"
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
		get                          httpgetter
		expectSimpleDiscoverySuccess bool
		expectMetaDiscoverySuccess   bool
		app                          App
		expectedSimpleACIEndpoints   []ACIEndpoint
		expectedMetaACIEndpoints     []ACIEndpoint
		expectedMetaKeys             []string
	}{
		{
			fakeHTTPGet("myapp.html", 0),
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://example.com/myapp-1.0.0-linux-amd64.aci",
					Sig: "https://example.com/myapp-1.0.0-linux-amd64.sig",
				},
				ACIEndpoint{
					ACI: "http://example.com/myapp-1.0.0-linux-amd64.aci",
					Sig: "http://example.com/myapp-1.0.0-linux-amd64.sig",
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0.aci?torrent",
					Sig: "https://storage.example.com/example.com/myapp-1.0.0.sig?torrent",
				},
				ACIEndpoint{
					ACI: "hdfs://storage.example.com/example.com/myapp-1.0.0.aci",
					Sig: "hdfs://storage.example.com/example.com/myapp-1.0.0.sig",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
		},
		{
			fakeHTTPGet("myapp.html", 1),
			true,
			true,
			App{
				Name: "example.com/myapp/foobar",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://example.com/myapp/foobar-1.0.0-linux-amd64.aci",
					Sig: "https://example.com/myapp/foobar-1.0.0-linux-amd64.sig",
				},
				ACIEndpoint{
					ACI: "http://example.com/myapp/foobar-1.0.0-linux-amd64.aci",
					Sig: "http://example.com/myapp/foobar-1.0.0-linux-amd64.sig",
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp/foobar-1.0.0.aci?torrent",
					Sig: "https://storage.example.com/example.com/myapp/foobar-1.0.0.sig?torrent",
				},
				ACIEndpoint{
					ACI: "hdfs://storage.example.com/example.com/myapp/foobar-1.0.0.aci",
					Sig: "hdfs://storage.example.com/example.com/myapp/foobar-1.0.0.sig",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
		},
		{
			fakeHTTPGet("myapp.html", 20),
			false,
			false,
			App{
				Name: "example.com/myapp/foobar/bazzer",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://example.com/myapp/foobar/bazzer-1.0.0-linux-amd64.aci",
					Sig: "https://example.com/myapp/foobar/bazzer-1.0.0-linux-amd64.sig",
				},
				ACIEndpoint{
					ACI: "http://example.com/myapp/foobar/bazzer-1.0.0-linux-amd64.aci",
					Sig: "http://example.com/myapp/foobar/bazzer-1.0.0-linux-amd64.sig",
				},
			},
			[]ACIEndpoint{},
			[]string{},
		},
		// Test with a label called "name". It should be ignored.
		{
			fakeHTTPGet("myapp.html", 0),
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[string]string{
					"name":    "labelcalledname",
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://example.com/myapp-1.0.0-linux-amd64.aci",
					Sig: "https://example.com/myapp-1.0.0-linux-amd64.sig",
				},
				ACIEndpoint{
					ACI: "http://example.com/myapp-1.0.0-linux-amd64.aci",
					Sig: "http://example.com/myapp-1.0.0-linux-amd64.sig",
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0.aci?torrent",
					Sig: "https://storage.example.com/example.com/myapp-1.0.0.sig?torrent",
				},
				ACIEndpoint{
					ACI: "hdfs://storage.example.com/example.com/myapp-1.0.0.aci",
					Sig: "hdfs://storage.example.com/example.com/myapp-1.0.0.sig",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
		},

		// Test missing label. Only one ac-discovery template should be
		// returned as the other one cannot be completely rendered due to
		// missing labels.
		{
			fakeHTTPGet("myapp2.html", 0),
			false,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[string]string{
					"version": "1.0.0",
				},
			},
			[]ACIEndpoint{},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0.aci",
					Sig: "https://storage.example.com/example.com/myapp-1.0.0.sig",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
		},
		// Test missing labels. version label should default to
		// "latest" and the first template should be rendered
		{
			fakeHTTPGet("myapp2.html", 0),
			false,
			true,
			App{
				Name:   "example.com/myapp",
				Labels: map[string]string{},
			},
			[]ACIEndpoint{},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-latest.aci",
					Sig: "https://storage.example.com/example.com/myapp-latest.sig",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
		},
	}

	for i, tt := range tests {
		for _, discoveryType := range []string{"simple", "meta"} {
			var expectedACIEndpoints []ACIEndpoint
			var expectedKeys []string
			var de *Endpoints
			var err error
			switch discoveryType {
			case "simple":
				expectedACIEndpoints = tt.expectedSimpleACIEndpoints
				expectedKeys = []string{}
				de, err = SimpleDiscoverEndpoints(tt.app, true)
				if err != nil && !tt.expectSimpleDiscoverySuccess {
					continue
				}
				if err != nil {
					t.Fatalf("#%d DiscoverEndpoints failed: %v", i, err)
				}
			case "meta":
				expectedACIEndpoints = tt.expectedMetaACIEndpoints
				expectedKeys = tt.expectedMetaKeys
				httpGet = tt.get
				de, err = MetaDiscoverEndpoints(tt.app, true)
				if err != nil && !tt.expectMetaDiscoverySuccess {
					continue
				}
				if err != nil {
					t.Fatalf("#%d DiscoverEndpoints failed: %v", i, err)
				}
			}

			if len(de.ACIEndpoints) != len(expectedACIEndpoints) {
				t.Errorf("#%d %s, ACIEndpoints array is wrong length want %d got %d", i, discoveryType, len(expectedACIEndpoints), len(de.ACIEndpoints))
			} else {
				for n, _ := range de.ACIEndpoints {
					if de.ACIEndpoints[n] != expectedACIEndpoints[n] {
						t.Errorf("#%d %s, ACIEndpoints[%d] mismatch: want %v got %v", i, discoveryType, n, expectedACIEndpoints[n], de.ACIEndpoints[n])
					}
				}
			}

			if len(de.Keys) != len(expectedKeys) {
				t.Errorf("#%d %s, Keys array is wrong length want %d got %d", i, discoveryType, len(expectedKeys), len(de.Keys))
			} else {
				for n, _ := range de.Keys {
					if de.Keys[n] != expectedKeys[n] {
						t.Errorf("#%d %s, Key[%d] mismatch: want %v got %v", i, discoveryType, n, expectedKeys[n], de.Keys[n])
					}
				}
			}
		}
	}
}

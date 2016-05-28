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

package discovery

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

type meta struct {
	path    string
	content string
}

func fakeHTTPGet(metas []meta, header http.Header) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		var err error
		var resp *http.Response

		if header != nil && !reflect.DeepEqual(req.Header, header) {
			err = fmt.Errorf("fakeHTTPGet: wrong header %v. Expected %v", req.Header, header)
			return nil, err
		}

		for _, meta := range metas {
			if req.URL.Path == meta.path {
				f, err := os.Open(filepath.Join("testdata", meta.content))
				if err != nil {
					return nil, err
				}

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
				break
			}
		}

		if resp == nil {
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
		}

		return resp, nil
	}
}

func TestDiscoverEndpoints(t *testing.T) {
	tests := []struct {
		do                                 httpDoer
		expectMergeTagSuccess              bool
		expectDiscoveryACIEndpointsSuccess bool
		expectDiscoveryPublicKeysSuccess   bool
		app                                App
		tags                               *schema.ImageTags
		expectedACIEndpoints               []ACIEndpoint
		expectedPublicKeys                 []string
		authHeader                         http.Header
	}{
		//
		// Tests for meta tag discovery. Suppose template matching works.
		//

		// Test discovery for ACIEndpoint and publicKeys should work
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta01.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test discovery for ACIEndpoint and publicKeys should work walking up
		// to parent paths
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{
							"",
							"meta01.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp/foobar",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp/foobar-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp/foobar-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test discovery for ACIEndpoint and publicKeys should fail due to
		// missing meta tags in any walked path
		{
			&mockHTTPDoer{
				// always fails
				doer: fakeHTTPGet(
					[]meta{
						{
							"/path/out/of/myapp",
							"meta01.html",
						},
					},
					nil,
				),
			},
			true,
			false,
			false,
			App{
				Name: "example.com/myapp/foobar/bazzer",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			nil,
			nil,
			nil,
		},

		// Test with only 'ac-discovery' at / and only
		// 'ac-discovery-pubkeys' at /myapp. Both ACIEndpoints and PublicKeys
		// discovery should work.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{
							"",
							"meta02.html",
						},
						{
							"/myapp",
							"meta03.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test with only 'ac-discovery-pubkeys' at / and only
		// 'ac-discovery' at /myapp. Both ACIEndpoints and PublicKeys discovery
		// should work.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{
							"",
							"meta03.html",
						},
						{
							"/myapp",
							"meta02.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test with only 'ac-discovery-pubkeys' at / . PublicKeys discovery should fail and ACIEndpoints should work.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{
							"",
							"meta02.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			false,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			nil,
			nil,
		},
		// Test with only 'ac-discovery' at / . ACIEndpoints discovery should fail and PublicKeys should work.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{
							"",
							"meta03.html",
						},
					},
					nil,
				),
			},
			true,
			false,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			nil,
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test with both 'ac-discovery' and 'ac-discovery-pubkeys' at
		// / and only 'ac-discovery' at /myapp.
		// ACIEndpoints discovery should work and use the template provided
		// by /myapp (ignoring /) and PublicKeys should work using the
		// template provided by /.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{
							"",
							"meta04.html",
						},
						{
							"/myapp",
							"meta02.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},

		//
		// Tests for template matching. Suppose meta tags are always returned.
		//

		// Test missing label. Only one ac-discovery template should be
		// returned as the other one cannot be completely rendered due to
		// missing labels.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta05.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test with a label called "name". It should be ignored.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta05.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"name":    "labelcalledname",
					"version": "1.0.0",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test multiple ACIEndpoints.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta06.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0.aci.asc",
				},
				ACIEndpoint{
					ACI: "hdfs://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "hdfs://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test tag alias
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta01.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Tag:  "latest",
				Labels: map[types.ACIdentifier]string{
					"os":   "linux",
					"arch": "amd64",
				},
			},
			&schema.ImageTags{
				Aliases: schema.TagAliases{
					"latest": "2.x",
				},
				Labels: schema.TagLabels{
					"2.x": map[types.ACIdentifier]string{
						"version": "2.0.0",
					},
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-2.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-2.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test tag alias should not override required version label
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta01.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Tag:  "latest",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			&schema.ImageTags{
				Aliases: schema.TagAliases{
					"latest": "2.x",
				},
				Labels: schema.TagLabels{
					"2.x": map[types.ACIdentifier]string{
						"version": "2.0.0",
					},
				},
			},
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test tag without image tags. Should set tag to version label.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta01.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Tag:  "latest",
				Labels: map[types.ACIdentifier]string{
					"os":   "linux",
					"arch": "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-latest-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-latest-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},
		// Test tag without image tags. Should set tag to version label but fail since version is already specified.
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta01.html",
						},
					},
					nil,
				),
			},
			false,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Tag:  "latest",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			nil,
		},

		// Test with an auth header
		{
			&mockHTTPDoer{
				doer: fakeHTTPGet(
					[]meta{
						{"/myapp",
							"meta01.html",
						},
					},
					nil,
				),
			},
			true,
			true,
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[types.ACIdentifier]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			nil,
			[]ACIEndpoint{
				ACIEndpoint{
					ACI: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci",
					ASC: "https://storage.example.com/example.com/myapp-1.0.0-linux-amd64.aci.asc",
				},
			},
			[]string{"https://example.com/pubkeys.gpg"},
			testAuthHeader,
		},
	}

	for i, tt := range tests {
		httpDo = tt.do
		httpDoInsecureTLS = tt.do
		var hostHeaders map[string]http.Header
		if tt.authHeader != nil {
			hostHeaders = map[string]http.Header{
				strings.Split(tt.app.String(), "/")[0]: tt.authHeader,
			}
		}
		insecureList := []InsecureOption{
			InsecureNone,
			InsecureTLS,
			InsecureHTTP,
			InsecureTLS | InsecureHTTP,
		}
		for _, insecure := range insecureList {
			// Expand App labels with tags info labels
			app, err := tt.app.MergeTag(tt.tags)
			if err != nil && !tt.expectMergeTagSuccess {
				continue
			}
			if err == nil && !tt.expectMergeTagSuccess {
				t.Fatalf("#%d MergeTag should have failed but didn't", i)
			}
			if err != nil {
				t.Fatalf("#%d MergeTag failed: %v", i, err)
			}

			eps, _, err := DiscoverACIEndpoints(*app, hostHeaders, insecure)
			if err != nil && !tt.expectDiscoveryACIEndpointsSuccess {
				continue
			}
			if err == nil && !tt.expectDiscoveryACIEndpointsSuccess {
				t.Fatalf("#%d DiscoverACIEndpoints should have failed but didn't", i)
			}
			if err != nil {
				t.Fatalf("#%d DiscoverACIEndpoints failed: %v", i, err)
			}

			publicKeys, _, err := DiscoverPublicKeys(*app, hostHeaders, insecure)
			if err != nil && !tt.expectDiscoveryPublicKeysSuccess {
				continue
			}
			if err == nil && !tt.expectDiscoveryPublicKeysSuccess {
				t.Fatalf("#%d DiscoverPublicKeys should have failed but didn't", i)
			}
			if err != nil {
				t.Fatalf("#%d DiscoverPublicKeys failed: %v", i, err)
			}

			if len(eps) != len(tt.expectedACIEndpoints) {
				t.Errorf("#%d ACIEndpoints array is wrong length want %d got %d", i, len(tt.expectedACIEndpoints), len(eps))
			} else {
				for n, _ := range eps {
					if eps[n] != tt.expectedACIEndpoints[n] {
						t.Errorf("#%d ACIEndpoints[%d] mismatch: want %v got %v", i, n, tt.expectedACIEndpoints[n], eps[n])
					}
				}
			}

			if len(publicKeys) != len(tt.expectedPublicKeys) {
				t.Errorf("#%d PublicKeys array is wrong length want %d got %d", i, len(tt.expectedPublicKeys), len(publicKeys))
			} else {
				for n, _ := range publicKeys {
					if publicKeys[n] != tt.expectedPublicKeys[n] {
						t.Errorf("#%d sig[%d] mismatch: want %v got %v", i, n, tt.expectedPublicKeys[n], publicKeys[n])
					}
				}
			}
		}
	}
}

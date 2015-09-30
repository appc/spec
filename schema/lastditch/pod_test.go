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

package lastditch

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestInvalidPodManifest(t *testing.T) {
	tests := []struct {
		desc     string
		json     string
		expected PodManifest
	}{
		{
			desc:     "Check an empty pod manifest",
			json:     podJ(appsJ(), ""),
			expected: podI(appsI()),
		},
		{
			desc:     "Check a pod manifest with an empty app",
			json:     podJ(appsJ(appJ("", rImgJ("i", "id", ""), "")), ""),
			expected: podI(appsI(appI("", rImgI("i", "id")))),
		},
		{
			desc:     "Check a pod manifest with an app based on an empty image",
			json:     podJ(appsJ(appJ("a", rImgJ("", "", ""), "")), ""),
			expected: podI(appsI(appI("a", rImgI("", "")))),
		},
		{
			desc:     "Check a pod manifest with an invalid app name",
			json:     podJ(appsJ(appJ("!", rImgJ("i", "id", ""), "")), ""),
			expected: podI(appsI(appI("!", rImgI("i", "id")))),
		},
		{
			desc:     "Check a pod manifest with duplicated app names",
			json:     podJ(appsJ(appJ("a", rImgJ("i", "id", ""), ""), appJ("a", rImgJ("", "", ""), "")), ""),
			expected: podI(appsI(appI("a", rImgI("i", "id")), appI("a", rImgI("", "")))),
		},
		{
			desc:     "Check a pod manifest with an invalid image name and ID",
			json:     podJ(appsJ(appJ("?", rImgJ("!!!", "&&&", ""), "")), ""),
			expected: podI(appsI(appI("?", rImgI("!!!", "&&&")))),
		},
		{
			desc:     "Check a pod manifest with some extra fields",
			json:     podJ(appsJ(), extJ("goblins")),
			expected: podI(appsI()),
		},
		{
			desc:     "Check a pod manifest with an app containing some extra fields",
			json:     podJ(appsJ(appJ("a", rImgJ("i", "id", ""), extJ("trolls"))), extJ("goblins")),
			expected: podI(appsI(appI("a", rImgI("i", "id")))),
		},
		{
			desc:     "Check a pod manifest with an app based on an image containing some extra fields",
			json:     podJ(appsJ(appJ("a", rImgJ("i", "id", extJ("stuff")), extJ("trolls"))), extJ("goblins")),
			expected: podI(appsI(appI("a", rImgI("i", "id")))),
		},
	}
	for _, tt := range tests {
		got := PodManifest{}
		if err := got.UnmarshalJSON([]byte(tt.json)); err != nil {
			t.Errorf("%s: unexpected error during unmarshalling pod manifest: %v", tt.desc, err)
		}
		if !reflect.DeepEqual(tt.expected, got) {
			t.Errorf("%s: did not get expected pod manifest, got:\n  %#v\nexpected:\n  %#v", tt.desc, got, tt.expected)
		}
	}
}

func TestBogusPodManifest(t *testing.T) {
	bogus := []string{
		`
			{
			    "acKind": "Bogus",
			    "acVersion": "0.7.0",
			}
			`,
		`
			<html>
			    <head>
				<title>Certainly not a JSON</title>
			    </head>
			</html>`,
	}

	for _, str := range bogus {
		pm := PodManifest{}
		if pm.UnmarshalJSON([]byte(str)) == nil {
			t.Errorf("bogus pod manifest unmarshalled successfully: %s", str)
		}
	}
}

// podJ returns a pod manifest JSON with given apps
func podJ(apps, extra string) string {
	return fmt.Sprintf(`
		{
		    %s
		    "acKind": "PodManifest",
		    "acVersion": "0.7.0",
		    "apps": %s
		}`, extra, apps)
}

// podI returns a pod manifest instance with given apps
func podI(apps AppList) PodManifest {
	return PodManifest{
		ACVersion: "0.7.0",
		ACKind:    "PodManifest",
		Apps:      apps,
	}
}

// appsJ returns an applist JSON snippet with given apps
func appsJ(apps ...string) string {
	return fmt.Sprintf("[%s]", strings.Join(apps, ","))
}

// appsI returns an applist instance with given apps
func appsI(apps ...RuntimeApp) AppList {
	if apps == nil {
		return AppList{}
	}
	return apps
}

// appJ returns an app JSON snippet with given name and image
func appJ(name, image, extra string) string {
	return fmt.Sprintf(`
		{
		    %s
		    "name": "%s",
		    "image": %s
		}`, extra, name, image)
}

// appI returns an app instance with given name and image
func appI(name string, image RuntimeImage) RuntimeApp {
	return RuntimeApp{
		Name:  name,
		Image: image,
	}
}

// rImgJ returns a runtime image JSON snippet with given name and id
func rImgJ(name, id, extra string) string {
	return fmt.Sprintf(`
		{
		    %s
		    "name": "%s",
		    "id": "%s"
		}`, extra, name, id)
}

// rImgI returns a runtime image instance with given name and id
func rImgI(name, id string) RuntimeImage {
	return RuntimeImage{
		Name: name,
		ID:   id,
	}
}

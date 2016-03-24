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

package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
)

var (
	outputJson  bool
	cmdDiscover = &Command{
		Name:        "discover",
		Description: "Discover the download URLs for an app",
		Summary:     "Discover the download URLs for one or more app container images",
		Usage:       "[--json] APP...",
		Run:         runDiscover,
	}
)

func init() {
	cmdDiscover.Flags.BoolVar(&transportFlags.Insecure, "insecure", false,
		"Don't check TLS certificates and allow insecure non-TLS downloads over http")
	cmdDiscover.Flags.BoolVar(&outputJson, "json", false,
		"Output result as JSON")
}

func runDiscover(args []string) (exit int) {
	if len(args) < 1 {
		stderr("discover: at least one name required")
	}

	for _, name := range args {
		app, err := discovery.NewAppFromString(name)
		if app.Labels["os"] == "" {
			app.Labels["os"] = runtime.GOOS
		}
		if app.Labels["arch"] == "" {
			app.Labels["arch"] = runtime.GOARCH
		}
		if err != nil {
			stderr("%s: %s", name, err)
			return 1
		}
		insecure := discovery.InsecureNone
		if transportFlags.Insecure {
			insecure = discovery.InsecureTLS | discovery.InsecureHTTP
		}
		tagsEndpoints, attempts, err := discovery.DiscoverImageTags(*app, nil, insecure)
		if err != nil {
			stderr("error fetching endpoints for %s: %s", name, err)
			return 1
		}
		for _, a := range attempts {
			fmt.Printf("discover tags walk: prefix: %s error: %v\n", a.Prefix, a.Error)
		}
		if len(tagsEndpoints) != 0 {
			tags, err := fetchImageTags(tagsEndpoints[0].ImageTags, insecure)
			if err != nil {
				stderr("error fetching tags info: %s", err)
				return 1
			}
			// Merge tag labels
			app, err = app.MergeTag(tags)
			if err != nil {
				stderr("error resolving tags to labels: %s", err)
				return 1
			}
		} else {
			fmt.Printf("no discover tags found")
		}

		eps, attempts, err := discovery.DiscoverACIEndpoints(*app, nil, insecure)
		if err != nil {
			stderr("error fetching endpoints for %s: %s", name, err)
			return 1
		}
		for _, a := range attempts {

			fmt.Printf("discover endpoints walk: prefix: %s error: %v\n", a.Prefix, a.Error)
		}
		publicKeys, attempts, err := discovery.DiscoverPublicKeys(*app, nil, insecure)
		if err != nil {
			stderr("error fetching public keys for %s: %s", name, err)
			return 1
		}
		for _, a := range attempts {
			fmt.Printf("discover public keys walk: prefix: %s error: %v\n", a.Prefix, a.Error)
		}

		type discoveryData struct {
			ACIEndpoints []discovery.ACIEndpoint
			PublicKeys   []string
		}

		if outputJson {
			dd := discoveryData{ACIEndpoints: eps, PublicKeys: publicKeys}
			jsonBytes, err := json.MarshalIndent(dd, "", "    ")
			if err != nil {
				stderr("error generating JSON: %s", err)
				return 1
			}
			fmt.Println(string(jsonBytes))
		} else {
			for _, aciEndpoint := range eps {
				fmt.Printf("ACI: %s, ASC: %s\n", aciEndpoint.ACI, aciEndpoint.ASC)
			}
			if len(publicKeys) > 0 {
				fmt.Println("PublicKeys: " + strings.Join(publicKeys, ","))
			}
		}
	}

	return
}

func fetchImageTags(urlStr string, insecure discovery.InsecureOption) (*schema.ImageTags, error) {
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: func(n, a string) (net.Conn, error) {
			return net.DialTimeout(n, a, 5*time.Second)
		},
	}
	if insecure&discovery.InsecureTLS != 0 {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{
		Transport: t,
	}

	fetch := func(scheme string) (res *http.Response, err error) {
		u, err := url.Parse(urlStr)
		if err != nil {
			return nil, err
		}
		u.Scheme = scheme
		urlStr := u.String()
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return nil, err
		}
		res, err = client.Do(req)
		return
	}
	closeBody := func(res *http.Response) {
		if res != nil {
			res.Body.Close()
		}
	}
	res, err := fetch("https")
	if err != nil || res.StatusCode != http.StatusOK {
		if insecure&discovery.InsecureHTTP != 0 {
			closeBody(res)
			res, err = fetch("http")
		}
	}

	if res != nil && res.StatusCode != http.StatusOK {
		err = fmt.Errorf("expected a 200 OK got %d", res.StatusCode)
	}

	if err != nil {
		closeBody(res)
		return nil, err
	}

	var tags *schema.ImageTags
	jd := json.NewDecoder(res.Body)
	jd.Decode(&tags)
	closeBody(res)

	return tags, nil
}

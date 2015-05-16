package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/appc/spec/discovery"
)

var (
	cmdDiscover = &Command{
		Name:        "discover",
		Description: "Discover the download URLs for an app",
		Summary:     "Discover the download URLs for one or more app container images",
		Usage:       "[--http-port PORT] [--https-port PORT] [--insecure] APP...",
		Run:         runDiscover,
	}
	flagHttpPort  uint
	flagHttpsPort uint
)

func init() {
	cmdDiscover.Flags.BoolVar(&transportFlags.Insecure, "insecure", false,
		"Allow insecure non-TLS downloads over http")
	cmdDiscover.Flags.UintVar(&flagHttpPort, "http-port", 0,
		"Port to connect when performing discovery using HTTP. If unset or set to 0, defaults to 80. Sets insecure.")
	cmdDiscover.Flags.UintVar(&flagHttpsPort, "https-port", 0,
		"Port to connect when performing discovery HTTPS. If unset or set to 0, defaults to 443.")
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
		if flagHttpPort != 0 {
			transportFlags.Insecure = true
		}
		eps, attempts, err := discovery.DiscoverEndpoints(*app, flagHttpPort, flagHttpsPort, transportFlags.Insecure)
		if err != nil {
			stderr("error fetching %s: %s", name, err)
			return 1
		}
		for _, a := range attempts {
			fmt.Printf("discover walk: prefix: %s error: %v\n", a.Prefix, a.Error)
		}
		for _, aciEndpoint := range eps.ACIEndpoints {
			fmt.Printf("ACI: %s, ASC: %s\n", aciEndpoint.ACI, aciEndpoint.ASC)
		}
		if len(eps.Keys) > 0 {
			fmt.Println("Keys: " + strings.Join(eps.Keys, ","))
		}
	}

	return
}

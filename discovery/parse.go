package discovery

import (
	"fmt"
	"net/url"
	"runtime"
	"sort"
	"strings"

	"github.com/appc/spec/schema/types"
)

const (
	defaultVersion = "latest"
	defaultOS      = runtime.GOOS
	defaultArch    = runtime.GOARCH
)

type App struct {
	Name   types.ACName
	Labels types.Labels
}

func NewStringFromApp(app *App) string {
	// Take a copy and sort the labels names
	labels := app.Labels
	sort.Sort(labels)

	s := app.Name.String()

	for _, n := range []string{"version", "os", "arch"} {
		if val, ok := labels.Get(n); ok {
			s = s + "," + n + "=" + val
		}
	}
	for _, l := range labels {
		n := l.Name.String()
		if n != "version" && n != "os" && n != "arch" {
			s = s + "," + l.Name.String() + "=" + l.Value
		}
	}
	return s
}

func NewApp(name string, labelsMap map[string]string) (*App, error) {
	labels := types.Labels{}

	if labelsMap != nil {
		for n, v := range labelsMap {
			err := labels.Set(n, v)
			if err != nil {
				return nil, err
			}
		}
	}
	acn, err := types.NewACName(name)
	if err != nil {
		return nil, err
	}

	return &App{
		Name:   *acn,
		Labels: labels,
	}, nil
}

// NewAppFromString takes a command line app parameter and returns a map of labels.
//
// Example app parameters:
// 	example.com/reduce-worker:1.0.0
// 	example.com/reduce-worker,channel=alpha,label=value
func NewAppFromString(app string) (*App, error) {
	var (
		name   string
		labels map[string]string
	)

	app = strings.Replace(app, ":", ",version=", -1)
	app = "name=" + app
	v, err := url.ParseQuery(strings.Replace(app, ",", "&", -1))
	if err != nil {
		return nil, err
	}
	labels = make(map[string]string, 0)
	for key, val := range v {
		if len(val) > 1 {
			return nil, fmt.Errorf("label %s with multiple values %q", key, val)
		}
		if key == "name" {
			name = val[0]
			continue
		}
		labels[key] = val[0]
	}
	if labels["version"] == "" {
		labels["version"] = defaultVersion
	}
	if labels["os"] == "" {
		labels["os"] = defaultOS
	}
	if labels["arch"] == "" {
		labels["arch"] = defaultArch
	}

	a, err := NewApp(name, labels)
	if err != nil {
		return nil, err
	}
	return a, nil
}

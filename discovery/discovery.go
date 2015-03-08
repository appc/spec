package discovery

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/appc/spec/Godeps/_workspace/src/golang.org/x/net/html"
	"github.com/appc/spec/Godeps/_workspace/src/golang.org/x/net/html/atom"
)

type acMeta struct {
	name   string
	prefix string
	uri    string
}

type ACIEndpoint struct {
	ACI string
	ASC string
}

type Endpoints struct {
	ACIEndpoints []ACIEndpoint
	Keys         []string
}

func (e *Endpoints) Append(ep Endpoints) {
	e.ACIEndpoints = append(e.ACIEndpoints, ep.ACIEndpoints...)
	e.Keys = append(e.Keys, ep.Keys...)
}

const (
	defaultTag = "latest"
)

var (
	templateExpression = regexp.MustCompile(`{.*?}`)
	errEnough          = errors.New("enough discovery information found")
)

func appendMeta(meta []acMeta, attrs []html.Attribute) []acMeta {
	m := acMeta{}

	for _, a := range attrs {
		if a.Namespace != "" {
			continue
		}

		switch a.Key {
		case "name":
			m.name = a.Val

		case "content":
			parts := strings.SplitN(strings.TrimSpace(a.Val), " ", 2)
			if len(parts) < 2 {
				break
			}
			m.prefix = parts[0]
			m.uri = strings.TrimSpace(parts[1])
		}
	}

	// TODO(eyakubovich): should prefix be optional?
	if !strings.HasPrefix(m.name, "ac-") || m.prefix == "" || m.uri == "" {
		return meta
	}

	return append(meta, m)
}

func extractACMeta(r io.Reader) []acMeta {
	var meta []acMeta

	z := html.NewTokenizer(r)

	for {
		switch z.Next() {
		case html.ErrorToken:
			return meta

		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			if tok.DataAtom == atom.Meta {
				meta = appendMeta(meta, tok.Attr)
			}
		}
	}
}

func renderTemplate(tpl string, kvs ...string) (string, bool) {
	for i := 0; i < len(kvs); i += 2 {
		k := kvs[i]
		v := kvs[i+1]
		tpl = strings.Replace(tpl, k, v, -1)
	}
	return tpl, !templateExpression.MatchString(tpl)
}

func createTemplateVars(app App) []string {
	tplVars := []string{"{name}", app.Name.String()}
	// If a label is called "name", it will be ignored as it appears after
	// in the slice
	for n, v := range app.Labels {
		tplVars = append(tplVars, fmt.Sprintf("{%s}", n), v)
	}
	return tplVars
}

func doDiscover(pre string, app App, insecure bool) (*Endpoints, error) {
	if app.Labels["tag"] == "" {
		app.Labels["tag"] = defaultTag
	}

	_, body, err := httpsOrHTTP(pre, insecure)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	meta := extractACMeta(body)

	tplVars := createTemplateVars(app)

	de := &Endpoints{}

	for _, m := range meta {
		if !strings.HasPrefix(app.Name.String(), m.prefix) {
			continue
		}

		switch m.name {
		case "ac-discovery":
			// Ignore not handled variables as {ext} isn't already rendered.
			uri, _ := renderTemplate(m.uri, tplVars...)
			asc, ok := renderTemplate(uri, "{ext}", "aci.asc")
			if !ok {
				continue
			}
			aci, ok := renderTemplate(uri, "{ext}", "aci")
			if !ok {
				continue
			}
			de.ACIEndpoints = append(de.ACIEndpoints, ACIEndpoint{ACI: aci, ASC: asc})

		case "ac-discovery-pubkeys":
			de.Keys = append(de.Keys, m.uri)
		}
	}

	return de, nil
}

// DiscoverWalk will make HTTPS requests to find discovery meta tags and
// optionally will use HTTP if insecure is set. Based on the response of the
// discoverFn it will continue to recurse up the tree.
func DiscoverWalk(app App, insecure bool, discoverFn DiscoverWalkFunc) (err error) {
	var (
		eps *Endpoints
	)

	parts := strings.Split(string(app.Name), "/")
	for i := range parts {
		end := len(parts) - i
		pre := strings.Join(parts[:end], "/")

		eps, err = doDiscover(pre, app, insecure)
		derr := discoverFn(pre, eps, err)
		if derr != nil {
			return err
		}
	}

	return
}

// DiscoverWalkFunc can stop a DiscoverWalk by returning non-nil error.
type DiscoverWalkFunc func(prefix string, eps *Endpoints, err error) error

// FailedAttempt represents a failed discovery attempt. This is for debugging
// and user feedback.
type FailedAttempt struct {
	Prefix string
	Error  error
}

func walker(out *Endpoints, attempts *[]FailedAttempt, testFn DiscoverWalkFunc) DiscoverWalkFunc {
	return func(pre string, eps *Endpoints, err error) error {
		if err != nil {
			*attempts = append(*attempts, FailedAttempt{pre, err})
			return nil
		}
		out.Append(*eps)
		if err := testFn(pre, eps, err); err != nil {
			return err
		}
		return nil
	}
}

// DiscoverEndpoints will make HTTPS requests to find the ac-discovery meta
// tags and optionally will use HTTP if insecure is set. It will not give up
// until it has exhausted the path or found an image discovery.
func DiscoverEndpoints(app App, insecure bool) (out *Endpoints, attempts []FailedAttempt, err error) {
	out = &Endpoints{}
	testFn := func(pre string, eps *Endpoints, err error) error {
		if len(out.ACIEndpoints) != 0 {
			return errEnough
		}
		return nil
	}

	err = DiscoverWalk(app, insecure, walker(out, &attempts, testFn))
	if err != nil && err != errEnough {
		return nil, attempts, err
	}

	return out, attempts, nil
}

// DiscoverPublicKey will make HTTPS requests to find the ac-public-keys meta
// tags and optionally will use HTTP if insecure is set. It will not give up
// until it has exhausted the path or found an public key.
func DiscoverPublicKeys(app App, insecure bool) (out *Endpoints, attempts []FailedAttempt, err error) {
	out = &Endpoints{}
	testFn := func(pre string, eps *Endpoints, err error) error {
		if len(out.Keys) != 0 {
			return errEnough
		}
		return nil
	}

	err = DiscoverWalk(app, insecure, walker(out, &attempts, testFn))
	if err != nil && err != errEnough {
		return nil, attempts, err
	}

	return out, attempts, nil
}

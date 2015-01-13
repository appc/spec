package discovery

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultDialTimeout = 5 * time.Second
)

var (
	// httpGet is the function used by discovery to retrieve URLs; it is
	// defined here so it can be overridden for testing
	httpGet func(url string) (resp *http.Response, err error)
)

func init() {
	t := &http.Transport{
		Dial: func(n, a string) (net.Conn, error) {
			return net.DialTimeout(n, a, defaultDialTimeout)
		},
	}
	c := &http.Client{
		Transport: t,
	}
	httpGet = c.Get
}

func httpsOrHTTP(name string, httpPort uint, httpsPort uint, insecure bool) (urlStr string, body io.ReadCloser, err error) {
	fetch := func(scheme string, port uint) (urlStr string, res *http.Response, err error) {
		u, err := url.Parse(scheme + "://" + name)
		if err != nil {
			return "", nil, err
		}
		u.RawQuery = "ac-discovery=1"
		if port != 0 {
			u.Host += ":" + strconv.FormatUint(uint64(port), 10)
		}
		urlStr = u.String()
		res, err = httpGet(urlStr)
		return
	}
	closeBody := func(res *http.Response) {
		if res != nil {
			res.Body.Close()
		}
	}
	urlStr, res, err := fetch("https", httpsPort)
	if err != nil || res.StatusCode != http.StatusOK {
		if insecure {
			closeBody(res)
			urlStr, res, err = fetch("http", httpPort)
		}
	}

	if res != nil && res.StatusCode != http.StatusOK {
		err = fmt.Errorf("expected a 200 OK got %d", res.StatusCode)
	}

	if err != nil {
		closeBody(res)
		return "", nil, err
	}
	return urlStr, res.Body, nil
}

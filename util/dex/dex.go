package dex

import (
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"

	"github.com/argoproj/argo-cd/errors"
)

var messageRe = regexp.MustCompile(`<p>(.*)([\s\S]*?)<\/p>`)

// NewDexHTTPReverseProxy returns a reverse proxy to the Dex server. Dex is assumed to be configured
// with the external issuer URL muxed to the same path configured in server.go. In other words, if
// ArgoCD API server wants to proxy requests at /api/dex, then the dex config yaml issuer URL should
// also be /api/dex (e.g. issuer: https://argocd.example.com/api/dex)
func NewDexHTTPReverseProxy(serverAddr string) func(writer http.ResponseWriter, request *http.Request) {
	target, err := url.Parse(serverAddr)
	errors.CheckError(err)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == 500 {
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			err = resp.Body.Close()
			if err != nil {
				return err
			}
			var message string
			matches := messageRe.FindSubmatch(b)
			if len(matches) > 1 {
				message = html.UnescapeString(string(matches[1]))
			} else {
				message = "Unknown error"
			}
			resp.ContentLength = 0
			resp.Header.Set("Content-Length", strconv.Itoa(0))
			resp.Header.Set("Location", fmt.Sprintf("/login?sso_error=%s", url.QueryEscape(message)))
			resp.StatusCode = http.StatusSeeOther
			return nil
		}
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}

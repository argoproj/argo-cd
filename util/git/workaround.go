package git

import (
	"fmt"
	neturl "net/url"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/utils/ioutil"
)

// Below is a workaround for https://github.com/src-d/go-git/issues/1177: the `github.com/src-d/go-git` does not support disable SSL cert verification is a single repo.
// As workaround methods `newUploadPackSession`, `newClient` and `listRemote` were copied from https://github.com/src-d/go-git/blob/master/remote.go and modified to use
// transport with InsecureSkipVerify flag is verification should be disabled.

func newUploadPackSession(url string, auth transport.AuthMethod, insecure bool, creds Creds, proxy string, noProxy string) (transport.UploadPackSession, error) {
	c, ep, err := newClient(url, insecure, creds, proxy, noProxy)
	if err != nil {
		return nil, err
	}

	return c.NewUploadPackSession(ep, auth)
}

func newClient(url string, insecure bool, creds Creds, proxy string, noProxy string) (transport.Transport, *transport.Endpoint, error) {
	ep, err := transport.NewEndpoint(url)
	if err != nil {
		return nil, nil, err
	}

	if !IsHTTPSURL(url) && !IsHTTPURL(url) {
		// use the default client for protocols other than HTTP/HTTPS
		ep.InsecureSkipTLS = insecure
		if proxy != "" {
			parsedProxyURL, err := neturl.Parse(proxy)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create client for url '%s', error parsing proxy url '%s': %w", url, proxy, err)
			}
			var proxyUsername, proxyPasswd string
			if parsedProxyURL.User != nil {
				proxyUsername = parsedProxyURL.User.Username()
				proxyPasswd, _ = parsedProxyURL.User.Password()
			}
			ep.Proxy = transport.ProxyOptions{
				URL:      fmt.Sprintf("%s://%s:%s", parsedProxyURL.Scheme, parsedProxyURL.Hostname(), parsedProxyURL.Port()),
				Username: proxyUsername,
				Password: proxyPasswd,
			}
		}
		c, err := client.NewClient(ep)
		if err != nil {
			return nil, nil, err
		}
		return c, ep, nil
	}

	return http.NewClient(GetRepoHTTPClient(url, insecure, creds, proxy, noProxy)), ep, nil
}

func listRemote(r *git.Remote, o *git.ListOptions, insecure bool, creds Creds, proxy string, noProxy string) (rfs []*plumbing.Reference, err error) {
	s, err := newUploadPackSession(r.Config().URLs[0], o.Auth, insecure, creds, proxy, noProxy)
	if err != nil {
		return nil, err
	}

	defer ioutil.CheckClose(s, &err)

	ar, err := s.AdvertisedReferences()
	if err != nil {
		return nil, err
	}

	allRefs, err := ar.AllReferences()
	if err != nil {
		return nil, err
	}

	refs, err := allRefs.IterReferences()
	if err != nil {
		return nil, err
	}

	var resultRefs []*plumbing.Reference
	_ = refs.ForEach(func(ref *plumbing.Reference) error {
		resultRefs = append(resultRefs, ref)
		return nil
	})

	return resultRefs, nil
}

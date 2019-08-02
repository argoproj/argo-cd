package git

import (
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

// Below is a workaround for https://github.com/src-d/go-git/issues/1177: the `github.com/src-d/go-git` does not support disable SSL cert verification is a single repo.
// As workaround methods `newUploadPackSession`, `newClient` and `listRemote` were copied from https://github.com/src-d/go-git/blob/master/remote.go and modified to use
// transport with InsecureSkipVerify flag is verification should be disabled.

func newUploadPackSession(url string, auth transport.AuthMethod, insecure bool, creds Creds) (transport.UploadPackSession, error) {
	c, ep, err := newClient(url, insecure, creds)
	if err != nil {
		return nil, err
	}

	return c.NewUploadPackSession(ep, auth)
}

func newClient(url string, insecure bool, creds Creds) (transport.Transport, *transport.Endpoint, error) {
	ep, err := transport.NewEndpoint(url)
	if err != nil {
		return nil, nil, err
	}

	var c transport.Transport
	// For HTTPS repositories, we get a custom Transport. Everything else will
	// be default.
	if IsHTTPSURL(url) {
		c = http.NewClient(GetRepoHTTPClient(url, insecure, creds))
	} else {
		c, err = client.NewClient(ep)
		if err != nil {
			return nil, nil, err
		}
	}
	return c, ep, err
}

func listRemote(r *git.Remote, o *git.ListOptions, insecure bool, creds Creds) (rfs []*plumbing.Reference, err error) {
	s, err := newUploadPackSession(r.Config().URLs[0], o.Auth, insecure, creds)
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

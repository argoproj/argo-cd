//go:build !race
// +build !race

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/test"
)

func TestUserAgent(t *testing.T) {

	// !race:
	// A data race in go-client's `shared_informer.go`, between `sharedProcessor.run(...)` and itself. Based on
	// the data race, it APPEARS to be intentional, but in any case it's nothing we are doing in Argo CD
	// that is causing this issue.

	s, closer := fakeServer()
	defer closer()
	lns, err := s.Listen()
	assert.NoError(t, err)

	cancelInformer := test.StartInformer(s.projInformer)
	defer cancelInformer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, lns)
	defer func() { time.Sleep(3 * time.Second) }()

	type testData struct {
		userAgent string
		errorMsg  string
	}
	currentVersionBytes, err := ioutil.ReadFile("../VERSION")
	assert.NoError(t, err)
	currentVersion := strings.TrimSpace(string(currentVersionBytes))
	var tests = []testData{
		{
			// Reject out-of-date user-agent
			userAgent: fmt.Sprintf("%s/0.10.0", common.ArgoCDUserAgentName),
			errorMsg:  "unsatisfied client version constraint",
		},
		{
			// Accept up-to-date user-agent
			userAgent: fmt.Sprintf("%s/%s", common.ArgoCDUserAgentName, currentVersion),
		},
		{
			// Accept up-to-date pre-release user-agent
			userAgent: fmt.Sprintf("%s/%s-rc1", common.ArgoCDUserAgentName, currentVersion),
		},
		{
			// Permit custom clients
			userAgent: "foo/1.2.3",
		},
	}

	for _, test := range tests {
		opts := apiclient.ClientOptions{
			ServerAddr: fmt.Sprintf("localhost:%d", s.ListenPort),
			PlainText:  true,
			UserAgent:  test.userAgent,
		}
		clnt, err := apiclient.NewClient(&opts)
		assert.NoError(t, err)
		conn, appClnt := clnt.NewApplicationClientOrDie()
		_, err = appClnt.List(ctx, &applicationpkg.ApplicationQuery{})
		if test.errorMsg != "" {
			assert.Error(t, err)
			assert.Regexp(t, test.errorMsg, err.Error())
		} else {
			assert.NoError(t, err)
		}
		_ = conn.Close()
	}
}

func Test_StaticHeaders(t *testing.T) {

	// !race:
	// Same as TestUserAgent

	t.Run(`Test default policy "sameorigin" and "frame-ancestors 'self';"`, func(t *testing.T) {
		s, closer := fakeServer()
		defer closer()
		lns, err := s.Listen()
		assert.NoError(t, err)
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx, lns)
		defer func() { time.Sleep(3 * time.Second) }()

		// Allow server startup
		time.Sleep(1 * time.Second)

		testCases := []struct{
			url string
		}{
			{fmt.Sprintf("http://127.0.0.1:%d/test.html", s.ListenPort)},
			{fmt.Sprintf("http://127.0.0.1:%d/auth/callback", s.ListenPort)},
			{fmt.Sprintf("http://127.0.0.1:%d/auth/login", s.ListenPort)},
		}

		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.url, func(t *testing.T) {
				t.Parallel()
				assertGotExpectedHeaders(t, testCase.url, "sameorigin", "frame-ancestors 'self';")
			})
		}
	})

	t.Run("Test custom policy for X-Frame-Options and Content-Security-Policy", func(t *testing.T) {
		s, closer := fakeServer()
		defer closer()
		s.SecurityHeaders.XFrameOptions = "deny"
		s.SecurityHeaders.ContentSecurityPolicy = "frame-ancestors 'none';"
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		lns, err := s.Listen()
		assert.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx, lns)
		defer func() { time.Sleep(3 * time.Second) }()

		// Allow server startup
		time.Sleep(1 * time.Second)

		testCases := []struct{
			url string
			expectedXFrameOptions string
			expectedContentSecurityPolicy string
		}{
			{fmt.Sprintf("http://127.0.0.1:%d/test.html", s.ListenPort), "deny", "frame-ancestors 'none';"},
			{fmt.Sprintf("http://127.0.0.1:%d/auth/callback", s.ListenPort), "deny", "frame-ancestors 'none';"},
			{fmt.Sprintf("http://127.0.0.1:%d/auth/login", s.ListenPort), "deny", "frame-ancestors 'none';"},
		}

		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.url, func(t *testing.T) {
				t.Parallel()
				assertGotExpectedHeaders(t, testCase.url, testCase.expectedXFrameOptions, testCase.expectedContentSecurityPolicy)
			})
		}
	})

	t.Run("Test disabled X-Frame-Options and Content-Security-Policy", func(t *testing.T) {
		s, closer := fakeServer()
		defer closer()
		s.SecurityHeaders.XFrameOptions = ""
		s.SecurityHeaders.ContentSecurityPolicy = ""
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		lns, err := s.Listen()
		assert.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx, lns)
		defer func() { time.Sleep(3 * time.Second) }()

		err = test.WaitForPortListen(fmt.Sprintf("127.0.0.1:%d", s.ListenPort), 10*time.Second)
		assert.NoError(t, err)

		// Allow server startup
		time.Sleep(1 * time.Second)

		testCases := []struct{
			url string
			expectedXFrameOptions string
			expectedContentSecurityPolicy string
		}{
			{fmt.Sprintf("http://127.0.0.1:%d/test.html", s.ListenPort), "", ""},
			{fmt.Sprintf("http://127.0.0.1:%d/auth/callback", s.ListenPort), "", ""},
			{fmt.Sprintf("http://127.0.0.1:%d/auth/login", s.ListenPort), "", ""},
		}

		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.url, func(t *testing.T) {
				t.Parallel()
				assertGotExpectedHeaders(t, testCase.url, testCase.expectedXFrameOptions, testCase.expectedContentSecurityPolicy)
			})
		}
	})
}

func assertGotExpectedHeaders(t *testing.T, url string, expectedXFrameOptions string, expectedContentSecurityPolicy string) {
	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	assert.NoError(t, err)
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equalf(t, expectedXFrameOptions, resp.Header.Get("X-Frame-Options"), "got the wrong X-Frame-Options for url %q", url)
	assert.Equalf(t, expectedContentSecurityPolicy, resp.Header.Get("Content-Security-Policy"), "got the wrong Content-Security-Policy for url %q", url)
}

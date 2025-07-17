//go:build !race
// +build !race

package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	s, closer := fakeServer(t)
	defer closer()
	lns, err := s.Listen()
	require.NoError(t, err)

	cancelInformer := test.StartInformer(s.projInformer)
	defer cancelInformer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Init(ctx)
	go s.Run(ctx, lns)
	defer func() { time.Sleep(3 * time.Second) }()

	type testData struct {
		userAgent string
		errorMsg  string
	}
	currentVersionBytes, err := os.ReadFile("../VERSION")
	require.NoError(t, err)
	currentVersion := strings.TrimSpace(string(currentVersionBytes))
	tests := []testData{
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
		require.NoError(t, err)
		conn, appClnt := clnt.NewApplicationClientOrDie()
		_, err = appClnt.List(ctx, &applicationpkg.ApplicationQuery{})
		if test.errorMsg != "" {
			require.Error(t, err)
			assert.Regexp(t, test.errorMsg, err.Error())
		} else {
			require.NoError(t, err)
		}
		_ = conn.Close()
	}
}

func Test_StaticHeaders(t *testing.T) {
	// !race:
	// Same as TestUserAgent

	// Test default policy "sameorigin" and "frame-ancestors 'self';"
	{
		s, closer := fakeServer(t)
		defer closer()
		lns, err := s.Listen()
		require.NoError(t, err)
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.Init(ctx)
		go s.Run(ctx, lns)
		defer func() { time.Sleep(3 * time.Second) }()

		// Allow server startup
		time.Sleep(1 * time.Second)

		client := http.Client{}
		url := fmt.Sprintf("http://127.0.0.1:%d/test.html", s.ListenPort)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, "sameorigin", resp.Header.Get("X-Frame-Options"))
		assert.Equal(t, "frame-ancestors 'self';", resp.Header.Get("Content-Security-Policy"))
	}

	// Test custom policy for X-Frame-Options and Content-Security-Policy
	{
		s, closer := fakeServer(t)
		defer closer()
		s.XFrameOptions = "deny"
		s.ContentSecurityPolicy = "frame-ancestors 'none';"
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		lns, err := s.Listen()
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.Init(ctx)
		go s.Run(ctx, lns)
		defer func() { time.Sleep(3 * time.Second) }()

		// Allow server startup
		time.Sleep(1 * time.Second)

		client := http.Client{}
		url := fmt.Sprintf("http://127.0.0.1:%d/test.html", s.ListenPort)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, "deny", resp.Header.Get("X-Frame-Options"))
		assert.Equal(t, "frame-ancestors 'none';", resp.Header.Get("Content-Security-Policy"))
	}

	// Test disabled X-Frame-Options and Content-Security-Policy
	{
		s, closer := fakeServer(t)
		defer closer()
		s.XFrameOptions = ""
		s.ContentSecurityPolicy = ""
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		lns, err := s.Listen()
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.Init(ctx)
		go s.Run(ctx, lns)
		defer func() { time.Sleep(3 * time.Second) }()

		err = test.WaitForPortListen(fmt.Sprintf("127.0.0.1:%d", s.ListenPort), 10*time.Second)
		require.NoError(t, err)

		// Allow server startup
		time.Sleep(1 * time.Second)

		client := http.Client{}
		url := fmt.Sprintf("http://127.0.0.1:%d/test.html", s.ListenPort)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Empty(t, resp.Header.Get("X-Frame-Options"))
		assert.Empty(t, resp.Header.Get("Content-Security-Policy"))
	}
}

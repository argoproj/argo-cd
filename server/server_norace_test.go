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

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/test"

	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
)

func TestUserAgent(t *testing.T) {

	// !race:
	// A data race in go-client's `shared_informer.go`, between `sharedProcessor.run(...)` and itself. Based on
	// the data race, it APPEARS to be intentional, but in any case it's nothing we are doing in Argo CD
	// that is causing this issue.

	s := fakeServer()
	cancelInformer := test.StartInformer(s.projInformer)
	defer cancelInformer()
	port, err := test.GetFreePort()
	assert.NoError(t, err)
	metricsPort, err := test.GetFreePort()
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, port, metricsPort)
	defer func() { time.Sleep(3 * time.Second) }()

	err = test.WaitForPortListen(fmt.Sprintf("127.0.0.1:%d", port), 10*time.Second)
	assert.NoError(t, err)

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
			// Reject legacy client
			// NOTE: after we update the grpc-go client past 1.15.0, this test will break and should be deleted
			userAgent: " ", // need a space here since the apiclient will set the default user-agent if empty
			errorMsg:  "unsatisfied client version constraint",
		},
		{
			// Permit custom clients
			userAgent: "foo/1.2.3",
		},
	}

	for _, test := range tests {
		opts := apiclient.ClientOptions{
			ServerAddr: fmt.Sprintf("localhost:%d", port),
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

	// Test default policy "sameorigin"
	{
		s := fakeServer()
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		port, err := test.GetFreePort()
		assert.NoError(t, err)
		metricsPort, err := test.GetFreePort()
		assert.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx, port, metricsPort)
		defer func() { time.Sleep(3 * time.Second) }()

		err = test.WaitForPortListen(fmt.Sprintf("127.0.0.1:%d", port), 10*time.Second)
		assert.NoError(t, err)

		// Allow server startup
		time.Sleep(1 * time.Second)

		client := http.Client{}
		url := fmt.Sprintf("http://127.0.0.1:%d/test.html", port)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, "sameorigin", resp.Header.Get("X-Frame-Options"))
	}

	// Test custom policy
	{
		s := fakeServer()
		s.XFrameOptions = "deny"
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		port, err := test.GetFreePort()
		assert.NoError(t, err)
		metricsPort, err := test.GetFreePort()
		assert.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx, port, metricsPort)
		defer func() { time.Sleep(3 * time.Second) }()

		err = test.WaitForPortListen(fmt.Sprintf("127.0.0.1:%d", port), 10*time.Second)
		assert.NoError(t, err)

		// Allow server startup
		time.Sleep(1 * time.Second)

		client := http.Client{}
		url := fmt.Sprintf("http://127.0.0.1:%d/test.html", port)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, "deny", resp.Header.Get("X-Frame-Options"))
	}

	// Test disabled
	{
		s := fakeServer()
		s.XFrameOptions = ""
		cancelInformer := test.StartInformer(s.projInformer)
		defer cancelInformer()
		port, err := test.GetFreePort()
		assert.NoError(t, err)
		metricsPort, err := test.GetFreePort()
		assert.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.Run(ctx, port, metricsPort)
		defer func() { time.Sleep(3 * time.Second) }()

		err = test.WaitForPortListen(fmt.Sprintf("127.0.0.1:%d", port), 10*time.Second)
		assert.NoError(t, err)

		// Allow server startup
		time.Sleep(1 * time.Second)

		client := http.Client{}
		url := fmt.Sprintf("http://127.0.0.1:%d/test.html", port)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Empty(t, resp.Header.Get("X-Frame-Options"))
	}
}

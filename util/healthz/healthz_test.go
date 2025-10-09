package healthz

import (
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	sentinel := false
	lc := &net.ListenConfig{}
	ctx := t.Context()

	serve := func(c chan<- string) {
		// listen on first available dynamic (unprivileged) port
		listener, err := lc.Listen(ctx, "tcp", ":0")
		if err != nil {
			panic(err)
		}

		// send back the address so that it can be used
		c <- listener.Addr().String()

		mux := http.NewServeMux()
		ServeHealthCheck(mux, func(_ *http.Request) error {
			if sentinel {
				return errors.New("This is a dummy error")
			}
			return nil
		})
		panic(http.Serve(listener, mux))
	}

	c := make(chan string, 1)

	// run a local webserver to test data retrieval
	go serve(c)

	address := <-c
	t.Logf("Listening at address: %s", address)

	server := "http://" + address

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+"/healthz", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "Was expecting status code 200 from health check, but got %d instead", resp.StatusCode)

	sentinel = true

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, server+"/healthz", http.NoBody)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equalf(t, http.StatusServiceUnavailable, resp.StatusCode, "Was expecting status code 503 from health check, but got %d instead", resp.StatusCode)
}

package healthz

import (
	"fmt"
	"net"
	"net/http"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	sentinel := false

	serve := func(c chan<- string) {
		// listen on first available dynamic (unprivileged) port
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			panic(err)
		}

		// send back the address so that it can be used
		c <- listener.Addr().String()

		mux := http.NewServeMux()
		ServeHealthCheck(mux, func(r *http.Request) error {
			if sentinel {
				return fmt.Errorf("This is a dummy error")
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

	resp, err := http.Get(server + "/healthz")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Was expecting status code 200 from health check, but got %d instead", resp.StatusCode)
	}

	sentinel = true

	resp, _ = http.Get(server + "/healthz")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Was expecting status code 503 from health check, but got %d instead", resp.StatusCode)
	}
}

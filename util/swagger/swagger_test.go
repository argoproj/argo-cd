package swagger

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/go-openapi/loads"

	"github.com/argoproj/argo-cd/v2/util/assets"
)

func TestSwaggerUI(t *testing.T) {
	serve := func(c chan<- string) {
		// listen on first available dynamic (unprivileged) port
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			panic(err)
		}

		// send back the address so that it can be used
		c <- listener.Addr().String()

		mux := http.NewServeMux()
		ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui", "")
		panic(http.Serve(listener, mux))
	}

	c := make(chan string, 1)

	// run a local webserver to test data retrieval
	go serve(c)

	address := <-c
	t.Logf("Listening at address: %s", address)

	server := "http://" + address

	specDoc, err := loads.Spec(server + "/swagger.json")
	if err != nil {
		t.Fatal(err)
	}

	_, err = json.MarshalIndent(specDoc.Spec(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(server + "/swagger-ui")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Was expecting status code 200 from swagger-ui, but got %d instead", resp.StatusCode)
	}
}

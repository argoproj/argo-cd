package swagger

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/gobuffalo/packr"
)

func TestSwaggerUI(t *testing.T) {
	const sentinel = `{
  "swagger": "2.0",
  "info": {
    "title": "Consolidate Services",
    "description": "Description of all APIs",
    "version": "version not set"
  },
  "paths": {}
}`

	serve := func(c chan<- string) {
		// listen on first available dynamic (unprivileged) port
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			panic(err)
		}

		// send back the address so that it can be used
		c <- listener.Addr().String()

		mux := http.NewServeMux()
		ServeSwaggerUI(mux, packr.NewBox("../../server"), "/swagger-ui")
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

	if resp.StatusCode != 200 {
		t.Fatalf("Was expecting status code 200 from swagger-ui, but got %d instead", resp.StatusCode)
	}
}

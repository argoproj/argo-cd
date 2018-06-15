package swagger

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/go-openapi/loads"
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

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// return the sentinel text at root URL
			fmt.Fprint(w, sentinel)
		})

		panic(http.Serve(listener, nil))
	}

	c := make(chan string, 1)

	// run a local webserver to test data retrieval
	go serve(c)

	address := <-c
	t.Logf("Listening at address: %s", address)

	specDoc, err := loads.Spec("http://" + address)
	if err != nil {
		t.Error(err)
	}

	_, err = json.MarshalIndent(specDoc.Spec(), "", "  ")
	if err != nil {
		t.Error(err)
	}
}

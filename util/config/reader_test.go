package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalLocalFile(t *testing.T) {
	const (
		field1 = "Hello, world!"
		field2 = 42
	)
	sentinel := fmt.Sprintf("---\nfield1: %q\nfield2: %d", field1, field2)

	file, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	_, _ = file.WriteString(sentinel)
	_ = file.Sync()

	var testStruct struct {
		Field1 string
		Field2 int
	}
	err = UnmarshalLocalFile(file.Name(), &testStruct)
	if err != nil {
		t.Errorf("Could not unmarshal test data: %s", err)
	}

	if testStruct.Field1 != field1 || testStruct.Field2 != field2 {
		t.Errorf("Test data did not match! Expected {%s %d} but got: %v", field1, field2, testStruct)
	}
}

func TestUnmarshal(t *testing.T) {
	const (
		field1 = "Hello, world!"
		field2 = 42
	)
	sentinel := fmt.Sprintf("---\nfield1: %q\nfield2: %d", field1, field2)

	var testStruct struct {
		Field1 string
		Field2 int
	}
	err := Unmarshal([]byte(sentinel), &testStruct)
	if err != nil {
		t.Errorf("Could not unmarshal test data: %s", err)
	}

	if testStruct.Field1 != field1 || testStruct.Field2 != field2 {
		t.Errorf("Test data did not match! Expected {%s %d} but got: %v", field1, field2, testStruct)
	}
}

func TestUnmarshalRemoteFile(t *testing.T) {
	const (
		field1 = "Hello, world!"
		field2 = 42
	)
	sentinel := fmt.Sprintf("---\nfield1: %q\nfield2: %d", field1, field2)

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

	data, err := ReadRemoteFile("http://" + address)
	if string(data) != sentinel {
		t.Errorf("Test data did not match (err = %v)! Expected %q and received %q", err, sentinel, string(data))
	}

	var testStruct struct {
		Field1 string
		Field2 int
	}
	err = UnmarshalRemoteFile("http://"+address, &testStruct)
	if err != nil {
		t.Errorf("Could not unmarshal test data: %s", err)
	}

	if testStruct.Field1 != field1 || testStruct.Field2 != field2 {
		t.Errorf("Test data did not match! Expected {%s %d} but got: %v", field1, field2, testStruct)
	}
}

func TestUnmarshalReader(t *testing.T) {
	type testStruct struct {
		Value string
	}
	value := "test-reader"
	instance := testStruct{value}
	data, err := json.Marshal(instance)
	require.NoError(t, err)
	var reader io.Reader = bytes.NewReader(data)
	err = UnmarshalReader(reader, &instance)
	require.NoError(t, err)
	assert.Equal(t, value, instance.Value)
}

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
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

	file, err := os.CreateTemp(t.TempDir(), "test-*.yaml")
	require.NoError(t, err)

	_, err = file.WriteString(sentinel)
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)

	var testStruct struct {
		Field1 string
		Field2 int
	}
	err = UnmarshalLocalFile(file.Name(), &testStruct)
	require.NoError(t, err)
	assert.Equal(t, field1, testStruct.Field1)
	assert.Equal(t, field2, testStruct.Field2)
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
	require.NoError(t, err)
	assert.Equal(t, field1, testStruct.Field1)
	assert.Equal(t, field2, testStruct.Field2)
}

func TestUnmarshalRemoteFile(t *testing.T) {
	const (
		field1 = "Hello, world!"
		field2 = 42
	)
	sentinel := fmt.Sprintf("---\nfield1: %q\nfield2: %d", field1, field2)

	serve := func(c chan<- string) {
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		c <- listener.Addr().String()

		http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, sentinel)
		})

		panic(http.Serve(listener, nil))
	}

	c := make(chan string, 1)
	go serve(c)
	address := <-c
	t.Logf("Listening at address: %s", address)

	data, err := ReadRemoteFile("http://" + address)
	require.NoError(t, err)
	assert.Equal(t, string(data), sentinel)

	var testStruct struct {
		Field1 string
		Field2 int
	}
	err = UnmarshalRemoteFile("http://"+address, &testStruct)
	require.NoError(t, err)
	assert.Equal(t, field1, testStruct.Field1)
	assert.Equal(t, field2, testStruct.Field2)
}

func TestUnmarshalReader_Valid(t *testing.T) {
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

func TestMarshalLocalYAMLFile(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yaml")
	require.NoError(t, err)

	data := struct{ Name string }{Name: "test"}
	err = MarshalLocalYAMLFile(tmpFile.Name(), data)
	require.NoError(t, err)

	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.Contains(t, string(content), "Name: test")
}

// ---------------- Error Tests ----------------

func TestUnmarshalReader_Error(t *testing.T) {
	reader := bytes.NewReader([]byte{0xff, 0xfe, 0xfd}) // invalid UTF-8
	var obj map[string]any
	err := UnmarshalReader(reader, &obj)
	assert.Error(t, err)
}

func TestUnmarshalObject_InvalidYAML(t *testing.T) {
	data := []byte(`key: "unterminated`)
	var obj map[string]any
	err := unmarshalObject(data, &obj)
	assert.Error(t, err)
}

func TestUnmarshalReader_InvalidYAML(t *testing.T) {
	data := []byte("key: [unclosed")
	var obj map[string]any
	err := UnmarshalReader(bytes.NewReader(data), &obj)
	assert.Error(t, err)
}

func TestGetStringSliceFlag_InvalidCSV(t *testing.T) {
	multiFlags = map[string][]string{"header": {`a,"unterminated`}}
	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, fmt.Sprint(r), "unterminated")
		}
	}()
	_ = GetStringSliceFlag("header", []string{"fallback"})
}

func TestInit_Subprocess(t *testing.T) {
	if os.Getenv("BE_INIT_TEST") == "1" {
		_ = GetFlag("nonexistent", "default")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestInit_Subprocess")
	cmd.Env = append(os.Environ(), "BE_INIT_TEST=1")
	err := cmd.Run()
	require.NoError(t, err)
}

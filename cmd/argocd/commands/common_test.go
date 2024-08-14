package commands

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Be careful with tabs vs. spaces in the following expected formats. Indents
// should all be spaces, no tabs.
const expectYamlSingle = `bar: ""
baz: foo
foo: bar
`

const expectJsonSingle = `{
  "bar": "",
  "baz": "foo",
  "foo": "bar"
}
`

const expectYamlList = `one:
  bar: ""
  baz: foo
  foo: bar
two:
  bar: ""
  baz: foo
  foo: bar
`

const expectJsonList = `{
  "one": {
    "bar": "",
    "baz": "foo",
    "foo": "bar"
  },
  "two": {
    "bar": "",
    "baz": "foo",
    "foo": "bar"
  }
}
`

// Rather dirty hack to capture stdout from PrintResource() and PrintResourceList()
func captureOutput(f func() error) (string, error) {
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	err = f()
	w.Close()
	if err != nil {
		os.Stdout = stdout
		return "", err
	}
	str, err := io.ReadAll(r)
	os.Stdout = stdout
	if err != nil {
		return "", err
	}
	return string(str), err
}

func Test_PrintResource(t *testing.T) {
	testResource := map[string]string{
		"foo": "bar",
		"bar": "",
		"baz": "foo",
	}

	str, err := captureOutput(func() error {
		err := PrintResource(testResource, "yaml")
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, expectYamlSingle, str)

	str, err = captureOutput(func() error {
		err := PrintResource(testResource, "json")
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, expectJsonSingle, str)

	err = PrintResource(testResource, "unknown")
	require.Error(t, err)
}

func Test_PrintResourceList(t *testing.T) {
	testResource := map[string]map[string]string{
		"one": {
			"foo": "bar",
			"bar": "",
			"baz": "foo",
		},
		"two": {
			"foo": "bar",
			"bar": "",
			"baz": "foo",
		},
	}

	testResource2 := make([]map[string]string, 0)
	testResource2 = append(testResource2, testResource["one"])

	str, err := captureOutput(func() error {
		err := PrintResourceList(testResource, "yaml", false)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, expectYamlList, str)

	str, err = captureOutput(func() error {
		err := PrintResourceList(testResource, "json", false)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, expectJsonList, str)

	str, err = captureOutput(func() error {
		err := PrintResourceList(testResource2, "yaml", true)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, expectYamlSingle, str)

	str, err = captureOutput(func() error {
		err := PrintResourceList(testResource2, "json", true)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, expectJsonSingle, str)

	err = PrintResourceList(testResource, "unknown", false)
	require.Error(t, err)
}

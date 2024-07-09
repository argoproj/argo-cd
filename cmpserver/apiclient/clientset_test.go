package apiclient

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type readSuccessTest struct {
	dir      string
	expected string
}

type readErrorTest struct {
	dir      string
	expected error
}

func TestReadAuth(t *testing.T) {
	c := clientSet{secretPath: `ns/svc/port`}
	tests := []readSuccessTest{
		{dir: `tests/test1`, expected: `abc123`},
		{dir: `tests/test2`, expected: `abc123`},
		{dir: `tests/test3`, expected: `def456`},
		{dir: `tests/test4`, expected: `foobar`},
	}
	wd, err := os.Getwd()
	assert.Nil(t, err)
	for _, test := range tests {
		result, err := c.readAuthSecret(filepath.Join(wd, test.dir))
		fmt.Printf("%v", err)
		assert.Nil(t, err)
		assert.Equal(t, test.expected, result)
	}
}

// Hacky creation of an unreadable secret
func initUnreadable(t *testing.T, wd string) {
	t.Helper()
	dir := filepath.Join(wd, `tests/unreadable`)
	os.RemoveAll(dir)
	err := os.Mkdir(dir, os.ModePerm)
	assert.Nil(t, err)
	file, err := os.OpenFile(filepath.Join(dir, `secret`), os.O_CREATE|os.O_WRONLY, 0200)
	assert.Nil(t, err)
	defer file.Close()
	n, err := file.Write([]byte(`abc123`))
	assert.Equal(t, 6, n)
	assert.Nil(t, err)
}

func TestReadAuthErrs(t *testing.T) {
	c := clientSet{secretPath: `ns/svc/port`}
	wd, err := os.Getwd()
	assert.Nil(t, err)
	initUnreadable(t, wd)
	tests := []string{
		`tests/empty`,
		`tests/unreadable`,
	}
	for _, test := range tests {
		result, err := c.readAuthSecret(filepath.Join(wd, test))
		fmt.Printf("%v", err)
		assert.NotNil(t, err)
		assert.Equal(t, "", result)
	}
}

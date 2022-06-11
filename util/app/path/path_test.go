package path

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathRoot(t *testing.T) {
	_, err := Path("./testdata", "/")
	assert.EqualError(t, err, "/: app path is absolute")
}

func TestPathAbsolute(t *testing.T) {
	_, err := Path("./testdata", "/etc/passwd")
	assert.EqualError(t, err, "/etc/passwd: app path is absolute")
}

func TestPathDotDot(t *testing.T) {
	_, err := Path("./testdata", "..")
	assert.EqualError(t, err, "..: app path outside root")
}

func TestPathDotDotSlash(t *testing.T) {
	_, err := Path("./testdata", "../")
	assert.EqualError(t, err, "../: app path outside root")
}

func TestPathDot(t *testing.T) {
	_, err := Path("./testdata", ".")
	assert.NoError(t, err)
}

func TestPathDotSlash(t *testing.T) {
	_, err := Path("./testdata", "./")
	assert.NoError(t, err)
}

func TestNonExistentPath(t *testing.T) {
	_, err := Path("./testdata", "does-not-exist")
	assert.EqualError(t, err, "does-not-exist: app path does not exist")
}

func TestPathNotDir(t *testing.T) {
	_, err := Path("./testdata", "file.txt")
	assert.EqualError(t, err, "file.txt: app path is not a directory")
}

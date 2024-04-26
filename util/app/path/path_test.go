package path

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fileutil "github.com/argoproj/argo-cd/v2/test/fixture/path"
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

func TestGoodSymlinks(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/goodlink")
	assert.NoError(t, err)
}

// Simple check of leaving the repo
func TestBadSymlinks(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/badlink")
	oobError := &OutOfBoundsSymlinkError{}
	assert.ErrorAs(t, err, &oobError)
	assert.Equal(t, oobError.File, "badlink")
}

// Crazy formatting check
func TestBadSymlinks2(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/badlink2")
	oobError := &OutOfBoundsSymlinkError{}
	assert.ErrorAs(t, err, &oobError)
	assert.Equal(t, oobError.File, "badlink")
}

// Make sure no part of the symlink can leave the repo, even if it ultimately targets inside the repo
func TestBadSymlinks3(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/badlink3")
	oobError := &OutOfBoundsSymlinkError{}
	assert.ErrorAs(t, err, &oobError)
	assert.Equal(t, oobError.File, "badlink")
}

// No absolute symlinks allowed
func TestAbsSymlink(t *testing.T) {
	const testDir = "./testdata/abslink"
	require.NoError(t, fileutil.CreateSymlink(t, testDir, "/somethingbad", "abslink"))
	defer os.Remove(path.Join(testDir, "abslink"))
	err := CheckOutOfBoundsSymlinks(testDir)
	oobError := &OutOfBoundsSymlinkError{}
	assert.ErrorAs(t, err, &oobError)
	assert.Equal(t, oobError.File, "abslink")
}

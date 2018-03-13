package commands

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestReadLocalFile(t *testing.T) {
	sentinel := "Hello, world!"

	file, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		t.Errorf("Could not write test data")
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	_, _ = file.WriteString(sentinel)
	_ = file.Sync()

	data := readLocalFile(file.Name())
	if string(data) != sentinel {
		t.Errorf("Test data did not match! Expected \"%s\" and received \"%s\"", sentinel, string(data))
	}
}

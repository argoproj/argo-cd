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
	defer os.Remove(file.Name())

	file.WriteString(sentinel)
	file.Sync()

	data := readLocalFile(file.Name())
	if data != sentinel {
		t.Errorf("Test data did not match! Expected \"%s\" and received \"%s\"", sentinel, data)
	}

	
}

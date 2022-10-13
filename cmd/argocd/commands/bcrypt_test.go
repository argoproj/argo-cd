package commands

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"io"
	"os"
	"strings"
	"testing"
)

// capture replaces os.Stdout with a writer that buffers any data written
// to os.Stdout. Call the returned function to clean up and get the data
// as a string.
func capture() func() (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	done := make(chan error, 1)

	save := os.Stdout
	os.Stdout = w

	var buf strings.Builder

	go func() {
		_, err = io.Copy(&buf, r)
		err = r.Close()
		if err != nil {
			return
		}
		done <- err
	}()

	return func() (string, error) {
		os.Stdout = save
		err := w.Close()
		if err != nil {
			return "", err
		}
		err = <-done
		return buf.String(), err
	}
}

func TestGeneratePassword(t *testing.T) {

	done := capture()
	bcryptCmd := NewBcryptCmd()

	bcryptCmd.SetArgs([]string{"--password", "abc"})
	err := bcryptCmd.Execute()
	if err != nil {
		return
	}
	capturedOutput, err := done()
	assert.NoError(t, err)

	err = bcrypt.CompareHashAndPassword([]byte(capturedOutput), []byte("abc"))
	assert.NoError(t, err)
}

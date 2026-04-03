package commands

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestGeneratePassword(t *testing.T) {
	bcryptCmd := NewBcryptCmd()
	bcryptCmd.SetArgs([]string{"--password", "abc"})
	output := new(bytes.Buffer)
	bcryptCmd.SetOut(output)
	err := bcryptCmd.Execute()
	if err != nil {
		return
	}
	err = bcrypt.CompareHashAndPassword(output.Bytes(), []byte("abc"))
	assert.NoError(t, err)
}

func TestGeneratePasswordWithStdin(t *testing.T) {
	oldStdin := os.Stdin
	defer func() {
		os.Stdin = oldStdin
	}()

	input := bytes.NewBufferString("abc\n")
	r, w, _ := os.Pipe()
	_, _ = w.Write(input.Bytes())
	w.Close()
	os.Stdin = r

	bcryptCmd := NewBcryptCmd()
	bcryptCmd.SetArgs([]string{})
	output := new(bytes.Buffer)
	bcryptCmd.SetOut(output)

	err := bcryptCmd.Execute()
	require.NoError(t, err)

	err = bcrypt.CompareHashAndPassword(output.Bytes(), []byte("abc"))
	assert.NoError(t, err)
}

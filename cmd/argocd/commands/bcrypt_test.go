package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
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

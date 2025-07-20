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

func TestComparePassword(t *testing.T) {
	bcryptCmd := NewBcryptCmd()
	bcryptCmd.SetArgs([]string{"--password", "abc"})
	output := new(bytes.Buffer)
	bcryptCmd.SetOut(output)
	err := bcryptCmd.Execute()
	if err != nil {
		return
	}

	bcryptCmd.SetArgs([]string{"--password", "abc", "--compare", output.String()})
	output.Reset()

	err = bcryptCmd.Execute()
	if err != nil {
		return
	}

	assert.Equal(t, output.String(), "yes")
}

func TestComparePasswordFailureCase(t *testing.T) {
	bcryptCmd := NewBcryptCmd()
	bcryptCmd.SetArgs([]string{"--password", "a"})
	output := new(bytes.Buffer)
	bcryptCmd.SetOut(output)
	err := bcryptCmd.Execute()
	if err != nil {
		return
	}

	bcryptCmd.SetArgs([]string{"--password", "abc", "--compare", output.String()})
	output.Reset()

	err = bcryptCmd.Execute()
	if err != nil {
		return
	}

	assert.Equal(t, output.String(), "no")
}

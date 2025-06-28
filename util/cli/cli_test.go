package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptPassword_Fallback(t *testing.T) {
	oldStdin := os.Stdin
	defer func() {
		os.Stdin = oldStdin
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	_, err = w.WriteString("test-password\n")
	if err != nil {
		t.Fatalf("Failed to write to pipe: %v", err)
	}
	w.Close()

	os.Stdin = r
	password := PromptPassword("")
	require.Equal(t, "test-password", password)
}

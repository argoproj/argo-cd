package dex

import (
	"testing"
)

func TestRandString(t *testing.T) {
	var ss string
	var err error

	ss, err = randString(10, "A")
	if err != nil {
		t.Fatalf("Could not generate entropy: %v", err)
	}
	if ss != "AAAAAAAAAA" {
		t.Errorf("Expected 10 As, but got %q", ss)
	}

	ss, err = randString(5, "ABC123")
	if err != nil {
		t.Fatalf("Could not generate entropy: %v", err)
	}
	if len(ss) != 5 {
		t.Errorf("Expected random string of length 10, but got %q", ss)
	}
}

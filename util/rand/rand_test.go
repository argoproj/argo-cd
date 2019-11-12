package rand

import (
	"testing"
)

func TestRandString(t *testing.T) {
	ss := RandStringCharset(10, "A")
	if ss != "AAAAAAAAAA" {
		t.Errorf("Expected 10 As, but got %q", ss)
	}
	ss = RandStringCharset(5, "ABC123")
	if len(ss) != 5 {
		t.Errorf("Expected random string of length 10, but got %q", ss)
	}
}

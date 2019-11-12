package misc

import (
	"testing"
)

func TestMakeSignature(t *testing.T) {
	for size := 1; size <= 64; size++ {
		s, err := MakeSignature(size)
		if err != nil {
			t.Errorf("Could not generate signature of size %d: %v", size, err)
		}
		t.Logf("Generated token: %v", s)
	}
}

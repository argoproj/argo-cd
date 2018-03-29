package util

import (
	"testing"
)

func TestSessionManager(t *testing.T) {
	const (
		defaultSecretKey = "Hello, world!"
		defaultSubject   = "argo"
	)
	mgr := MakeSessionManager(defaultSecretKey)

	token, err := mgr.Create(defaultSubject)
	if err != nil {
		t.Errorf("Could not create token: %v", err)
	}

	claims, err := mgr.Parse(token)
	if err != nil {
		t.Errorf("Could not parse token: %v", err)
	}

	subject := claims.Subject
	if subject != "argo" {
		t.Errorf("Token claim subject \"%s\" does not match expected subject \"%s\".", subject, defaultSubject)
	}
}

func TestMakeSignature(t *testing.T) {
	for size := 1; size <= 64; size++ {
		_, err := makeSignature(size)
		if err != nil {
			t.Errorf("Could not generate signature of size %d: %v", size, err)
		}
	}
}

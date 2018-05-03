package session

import (
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
)

func TestSessionManager(t *testing.T) {
	const (
		defaultSecretKey = "Hello, world!"
		defaultSubject   = "argo"
	)
	mgr := SessionManager{[]byte(defaultSecretKey)}

	token, err := mgr.Create(defaultSubject)
	if err != nil {
		t.Errorf("Could not create token: %v", err)
	}

	claims, err := mgr.Parse(token)
	if err != nil {
		t.Errorf("Could not parse token: %v", err)
	}

	mapClaims := *(claims.(*jwt.MapClaims))
	subject := mapClaims["sub"].(string)
	if subject != "argo" {
		t.Errorf("Token claim subject \"%s\" does not match expected subject \"%s\".", subject, defaultSubject)
	}
}

func TestMakeSignature(t *testing.T) {
	for size := 1; size <= 64; size++ {
		s, err := MakeSignature(size)
		if err != nil {
			t.Errorf("Could not generate signature of size %d: %v", size, err)
		}
		t.Logf("Generated token: %v", s)
	}
}

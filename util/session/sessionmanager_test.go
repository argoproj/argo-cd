package session

import (
	"testing"

	"github.com/argoproj/argo-cd/util/settings"
	jwt "github.com/dgrijalva/jwt-go"
)

func TestSessionManager(t *testing.T) {
	const (
		defaultSecretKey = "Hello, world!"
		defaultSubject   = "argo"
	)
	set := settings.ArgoCDSettings{
		ServerSignature: []byte(defaultSecretKey),
	}
	mgr := NewSessionManager(&set)

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

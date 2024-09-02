package security

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// parseJWT parses a jwt and returns it as json bytes.
//
// This function DOES NOT VERIFY THE TOKEN. You still have to verify the token to confirm that the token holder has not
// altered the claims.
//
// This code is copied almost verbatim from go-oidc (https://github.com/coreos/go-oidc).
func parseJWT(p string) ([]byte, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed jwt, expected 3 parts got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed jwt payload: %w", err)
	}
	return payload, nil
}

type audience []string

// UnmarshalJSON allows us to unmarshal either a single audience or a list of audiences.
// Taken from: https://github.com/coreos/go-oidc/blob/a8ceb9a2043fca2e43518633920db746808b1138/oidc/oidc.go#L475
func (a *audience) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*a = audience{s}
		return nil
	}
	var auds []string
	if err := json.Unmarshal(b, &auds); err != nil {
		return err
	}
	*a = auds
	return nil
}

// jwtWithOnlyAudClaim represents a jwt where only the "aud" claim is present. This struct allows us to unmarshal a jwt
// and be confident that the only information retrieved from that jwt is the "aud" claim.
type jwtWithOnlyAudClaim struct {
	Aud audience `json:"aud"`
}

// getUnverifiedAudClaim gets the "aud" claim from a jwt.
//
// This function DOES NOT VERIFY THE TOKEN. You still have to verify the token to confirm that the token holder has not
// altered the "aud" claim.
//
// This code is copied almost verbatim from go-oidc (https://github.com/coreos/go-oidc).
func getUnverifiedAudClaim(rawIDToken string) ([]string, error) {
	payload, err := parseJWT(rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("malformed jwt: %w", err)
	}
	var token jwtWithOnlyAudClaim
	if err = json.Unmarshal(payload, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}
	return token.Aud, nil
}

// UnverifiedHasAudClaim returns whether the "aud" claim is present in the given JWT.
//
// This function DOES NOT VERIFY THE TOKEN. You still have to verify the token to confirm that the token holder has not
// altered the "aud" claim.
func UnverifiedHasAudClaim(rawIDToken string) (bool, error) {
	aud, err := getUnverifiedAudClaim(rawIDToken)
	if err != nil {
		return false, fmt.Errorf("failed to determine whether token had an audience claim: %w", err)
	}
	return aud != nil, nil
}

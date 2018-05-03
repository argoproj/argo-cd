package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	util_password "github.com/argoproj/argo-cd/util/password"
	jwt "github.com/dgrijalva/jwt-go"
)

// SessionManager generates and validates JWT tokens for login sessions.
type SessionManager struct {
	serverSecretKey []byte
}

const (
	// sessionManagerClaimsIssuer fills the "iss" field of the token.
	sessionManagerClaimsIssuer = "argocd"

	// invalidLoginError, for security purposes, doesn't say whether the username or password was invalid.  This does not mitigate the potential for timing attacks to determine which is which.
	invalidLoginError  = "Invalid username or password"
	blankPasswordError = "Blank passwords are not allowed"
)

// MakeSessionManager creates a new session manager with the given secret key.
func MakeSessionManager(secretKey []byte) SessionManager {
	return SessionManager{
		serverSecretKey: secretKey,
	}
}

// Create creates a new token for a given subject (user) and returns it as a string.
func (mgr SessionManager) Create(subject string) (string, error) {
	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	now := time.Now().Unix()
	claims := jwt.StandardClaims{
		//ExpiresAt: time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
		IssuedAt:  now,
		Issuer:    sessionManagerClaimsIssuer,
		NotBefore: now,
		Subject:   subject,
	}
	return mgr.SignClaims(claims)
}

func (mgr SessionManager) SignClaims(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(mgr.serverSecretKey)
}

// Parse tries to parse the provided string and returns the token claims.
func (mgr SessionManager) Parse(tokenString string) (jwt.Claims, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	var claims jwt.MapClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return mgr.serverSecretKey, nil
	})
	if err != nil {
		return nil, err
	}
	return token.Claims, nil
}

// MakeSignature generates a cryptographically-secure pseudo-random token, based on a given number of random bytes, for signing purposes.
func MakeSignature(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		b = nil
	}
	// base64 encode it so signing key can be typed into validation utilities
	b = []byte(base64.StdEncoding.EncodeToString(b))
	return b, err
}

// LoginLocalUser checks if a username/password combo is correct and creates a new token if so.
// [TODO] This may belong elsewhere.
func (mgr SessionManager) LoginLocalUser(username, password string, users map[string]string) (string, error) {
	if password == "" {
		err := fmt.Errorf(blankPasswordError)
		return "", err
	}

	passwordHash, ok := users[username]
	if !ok {
		// Username was not found in local user store.
		// Ensure we still send password to hashing algorithm for comparison.
		// This mitigates potential for timing attacks that benefit from short-circuiting,
		// provided the hashing library/algorithm in use doesn't itself short-circuit.
		passwordHash = ""
	}

	if valid, _ := util_password.VerifyPassword(password, passwordHash); valid {
		token, err := mgr.Create(username)
		if err == nil {
			return token, nil
		}
	}

	return "", fmt.Errorf(invalidLoginError)
}

// MakeCookieMetadata generates a string representing a Web cookie.  Yum!
func MakeCookieMetadata(key, value string, flags ...string) string {
	components := []string{
		fmt.Sprintf("%s=%s", key, value),
	}
	components = append(components, flags...)
	return strings.Join(components, "; ")
}

package session

import (
	"crypto/rand"
	"fmt"
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

// SessionManagerTokenClaims holds claim metadata for a token.
type SessionManagerTokenClaims struct {
	//Foo string `json:"foo"`
	jwt.StandardClaims
}

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
	claims := SessionManagerTokenClaims{
		//"bar",
		jwt.StandardClaims{
			//ExpiresAt: time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
			IssuedAt:  now,
			Issuer:    sessionManagerClaimsIssuer,
			NotBefore: now,
			Subject:   subject,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Unix and get the complete encoded token as a string using the secret
	return token.SignedString(mgr.serverSecretKey)
}

// Parse tries to parse the provided string and returns the token claims.
func (mgr SessionManager) Parse(tokenString string) (*SessionManagerTokenClaims, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	token, err := jwt.ParseWithClaims(tokenString, &SessionManagerTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return mgr.serverSecretKey, nil
	})

	if claims, ok := token.Claims.(*SessionManagerTokenClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, err
}

// MakeSignature generates a cryptographically-secure pseudo-random token, based on a given number of random bytes, for signing purposes.
func MakeSignature(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		b = nil
	}
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

package pkce

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/rand"
)

// PKCEManager handles code verifications and their associated auth tokens
type PKCEManager struct {
	storage PKCEStateStorage
}

type PKCECodes struct {
	CodeVerifier      string
	CodeChallenge     string
	AuthCode          string
	Nonce             string
	CodeChallengeHash string
}

func GeneratePKCECodes() *PKCECodes {
	// PKCE implementation of https://tools.ietf.org/html/rfc7636
	codeVerifier, err := rand.StringFromCharset(
		43,
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~",
	)
	errors.CheckError(err)
	codeChallengeHash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(codeChallengeHash[:])
	return &PKCECodes{
		CodeVerifier:      codeVerifier,
		CodeChallenge:     codeChallenge,
		CodeChallengeHash: "S256",
	}
}

// PKCEManager creates a new pkce manager
func NewPKCEManager(storage PKCEStateStorage) *PKCEManager {
	s := PKCEManager{
		storage: storage,
	}
	return &s
}

func (mgr *PKCEManager) StorePKCEEntry(ctx context.Context, pkceCodes *PKCECodes, expiringAt time.Duration) error {
	return mgr.storage.StorePKCEEntry(ctx, pkceCodes, expiringAt)
}

func (mgr *PKCEManager) RetrieveVerifierCode(nonce string) string {
	return mgr.storage.RetrieveCodeVerifier(nonce)
}

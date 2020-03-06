package cert

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	executil "github.com/argoproj/argo-cd/util/exec"
)

// Regular expression to match public key beginning
var subTypeMatch = regexp.MustCompile(`^pub\s+([a-z0-9]+)\s\d+-\d+-\d+\s\[[A-Z]+\].*$`)

// Regular expression to match key ID output from gpg
var keyIdMatch = regexp.MustCompile(`^\s+([0-9A-Za-z]+)\s*$`)

// Regular expression to match identity output from gpg
var uidMatch = regexp.MustCompile(`^uid\s*\[\s*([a-z]+)\s*\]\s+(.*)$`)

// Regular expression to match import status
var importMatch = regexp.MustCompile(`^gpg: key ([A-Z0-9]+): public key "([^"]+)" imported$`)

// Regular expression to match the start of a commit signature verification
var verificationStartMatch = regexp.MustCompile(`^gpg: Signature made ([a-zA-Z0-9\ :]+)$`)

// Regular expression to match the key ID of a commit signature verification
var verificationKeyIDMatch = regexp.MustCompile(`^gpg:\s+using\s([A-Za-z]+)\skey\s([a-zA-Z0-9]+)$`)

// Regular expression to match the signature status of a commit signature verification
var verificationStatusMatch = regexp.MustCompile(`^gpg: ([a-zA-Z]+) signature from "([^"]+)" \[([a-zA-Z]+)\]$`)

// This is the recipe for automatic key generation, passed to gpg --batch --generate-key
// for initializing our keyring with a trustdb. A new private key will be generated each
// time argocd-server starts, so it's transient and is not used for anything except for
// creating the trustdb in a specific argocd-server and argocd-repo-server pod.
var batchKeyCreateRecipe = `%no-protection
%transient-key
Key-Type: default
Key-Length: 2048
Key-Usage: sign
Name-Real: Anon Ymous
Name-Comment: ArgoCD key signing key
Name-Email: noreply@argoproj.io
Expire-Date: 6m
%commit
`

// A representation of a GnuPG public key
type GnuPGPublicKey struct {
	// KeyID in hexadecimal string format
	KeyID string
	// Fingerprint of the key
	Fingerprint string
	// Owner identification
	Owner string
	// Trust level
	Trust string
	// Key sub type (e.g. rsa4096)
	SubType string
}

// Result of a git commit verification
type PGPVerifyResult struct {
	// Date the signature was made
	Date string
	// KeyID the signature was made with
	KeyID string
	// Identity
	Identity string
	// Trust level of the key
	Trust string
	// Cipher of the key the signature was made with
	Cipher string
	// Result of verification - "unknown", "good" or "bad"
	Result string
}

var pgpTrustLevels = map[string]int{
	"unknown":    2,
	"never":      3,
	"marginally": 4,
	"full":       5,
	"ultimate":   6,
}

// Default value for GNUPGHOME
const DefaultGnuPgHomePath = "/app/config/gpg"

func GetGnuPGHomePath() string {
	if gnuPgHome := os.Getenv("GNUPGHOME"); gnuPgHome == "" {
		return DefaultGnuPgHomePath
	} else {
		return gnuPgHome
	}
}

func getGPGEnviron() []string {
	if h := os.Getenv("GNUPGHOME"); h != "" {
		return append(os.Environ(), GetGnuPGHomePath())
	}
	return os.Environ()
}

// InitializePGP will initialize a GnuPG working directory and also create a
// transient private key so that the trust DB will work correctly.
func InitializeGnuPG() error {

	gnuPgHome := GetGnuPGHomePath()

	// We only operate if GNUPGHOME is set
	if gnuPgHome == "" {
		return fmt.Errorf("GNUPGHOME is not set; refusing to initialize")
	}

	// Directory set in GNUPGHOME must exist and has to be a directory
	st, err := os.Stat(gnuPgHome)
	if err != nil {
		return err
	}

	if !st.IsDir() {
		return fmt.Errorf("GNUPGHOME does not point to a directory")
	}

	_, err = os.Stat(path.Join(gnuPgHome, "pubring.kbx"))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		// We can't initialize a second time
		return fmt.Errorf("GNUPGHOME at %s already initialized, can't initialize again.", gnuPgHome)
	}

	f, err := ioutil.TempFile("", "gpg-key-recipe")
	if err != nil {
		return err
	}

	defer os.Remove(f.Name())

	_, err = f.WriteString(batchKeyCreateRecipe)
	if err != nil {
		return err
	}

	f.Close()

	cmd := exec.Command("gpg", "--logger-fd", "1", "--batch", "--generate-key", f.Name())
	cmd.Env = getGPGEnviron()

	_, err = executil.Run(cmd)
	return err
}

// ImportPGPKey imports one or more keys from a file into the local keyring and optionally
// signs them with the transient private key for leveraging the trust DB.
func ImportPGPKeys(keyFile string) ([]*GnuPGPublicKey, error) {
	keys := make([]*GnuPGPublicKey, 0)

	cmd := exec.Command("gpg", "--logger-fd", "1", "--import", keyFile)
	cmd.Env = getGPGEnviron()

	out, err := executil.Run(cmd)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), "gpg: ") {
			continue
		}
		// We ignore lines that are not of interest
		token := importMatch.FindStringSubmatch(scanner.Text())
		if len(token) != 3 {
			continue
		}

		key := GnuPGPublicKey{
			KeyID: token[1],
			Owner: token[2],
			// By default, trust level is unknown
			Trust: "unknown",
			// Subtype is unknown at this point
			SubType:     "unknown",
			Fingerprint: "",
		}

		keys = append(keys, &key)
	}

	return keys, nil
}

// SetPGPTrustLevel sets the given trust level on keys with specified key IDs
func SetPGPTrustLevelById(kids []string, trustLevel string) error {
	keys := make([]*GnuPGPublicKey, 0)
	for _, kid := range kids {
		keys = append(keys, &GnuPGPublicKey{KeyID: kid})
	}
	return SetPGPTrustLevel(keys, trustLevel)
}

// SetPGPTrustLevel sets the given trust level on specified keys
func SetPGPTrustLevel(pgpKeys []*GnuPGPublicKey, trustLevel string) error {
	trust, ok := pgpTrustLevels[trustLevel]
	if !ok {
		return fmt.Errorf("Unknown trust level: %s", trustLevel)
	}

	// We need to store ownertrust specification in a temp file. Format is <fingerprint>:<level>
	f, err := ioutil.TempFile("", "gpg-key-fps")
	if err != nil {
		return err
	}

	defer os.Remove(f.Name())

	for _, k := range pgpKeys {
		_, err := f.WriteString(fmt.Sprintf("%s:%d\n", k.KeyID, trust))
		if err != nil {
			return err
		}
	}

	f.Close()

	// Load ownertrust from the file we have constructed and instruct gpg to update the trustdb
	cmd := exec.Command("gpg", "--import-ownertrust", f.Name())
	cmd.Env = getGPGEnviron()

	_, err = executil.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

// DeletePGPKey deletes a key from our GnuPG key ring
func DeletePGPKey(keyID string) error {
	args := append([]string{}, "--yes", "--batch", "--delete-keys", keyID)
	cmd := exec.Command("gpg", args...)
	cmd.Env = getGPGEnviron()

	_, err := executil.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

// GetInstalledPGPKeys() runs gpg to retrieve public keys from our keyring. If kids is non-empty, limit result to those key IDs
func GetInstalledPGPKeys(kids []string) ([]*GnuPGPublicKey, error) {
	keys := make([]*GnuPGPublicKey, 0)

	args := append([]string{}, "--list-public-keys")
	// kids can contain an arbitrary list of key IDs we want to list. If empty, we list all keys.
	if len(kids) > 0 {
		args = append(args, kids...)
	}
	cmd := exec.Command("gpg", args...)
	cmd.Env = getGPGEnviron()

	out, err := executil.Run(cmd)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	var curKey *GnuPGPublicKey = nil
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "pub ") {
			// This is the beginning of a new key, time to store the previously parsed one in our list and start fresh.
			if curKey != nil {
				keys = append(keys, curKey)
				curKey = nil
			}

			key := GnuPGPublicKey{}

			// Second field in pub output denotes key sub type (cipher and length)
			token := subTypeMatch.FindStringSubmatch(scanner.Text())
			if len(token) != 2 {
				return nil, fmt.Errorf("Invalid line: %s (len=%d)", scanner.Text(), len(token))
			}
			key.SubType = token[1]

			// Next line should be the key ID, no prefix
			if !scanner.Scan() {
				return nil, fmt.Errorf("Invalid output from gpg, end of text after primary key")
			}

			token = keyIdMatch.FindStringSubmatch(scanner.Text())
			if len(token) != 2 {
				return nil, fmt.Errorf("Invalid output from gpg, no key ID for primary key")
			}

			key.Fingerprint = token[1]
			// KeyID is just the last bytes of the fingerprint
			key.KeyID = token[1][24:]

			if curKey == nil {
				curKey = &key
			}

			// Next line should be UID
			if !scanner.Scan() {
				return nil, fmt.Errorf("Invalid output from gpg, end of text after key ID")
			}

			if !strings.HasPrefix(scanner.Text(), "uid ") {
				return nil, fmt.Errorf("Invalid output from gpg, no identity for primary key")
			}

			token = uidMatch.FindStringSubmatch(scanner.Text())

			if len(token) < 3 {
				return nil, fmt.Errorf("Malformed identity line: %s (len=%d)", scanner.Text(), len(token))
			}

			// Store trust level
			key.Trust = token[1]

			// Identity - we are only interested in the first uid
			key.Owner = token[2]
		}
	}

	// Also store the last processed key into our list to be returned
	if curKey != nil {
		keys = append(keys, curKey)
	} else {
		// This probably means invalid/incomplete output. Not a single key was found.
		// FIXME: do we have to handle this, or just ignore it like we do now?
	}

	return keys, nil
}

// ParsePGPCommitSignature parses the output of "git verify-commit" and returns the result
func ParseGitCommitVerification(signature string) (PGPVerifyResult, error) {
	result := PGPVerifyResult{Result: "unknown"}
	parseOk := false

	scanner := bufio.NewScanner(strings.NewReader(signature))
	for scanner.Scan() {
		// Indicating the beginning of a signature
		start := verificationStartMatch.FindStringSubmatch(scanner.Text())
		if len(start) == 2 {
			result.Date = start[1]
			if !scanner.Scan() {
				return PGPVerifyResult{}, fmt.Errorf("Unexpected end-of-file while parsing commit verification output.")
			}
			// What key has made the signature?
			keyID := verificationKeyIDMatch.FindStringSubmatch(scanner.Text())
			if len(keyID) != 3 {
				return PGPVerifyResult{}, fmt.Errorf("Could not parse key ID of commit verification output.")
			}

			result.Cipher = keyID[1]
			result.KeyID = keyID[2]

			// What was the result of signature verification?
			if !scanner.Scan() {
				return PGPVerifyResult{}, fmt.Errorf("Unexpected end-of-file while parsing commit verification output.")
			}

			if strings.HasPrefix(scanner.Text(), "gpg: Can't check signature: ") {
				return PGPVerifyResult{}, fmt.Errorf(strings.SplitN(scanner.Text(), ":", 2)[1])
			} else {
				sigState := verificationStatusMatch.FindStringSubmatch(scanner.Text())
				if len(sigState) != 4 {
					return PGPVerifyResult{}, fmt.Errorf("Could not parse result of verify operation.")
				}
				result.Result = sigState[1]
				result.Identity = sigState[2]
				result.Trust = sigState[3]
			}
			parseOk = true
			break
		}
	}

	if parseOk {
		return result, nil
	} else {
		return PGPVerifyResult{}, fmt.Errorf("Could not parse output of verify-commit, no verification data found.")
	}
}

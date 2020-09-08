package db

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/gpg"
)

// Validates a single GnuPG key and returns the key's ID
func validatePGPKey(keyData string) (*appsv1.GnuPGPublicKey, error) {
	f, err := ioutil.TempFile("", "gpg-public-key")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())

	err = ioutil.WriteFile(f.Name(), []byte(keyData), 0600)
	if err != nil {
		return nil, err
	}
	f.Close()

	parsed, err := gpg.ValidatePGPKeys(f.Name())
	if err != nil {
		return nil, err
	}

	// Each key/value pair in the config map must exactly contain one public key, with the (short) GPG key ID as key
	if len(parsed) != 1 {
		return nil, fmt.Errorf("More than one key found in input data")
	}

	var retKey *appsv1.GnuPGPublicKey = nil
	// Is there a better way to get the first element from a map without knowing its key?
	for _, k := range parsed {
		retKey = k
		break
	}
	if retKey != nil {
		retKey.KeyData = keyData
		return retKey, nil
	} else {
		return nil, fmt.Errorf("Could not find the GPG key")
	}
}

// ListConfiguredGPGPublicKeys returns a list of all configured GPG public keys from the ConfigMap
func (db *db) ListConfiguredGPGPublicKeys(ctx context.Context) (map[string]*appsv1.GnuPGPublicKey, error) {
	log.Debugf("Loading PGP public keys from config map")
	result := make(map[string]*appsv1.GnuPGPublicKey)
	keysCM, err := db.settingsMgr.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
	if err != nil {
		return nil, err
	}

	// We have to verify all PGP keys in the ConfigMap to be valid keys before. To do so,
	// we write each single one out to a temporary file and validate them through gpg.
	// This is not optimal, but the executil from argo-pkg does not support writing to
	// stdin of the forked process. So for now, we must live with that.
	for k, p := range keysCM.Data {
		if expectedKeyID := gpg.KeyID(k); expectedKeyID != "" {
			parsedKey, err := validatePGPKey(p)
			if err != nil {
				return nil, fmt.Errorf("Could not parse GPG key for entry '%s': %s", expectedKeyID, err.Error())
			}
			if expectedKeyID != parsedKey.KeyID {
				return nil, fmt.Errorf("Key parsed for entry with key ID '%s' had different key ID '%s'", expectedKeyID, parsedKey.KeyID)
			}
			result[parsedKey.KeyID] = parsedKey
		} else {
			return nil, fmt.Errorf("Found entry with key '%s' in ConfigMap, but this is not a valid PGP key ID", k)
		}
	}

	return result, nil
}

// AddGPGPublicKey adds one or more public keys to the configuration
func (db *db) AddGPGPublicKey(ctx context.Context, keyData string) (map[string]*appsv1.GnuPGPublicKey, []string, error) {
	result := make(map[string]*appsv1.GnuPGPublicKey)
	skipped := make([]string, 0)

	keys, err := gpg.ValidatePGPKeysFromString(keyData)
	if err != nil {
		return nil, nil, err
	}

	keysCM, err := db.settingsMgr.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
	if err != nil {
		return nil, nil, err
	}

	if keysCM.Data == nil {
		keysCM.Data = make(map[string]string)
	}

	for kid, key := range keys {
		if _, ok := keysCM.Data[kid]; ok {
			skipped = append(skipped, kid)
			log.Debugf("Not adding incoming key with kid=%s because it is configured already", kid)
		} else {
			result[kid] = key
			keysCM.Data[kid] = key.KeyData
			log.Debugf("Adding incoming key with kid=%s to database", kid)
		}
	}

	err = db.settingsMgr.SaveGPGPublicKeyData(ctx, keysCM.Data)
	if err != nil {
		return nil, nil, err
	}

	return result, skipped, nil
}

// DeleteGPGPublicKey deletes a GPG public key from the configuration
func (db *db) DeleteGPGPublicKey(ctx context.Context, keyID string) error {
	keysCM, err := db.settingsMgr.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
	if err != nil {
		return err
	}

	if keysCM.Data == nil {
		return fmt.Errorf("No such key configured: %s", keyID)
	}

	if _, ok := keysCM.Data[keyID]; !ok {
		return fmt.Errorf("No such key configured: %s", keyID)
	}

	delete(keysCM.Data, keyID)

	err = db.settingsMgr.SaveGPGPublicKeyData(ctx, keysCM.Data)
	return err
}

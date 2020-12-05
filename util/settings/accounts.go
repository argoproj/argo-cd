package settings

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/common"
)

const (
	accountsKeyPrefix          = "accounts"
	accountPasswordSuffix      = "password"
	accountPasswordMtimeSuffix = "passwordMtime"
	accountEnabledSuffix       = "enabled"
	accountTokensSuffix        = "tokens"

	// Admin superuser password storage
	// settingAdminPasswordHashKey designates the key for a root password hash inside a Kubernetes secret.
	settingAdminPasswordHashKey = "admin.password"
	// settingAdminPasswordMtimeKey designates the key for a root password mtime inside a Kubernetes secret.
	settingAdminPasswordMtimeKey = "admin.passwordMtime"
	settingAdminEnabledKey       = "admin.enabled"
	settingAdminTokensKey        = "admin.tokens"
)

type AccountCapability string

const (
	// AccountCapabilityLogin represents capability to create UI session tokens.
	AccountCapabilityLogin AccountCapability = "login"
	// AccountCapabilityLogin represents capability to generate API auth tokens.
	AccountCapabilityApiKey AccountCapability = "apiKey"
)

// Token holds the information about the generated auth token.
type Token struct {
	ID        string `json:"id"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp,omitempty"`
}

// Account holds local account information
type Account struct {
	PasswordHash  string
	PasswordMtime *time.Time
	Enabled       bool
	Capabilities  []AccountCapability
	Tokens        []Token
}

// FormatPasswordMtime return the formatted password modify time or empty string of password modify time is nil.
func (a *Account) FormatPasswordMtime() string {
	if a.PasswordMtime == nil {
		return ""
	}
	return a.PasswordMtime.Format(time.RFC3339)
}

// FormatCapabilities returns comma separate list of user capabilities.
func (a *Account) FormatCapabilities() string {
	var items []string
	for i := range a.Capabilities {
		items = append(items, string(a.Capabilities[i]))
	}
	return strings.Join(items, ",")
}

// TokenIndex return an index of a token with the given identifier or -1 if token not found.
func (a *Account) TokenIndex(id string) int {
	for i := range a.Tokens {
		if a.Tokens[i].ID == id {
			return i
		}
	}
	return -1
}

// HasCapability return true if the account has the specified capability.
func (a *Account) HasCapability(capability AccountCapability) bool {
	for _, c := range a.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

func (mgr *SettingsManager) saveAccount(name string, account Account) error {
	return mgr.updateSecret(func(secret *v1.Secret) error {
		return mgr.updateConfigMap(func(cm *v1.ConfigMap) error {
			return saveAccount(secret, cm, name, account)
		})
	})
}

// AddAccount save an account with the given name and properties.
func (mgr *SettingsManager) AddAccount(name string, account Account) error {
	accounts, err := mgr.GetAccounts()
	if err != nil {
		return err
	}
	if _, ok := accounts[name]; ok {
		return status.Errorf(codes.AlreadyExists, "account '%s' already exists", name)
	}
	return mgr.saveAccount(name, account)
}

// GetAccount return an account info by the specified name.
func (mgr *SettingsManager) GetAccount(name string) (*Account, error) {
	accounts, err := mgr.GetAccounts()
	if err != nil {
		return nil, err
	}
	account, ok := accounts[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "account '%s' does not exist", name)
	}
	return &account, nil
}

// UpdateAccount runs the callback function against an account that matches to the specified name
//and persist changes applied by the callback.
func (mgr *SettingsManager) UpdateAccount(name string, callback func(account *Account) error) error {
	account, err := mgr.GetAccount(name)
	if err != nil {
		return err
	}
	err = callback(account)
	if err != nil {
		return err
	}
	return mgr.saveAccount(name, *account)
}

// GetAccounts returns list of configured accounts
func (mgr *SettingsManager) GetAccounts() (map[string]Account, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	secret, err := mgr.secrets.Secrets(mgr.namespace).Get(common.ArgoCDSecretName)
	if err != nil {
		return nil, err
	}
	cm, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName)
	if err != nil {
		return nil, err
	}
	return parseAccounts(secret, cm)
}

func updateAccountMap(cm *v1.ConfigMap, key string, val string, defVal string) {
	existingVal := cm.Data[key]
	if existingVal != val {
		if val == "" || val == defVal {
			delete(cm.Data, key)
		} else {
			cm.Data[key] = val
		}
	}
}

func updateAccountSecret(secret *v1.Secret, key string, val string, defVal string) {
	existingVal := string(secret.Data[key])
	if existingVal != val {
		if val == "" || val == defVal {
			delete(secret.Data, key)
		} else {
			secret.Data[key] = []byte(val)
		}
	}
}

func saveAccount(secret *v1.Secret, cm *v1.ConfigMap, name string, account Account) error {
	tokens, err := json.Marshal(account.Tokens)
	if err != nil {
		return err
	}
	if name == common.ArgoCDAdminUsername {
		updateAccountSecret(secret, settingAdminPasswordHashKey, account.PasswordHash, "")
		updateAccountSecret(secret, settingAdminPasswordMtimeKey, account.FormatPasswordMtime(), "")
		updateAccountSecret(secret, settingAdminTokensKey, string(tokens), "[]")
		updateAccountMap(cm, settingAdminEnabledKey, strconv.FormatBool(account.Enabled), "true")
	} else {
		updateAccountSecret(secret, fmt.Sprintf("%s.%s.%s", accountsKeyPrefix, name, accountPasswordSuffix), account.PasswordHash, "")
		updateAccountSecret(secret, fmt.Sprintf("%s.%s.%s", accountsKeyPrefix, name, accountPasswordMtimeSuffix), account.FormatPasswordMtime(), "")
		updateAccountSecret(secret, fmt.Sprintf("%s.%s.%s", accountsKeyPrefix, name, accountTokensSuffix), string(tokens), "[]")
		updateAccountMap(cm, fmt.Sprintf("%s.%s.%s", accountsKeyPrefix, name, accountEnabledSuffix), strconv.FormatBool(account.Enabled), "true")
		updateAccountMap(cm, fmt.Sprintf("%s.%s", accountsKeyPrefix, name), account.FormatCapabilities(), "")
	}
	return nil
}

func parseAdminAccount(secret *v1.Secret, cm *v1.ConfigMap) (*Account, error) {
	adminAccount := &Account{Enabled: true, Capabilities: []AccountCapability{AccountCapabilityLogin}}
	if adminPasswordHash, ok := secret.Data[settingAdminPasswordHashKey]; ok {
		adminAccount.PasswordHash = string(adminPasswordHash)
	}
	if adminPasswordMtimeBytes, ok := secret.Data[settingAdminPasswordMtimeKey]; ok {
		if mTime, err := time.Parse(time.RFC3339, string(adminPasswordMtimeBytes)); err == nil {
			adminAccount.PasswordMtime = &mTime
		}
	}

	adminAccount.Tokens = make([]Token, 0)
	if tokensStr, ok := secret.Data[settingAdminTokensKey]; ok && string(tokensStr) != "" {
		if err := json.Unmarshal(tokensStr, &adminAccount.Tokens); err != nil {
			return nil, err
		}
	}

	if enabledStr, ok := cm.Data[settingAdminEnabledKey]; ok {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			adminAccount.Enabled = enabled
		} else {
			log.Warnf("ConfigMap has invalid key %s: %v", settingAdminTokensKey, err)
		}
	}

	return adminAccount, nil
}

func parseAccounts(secret *v1.Secret, cm *v1.ConfigMap) (map[string]Account, error) {
	adminAccount, err := parseAdminAccount(secret, cm)
	if err != nil {
		return nil, err
	}
	accounts := map[string]Account{
		common.ArgoCDAdminUsername: *adminAccount,
	}

	for key, v := range cm.Data {
		if !strings.HasPrefix(key, fmt.Sprintf("%s.", accountsKeyPrefix)) {
			continue
		}

		val := v
		var accountName, suffix string

		parts := strings.Split(key, ".")
		switch len(parts) {
		case 2:
			accountName = parts[1]
		case 3:
			accountName = parts[1]
			suffix = parts[2]
		default:
			log.Warnf("Unexpected key %s in ConfigMap '%s'", key, cm.Name)
			continue
		}

		account, ok := accounts[accountName]
		if !ok {
			account = Account{Enabled: true}
			accounts[accountName] = account
		}
		switch suffix {
		case "":
			for _, capability := range strings.Split(val, ",") {
				capability = strings.TrimSpace(capability)
				if capability == "" {
					continue
				}

				switch capability {
				case string(AccountCapabilityLogin):
					account.Capabilities = append(account.Capabilities, AccountCapabilityLogin)
				case string(AccountCapabilityApiKey):
					account.Capabilities = append(account.Capabilities, AccountCapabilityApiKey)
				default:
					log.Warnf("not supported account capability '%s' in config map key '%s'", capability, key)
				}
			}
		case accountEnabledSuffix:
			account.Enabled, err = strconv.ParseBool(val)
			if err != nil {
				return nil, err
			}
		}
		accounts[accountName] = account
	}

	for name, account := range accounts {
		if name == common.ArgoCDAdminUsername {
			continue
		}

		if passwordHash, ok := secret.Data[fmt.Sprintf("%s.%s.%s", accountsKeyPrefix, name, accountPasswordSuffix)]; ok {
			account.PasswordHash = string(passwordHash)
		}
		if passwordMtime, ok := secret.Data[fmt.Sprintf("%s.%s.%s", accountsKeyPrefix, name, accountPasswordMtimeSuffix)]; ok {
			if mTime, err := time.Parse(time.RFC3339, string(passwordMtime)); err != nil {
				return nil, err
			} else {
				account.PasswordMtime = &mTime
			}
		}
		if tokensStr, ok := secret.Data[fmt.Sprintf("%s.%s.%s", accountsKeyPrefix, name, accountTokensSuffix)]; ok {
			account.Tokens = make([]Token, 0)
			if string(tokensStr) != "" {
				if err := json.Unmarshal(tokensStr, &account.Tokens); err != nil {
					log.Errorf("Account '%s' has invalid token in secret '%s'", name, secret.Name)
				}
			}
		}
		accounts[name] = account
	}

	return accounts, nil
}

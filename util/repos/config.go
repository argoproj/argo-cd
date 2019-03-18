package repos

import "errors"

type Config struct {
	Name, Url, Type, Username, Password, SSHPrivateKey string
	InsecureIgnoreHostKey                              bool
	CAData, CertData, KeyData                          []byte
}

func (r Config) Validate() error {
	if r.Url == "" {
		return errors.New("invalid config, must specify Url")
	}

	if r.Type == "helm" {
		if r.Name == "" {
			return errors.New("invalid config, must specify Name")
		}
		if r.SSHPrivateKey != "" {
			return errors.New("invalid config, must not specify SSHPrivateKey")
		}
		if r.InsecureIgnoreHostKey {
			return errors.New("invalid config, must not specify InsecureIgnoreHostKey")
		}
	} else {
		if r.Name != "" || r.CertData != nil || r.CAData != nil || r.KeyData != nil {
			return errors.New("invalid config, must not specify Name, CertData, CAData, or KeyData")
		}
	}

	return nil
}

package localconfig

import (
	"fmt"
	"os"
	"os/user"
	"path"

	"github.com/dgrijalva/jwt-go/v4"

	configUtil "github.com/argoproj/argo-cd/util/config"
)

// LocalConfig is a local Argo CD config file
type LocalConfig struct {
	CurrentContext string       `json:"current-context"`
	Contexts       []ContextRef `json:"contexts"`
	Servers        []Server     `json:"servers"`
	Users          []User       `json:"users"`
}

// ContextRef is a reference to a Server and User for an API client
type ContextRef struct {
	Name   string `json:"name"`
	Server string `json:"server"`
	User   string `json:"user"`
}

// Context is the resolved Server and User objects resolved
type Context struct {
	Name   string
	Server Server
	User   User
}

// Server contains Argo CD server information
type Server struct {
	// Server is the Argo CD server address
	Server string `json:"server"`
	// Insecure indicates to connect to the server over TLS insecurely
	Insecure bool `json:"insecure,omitempty"`
	// GRPCWeb indicates to connect to the server using gRPC Web protocol
	GRPCWeb bool `json:"grpc-web,omitempty"`
	// GRPCWebRootPath indicates to connect to the server using gRPC Web protocol with this root path
	GRPCWebRootPath string `json:"grpc-web-root-path"`
	// CACertificateAuthorityData is the base64 string of a PEM encoded certificate
	// TODO: not yet implemented
	CACertificateAuthorityData string `json:"certificate-authority-data,omitempty"`
	// ClientCertificateData is the base64 string of a PEM encoded certificate used to authenticate the client
	ClientCertificateData string `json:"client-certificate-data,omitempty"`
	// ClientCertificateKeyData is the base64 string of a PEM encoded private key of the client certificate
	ClientCertificateKeyData string `json:"client-certificate-key-data,omitempty"`
	// PlainText indicates to connect with TLS disabled
	PlainText bool `json:"plain-text,omitempty"`
}

// User contains user authentication information
type User struct {
	Name         string `json:"name"`
	AuthToken    string `json:"auth-token,omitempty"`
	RefreshToken string `json:"refresh-token,omitempty"`
}

// Claims returns the standard claims from the JWT claims
func (u *User) Claims() (*jwt.StandardClaims, error) {
	parser := &jwt.Parser{
		ValidationHelper: jwt.NewValidationHelper(jwt.WithoutClaimsValidation()),
	}
	claims := jwt.StandardClaims{}
	_, _, err := parser.ParseUnverified(u.AuthToken, &claims)
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

// ReadLocalConfig loads up the local configuration file. Returns nil if config does not exist
func ReadLocalConfig(path string) (*LocalConfig, error) {
	var err error
	var config LocalConfig
	err = configUtil.UnmarshalLocalFile(path, &config)
	if os.IsNotExist(err) {
		return nil, nil
	}
	err = ValidateLocalConfig(config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func ValidateLocalConfig(config LocalConfig) error {
	if config.CurrentContext == "" {
		return nil
	}
	if _, err := config.ResolveContext(config.CurrentContext); err != nil {
		return fmt.Errorf("Local config invalid: %s", err)
	}
	return nil
}

// WriteLocalConfig writes a new local configuration file.
func WriteLocalConfig(config LocalConfig, configPath string) error {
	err := os.MkdirAll(path.Dir(configPath), os.ModePerm)
	if err != nil {
		return err
	}
	return configUtil.MarshalLocalYAMLFile(configPath, config)
}

func DeleteLocalConfig(configPath string) error {
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		return err
	}
	return os.Remove(configPath)
}

// ResolveContext resolves the specified context. If unspecified, resolves the current context
func (l *LocalConfig) ResolveContext(name string) (*Context, error) {
	if name == "" {
		if l.CurrentContext == "" {
			return nil, fmt.Errorf("Local config: current-context unset")
		}
		name = l.CurrentContext
	}
	for _, ctx := range l.Contexts {
		if ctx.Name == name {
			server, err := l.GetServer(ctx.Server)
			if err != nil {
				return nil, err
			}
			user, err := l.GetUser(ctx.User)
			if err != nil {
				return nil, err
			}
			return &Context{
				Name:   ctx.Name,
				Server: *server,
				User:   *user,
			}, nil
		}
	}
	return nil, fmt.Errorf("Context '%s' undefined", name)
}

func (l *LocalConfig) GetServer(name string) (*Server, error) {
	for _, s := range l.Servers {
		if s.Server == name {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("Server '%s' undefined", name)
}

func (l *LocalConfig) UpsertServer(server Server) {
	for i, s := range l.Servers {
		if s.Server == server.Server {
			l.Servers[i] = server
			return
		}
	}
	l.Servers = append(l.Servers, server)
}

// Returns true if server was removed successfully
func (l *LocalConfig) RemoveServer(serverName string) bool {
	for i, s := range l.Servers {
		if s.Server == serverName {
			l.Servers = append(l.Servers[:i], l.Servers[i+1:]...)
			return true
		}
	}
	return false
}

func (l *LocalConfig) GetUser(name string) (*User, error) {
	for _, u := range l.Users {
		if u.Name == name {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("User '%s' undefined", name)
}

func (l *LocalConfig) UpsertUser(user User) {
	for i, u := range l.Users {
		if u.Name == user.Name {
			l.Users[i] = user
			return
		}
	}
	l.Users = append(l.Users, user)
}

// Returns true if user was removed successfully
func (l *LocalConfig) RemoveUser(serverName string) bool {
	for i, u := range l.Users {
		if u.Name == serverName {
			l.Users = append(l.Users[:i], l.Users[i+1:]...)
			return true
		}
	}
	return false
}

// Returns true if user was removed successfully
func (l *LocalConfig) RemoveToken(serverName string) bool {
	for i, u := range l.Users {
		if u.Name == serverName {
			l.Users[i].RefreshToken = ""
			l.Users[i].AuthToken = ""
			return true
		}
	}
	return false
}

func (l *LocalConfig) UpsertContext(context ContextRef) {
	for i, c := range l.Contexts {
		if c.Name == context.Name {
			l.Contexts[i] = context
			return
		}
	}
	l.Contexts = append(l.Contexts, context)
}

// Returns true if context was removed successfully
func (l *LocalConfig) RemoveContext(serverName string) (string, bool) {
	for i, c := range l.Contexts {
		if c.Name == serverName {
			l.Contexts = append(l.Contexts[:i], l.Contexts[i+1:]...)
			return c.Server, true
		}
	}
	return "", false
}

func (l *LocalConfig) IsEmpty() bool {
	return len(l.Servers) == 0
}

// DefaultConfigDir returns the local configuration path for settings such as cached authentication tokens.
func DefaultConfigDir() (string, error) {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		homeDir = usr.HomeDir
	}
	return path.Join(homeDir, ".argocd"), nil
}

// DefaultLocalConfigPath returns the local configuration path for settings such as cached authentication tokens.
func DefaultLocalConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "config"), nil
}

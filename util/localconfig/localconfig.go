package localconfig

import (
	"fmt"
	"os"
	"os/user"
	"path"

	"github.com/argoproj/argo-cd/util/cli"
)

// LocalConfig is a local ArgoCD config file
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

// Server contains ArgoCD server information
type Server struct {
	// Server is the ArgoCD server address
	Server string `json:"server"`
	// Insecure indicates to connect to the server over TLS insecurely
	Insecure bool `json:"insecure,omitempty"`
	// CACertificateAuthorityData is the base64 string of a PEM encoded certificate
	// TODO: not yet implemented
	CACertificateAuthorityData string `json:"certificate-authority-data,omitempty"`
	// PlainText indicates to connect with TLS disabled
	PlainText bool `json:"plain-text,omitempty"`
}

// User contains user authentication information
type User struct {
	Name      string `json:"name"`
	AuthToken string `json:"auth-token,omitempty"`
}

// ReadLocalConfig loads up the local configuration file. Returns nil if config does not exist
func ReadLocalConfig(path string) (*LocalConfig, error) {
	var err error
	var config LocalConfig
	err = cli.UnmarshalLocalFile(path, &config)
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
		return fmt.Errorf("Local config invalid: current-context unset")
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
	return cli.MarshalLocalYAMLFile(configPath, config)
}

// ResolveContext resolves the specified context. If unspecified, resolves the current context
func (l *LocalConfig) ResolveContext(name string) (*Context, error) {
	if name == "" {
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

func (l *LocalConfig) UpsertContext(context ContextRef) {
	for i, c := range l.Contexts {
		if c.Name == context.Name {
			l.Contexts[i] = context
			return
		}
	}
	l.Contexts = append(l.Contexts, context)
}

// LocalConfigDir returns the local configuration path for settings such as cached authentication tokens.
func localConfigDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(usr.HomeDir, ".argocd"), nil
}

// DefaultLocalConfigPath returns the local configuration path for settings such as cached authentication tokens.
func DefaultLocalConfigPath() (string, error) {
	dir, err := localConfigDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "config"), nil
}

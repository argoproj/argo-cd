package security

import (
	"fmt"
	"slices"
	"sync"

	"github.com/argoproj/argo-cd/v3/util/glob"
)

// ApplicationNamespaceSet is a thread safe wrapper that holds a list of namespaces to indicate what
// namespaces applications can be deployed in
type ApplicationNamespaceSet struct {
	namespaces []string
	// originalNamespaces keeps track of the original namespaces in the list to not remove them if they
	// were provided by flag
	originalNamespaces []string
	mu                 sync.RWMutex
}

// NewApplicationNamespaceSet creates an ApplicationNamespaceSet with a list of namespaces
func NewApplicationNamespaceSet(namespaces []string) *ApplicationNamespaceSet {
	return &ApplicationNamespaceSet{
		namespaces:         append([]string{}, namespaces...),
		originalNamespaces: slices.Clone(namespaces),
		mu:                 sync.RWMutex{},
	}
}

// NewApplicationNamespaceSetWithNoBackup creates a ApplicationNamespaceSet without backing up
// the original namespaces
func NewApplicationNamespaceSetWithNoBackup(namespaces []string) *ApplicationNamespaceSet {
	return &ApplicationNamespaceSet{
		namespaces: append([]string{}, namespaces...),
		mu:         sync.RWMutex{},
	}
}

// Add only appends the namespace if it does not exist in the current list
func (a *ApplicationNamespaceSet) Add(ns string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if slices.Index(a.namespaces, ns) == -1 {
		a.namespaces = append(a.namespaces, ns)
	}
}

func (a *ApplicationNamespaceSet) List() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return slices.Clone(a.namespaces)
}

func (a *ApplicationNamespaceSet) Delete(ns string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if i := slices.Index(a.namespaces, ns); i != -1 && !slices.Contains(a.originalNamespaces, ns) {
		a.namespaces = slices.Delete(a.namespaces, i, i+1)
	}
}

func IsNamespaceEnabled(namespace string, serverNamespace string, enabledNamespaces []string) bool {
	return namespace == serverNamespace || glob.MatchStringInList(enabledNamespaces, namespace, glob.REGEXP)
}

func NamespaceNotPermittedError(namespace string) error {
	return fmt.Errorf("namespace '%s' is not permitted", namespace)
}

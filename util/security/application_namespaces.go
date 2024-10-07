package security

import (
	"fmt"

	"github.com/argoproj/argo-cd/v2/util/glob"
)

func IsNamespaceEnabled(namespace string, serverNamespace string, enabledNamespaces []string) bool {
	return namespace == serverNamespace || glob.MatchStringInList(enabledNamespaces, namespace, glob.REGEXP)
}

func NamespaceNotPermittedError(namespace string) error {
	return fmt.Errorf("namespace '%s' is not permitted", namespace)
}

package settings

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/glob"
)

const (
	// serviceAccountDisallowedCharSet contains the characters that are not allowed to be present
	// in a DefaultServiceAccount configured for a DestinationServiceAccount
	serviceAccountDisallowedCharSet = "!*[]{}\\/"
)

// DeriveServiceAccountToImpersonate determines the service account to be used for impersonation for the sync operation.
// The returned service account will be fully qualified including namespace and the service account name in the format system:serviceaccount:<namespace>:<service_account>
func DeriveServiceAccountToImpersonate(project *v1alpha1.AppProject, application *v1alpha1.Application, destCluster *v1alpha1.Cluster) (string, error) {
	// spec.Destination.Namespace is optional. If not specified, use the Application's
	// namespace
	serviceAccountNamespace := application.Spec.Destination.Namespace
	if serviceAccountNamespace == "" {
		serviceAccountNamespace = application.Namespace
	}
	// Loop through the destinationServiceAccounts and see if there is any destination that is a candidate.
	// if so, return the service account specified for that destination.
	for _, item := range project.Spec.DestinationServiceAccounts {
		dstServerMatched, err := glob.MatchWithError(item.Server, destCluster.Server)
		if err != nil {
			return "", fmt.Errorf("invalid glob pattern for destination server: %w", err)
		}
		dstNamespaceMatched, err := glob.MatchWithError(item.Namespace, application.Spec.Destination.Namespace)
		if err != nil {
			return "", fmt.Errorf("invalid glob pattern for destination namespace: %w", err)
		}
		if dstServerMatched && dstNamespaceMatched {
			if strings.Trim(item.DefaultServiceAccount, " ") == "" || strings.ContainsAny(item.DefaultServiceAccount, serviceAccountDisallowedCharSet) {
				return "", fmt.Errorf("default service account contains invalid chars '%s'", item.DefaultServiceAccount)
			} else if strings.Contains(item.DefaultServiceAccount, ":") {
				// service account is specified along with its namespace.
				return "system:serviceaccount:" + item.DefaultServiceAccount, nil
			}
			// service account needs to be prefixed with a namespace
			return fmt.Sprintf("system:serviceaccount:%s:%s", serviceAccountNamespace, item.DefaultServiceAccount), nil
		}
	}
	// if there is no match found in the AppProject.Spec.DestinationServiceAccounts, use the default service account of the destination namespace.
	return "", fmt.Errorf("no matching service account found for destination server %s and namespace %s", application.Spec.Destination.Server, serviceAccountNamespace)
}

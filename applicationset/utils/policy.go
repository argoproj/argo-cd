package utils

import (
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// Policies is a registry of available policies.
var Policies = map[string]argov1alpha1.ApplicationsSyncPolicy{
	"create-only":   argov1alpha1.ApplicationsSyncPolicyPolicyCreateOnly,
	"create-update": argov1alpha1.ApplicationsSyncPolicyPolicyCreateUpdate,
	"create-delete": argov1alpha1.ApplicationsSyncPolicyPolicyCreateDelete,
	"sync":          argov1alpha1.ApplicationsSyncPolicyPolicySync,
	// Default is "sync"
	"": argov1alpha1.ApplicationsSyncPolicyPolicySync,
}

func DefaultPolicy(appSetSyncPolicy *argov1alpha1.ApplicationSetSyncPolicy, defaultPolicy argov1alpha1.ApplicationsSyncPolicy, allowPolicyOverride bool) argov1alpha1.ApplicationsSyncPolicy {
	if appSetSyncPolicy == nil || appSetSyncPolicy.ApplicationsSync == nil || !allowPolicyOverride {
		return defaultPolicy
	}
	return *appSetSyncPolicy.ApplicationsSync
}

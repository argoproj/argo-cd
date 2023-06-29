package utils

import (
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// Policies is a registry of available policies.
var Policies = map[string]argov1alpha1.ApplicationsSyncPolicy{
	"create-only":   argov1alpha1.ApplicationsSyncPolicyCreateOnly,
	"create-update": argov1alpha1.ApplicationsSyncPolicyCreateUpdate,
	"create-delete": argov1alpha1.ApplicationsSyncPolicyCreateDelete,
	"sync":          argov1alpha1.ApplicationsSyncPolicySync,
	// Default is "sync"
	"": argov1alpha1.ApplicationsSyncPolicySync,
}

func DefaultPolicy(appSetSyncPolicy *argov1alpha1.ApplicationSetSyncPolicy, controllerPolicy argov1alpha1.ApplicationsSyncPolicy, enablePolicyOverride bool) argov1alpha1.ApplicationsSyncPolicy {
	if appSetSyncPolicy == nil || appSetSyncPolicy.ApplicationsSync == nil || !enablePolicyOverride {
		return controllerPolicy
	}
	return *appSetSyncPolicy.ApplicationsSync
}

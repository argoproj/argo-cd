package v1beta1

import (
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// ConvertFromV1alpha1 converts a v1alpha1.Application to a v1beta1.Application.
// This is used by the conversion webhook when serving v1beta1 API requests.
func ConvertFromV1alpha1(src *v1alpha1.Application) *Application {
	dst := &Application{
		TypeMeta:   src.TypeMeta,
		ObjectMeta: src.ObjectMeta,
		Status:     src.Status,
		Operation:  src.Operation,
	}

	// Update API version and Kind
	dst.APIVersion = SchemeGroupVersion.String()
	dst.Kind = "Application"

	// Convert spec
	dst.Spec = convertSpecFromV1alpha1(&src.Spec)

	return dst
}

func convertSpecFromV1alpha1(src *v1alpha1.ApplicationSpec) ApplicationSpec {
	dst := ApplicationSpec{
		// Don't copy Source - v1beta1 only uses Sources
		// Source field is intentionally not set in v1beta1
		Destination:          src.Destination,
		Project:              src.Project,
		IgnoreDifferences:    IgnoreDifferences(src.IgnoreDifferences),
		Info:                 src.Info,
		RevisionHistoryLimit: src.RevisionHistoryLimit,
		SourceHydrator:       src.SourceHydrator,
	}

	// Merge source into sources
	// If Sources is already populated, use it directly
	// If only Source is set, convert it to Sources[0]
	if len(src.Sources) > 0 {
		dst.Sources = ApplicationSources(src.Sources)
	} else if src.Source != nil {
		dst.Sources = ApplicationSources{*src.Source}
	}

	// Convert SyncPolicy
	if src.SyncPolicy != nil {
		dst.SyncPolicy = convertSyncPolicyFromV1alpha1(src.SyncPolicy)
	}

	return dst
}

func convertSyncPolicyFromV1alpha1(src *v1alpha1.SyncPolicy) *SyncPolicy {
	if src == nil {
		return nil
	}

	dst := &SyncPolicy{
		Automated:                src.Automated,
		Retry:                    src.Retry,
		ManagedNamespaceMetadata: src.ManagedNamespaceMetadata,
	}

	// Convert []string SyncOptions to structured SyncOptions
	if len(src.SyncOptions) > 0 {
		dst.SyncOptions = convertSyncOptionsFromStrings(src.SyncOptions)
	}

	return dst
}

// convertSyncOptionsFromStrings converts v1alpha1 []string sync options to structured v1beta1 SyncOptions
func convertSyncOptionsFromStrings(opts v1alpha1.SyncOptions) *SyncOptions {
	dst := &SyncOptions{}

	for _, opt := range opts {
		switch opt {
		// Validate
		case "Validate=true":
			dst.Validate = boolPtr(true)
		case "Validate=false":
			dst.Validate = boolPtr(false)

		// CreateNamespace
		case "CreateNamespace=true":
			dst.CreateNamespace = boolPtr(true)

		// PruneLast
		case "PruneLast=true":
			dst.PruneLast = boolPtr(true)

		// Replace
		case "Replace=true":
			dst.Replace = boolPtr(true)
		case "Replace=false":
			dst.Replace = boolPtr(false)

		// Force
		case "Force=true":
			dst.Force = boolPtr(true)

		// ServerSideApply
		case "ServerSideApply=true":
			dst.ServerSideApply = boolPtr(true)
		case "ServerSideApply=false":
			dst.ServerSideApply = boolPtr(false)

		// ApplyOutOfSyncOnly
		case "ApplyOutOfSyncOnly=true":
			dst.ApplyOutOfSyncOnly = boolPtr(true)
		case "ApplyOutOfSyncOnly=false":
			dst.ApplyOutOfSyncOnly = boolPtr(false)

		// SkipDryRunOnMissingResource
		case "SkipDryRunOnMissingResource=true":
			dst.SkipDryRunOnMissingResource = boolPtr(true)

		// RespectIgnoreDifferences
		case "RespectIgnoreDifferences=true":
			dst.RespectIgnoreDifferences = boolPtr(true)

		// FailOnSharedResource
		case "FailOnSharedResource=true":
			dst.FailOnSharedResource = boolPtr(true)

		// ClientSideApplyMigration
		case "ClientSideApplyMigration=true":
			dst.ClientSideApplyMigration = boolPtr(true)
		case "ClientSideApplyMigration=false":
			dst.ClientSideApplyMigration = boolPtr(false)

		// Prune options
		case "Prune=false":
			dst.Prune = pruneDeletePtr(SyncOptionDisabled)
		case "Prune=confirm":
			dst.Prune = pruneDeletePtr(SyncOptionConfirm)

		// Delete options
		case "Delete=false":
			dst.Delete = pruneDeletePtr(SyncOptionDisabled)
		case "Delete=confirm":
			dst.Delete = pruneDeletePtr(SyncOptionConfirm)

		// PrunePropagationPolicy
		case "PrunePropagationPolicy=background":
			dst.PrunePropagationPolicy = propagationPtr(PrunePropagationPolicyBackground)
		case "PrunePropagationPolicy=foreground":
			dst.PrunePropagationPolicy = propagationPtr(PrunePropagationPolicyForeground)
		case "PrunePropagationPolicy=orphan":
			dst.PrunePropagationPolicy = propagationPtr(PrunePropagationPolicyOrphan)

		default:
			// Handle any unrecognized options by checking if they match known patterns
			if strings.HasPrefix(opt, "PrunePropagationPolicy=") {
				val := strings.TrimPrefix(opt, "PrunePropagationPolicy=")
				policy := PrunePropagationPolicy(val)
				dst.PrunePropagationPolicy = &policy
			}
		}
	}

	return dst
}

// ConvertToV1alpha1 converts a v1beta1.Application to a v1alpha1.Application.
// This is used by the conversion webhook when storing objects (v1alpha1 is the storage version).
func ConvertToV1alpha1(src *Application) *v1alpha1.Application {
	dst := &v1alpha1.Application{
		TypeMeta:   src.TypeMeta,
		ObjectMeta: src.ObjectMeta,
		Status:     src.Status,
		Operation:  src.Operation,
	}

	// Update API version
	dst.APIVersion = v1alpha1.SchemeGroupVersion.String()

	// Convert spec
	dst.Spec = convertSpecToV1alpha1(&src.Spec)

	return dst
}

func convertSpecToV1alpha1(src *ApplicationSpec) v1alpha1.ApplicationSpec {
	dst := v1alpha1.ApplicationSpec{
		Destination:          src.Destination,
		Project:              src.Project,
		IgnoreDifferences:    v1alpha1.IgnoreDifferences(src.IgnoreDifferences),
		Info:                 src.Info,
		RevisionHistoryLimit: src.RevisionHistoryLimit,
		SourceHydrator:       src.SourceHydrator,
	}

	// Preserve original v1alpha1 source structure for backward compatibility:
	// - If exactly one source: set only Source (not Sources) to keep HasMultipleSources() false
	// - If multiple sources: set Sources
	if len(src.Sources) == 1 {
		dst.Source = &src.Sources[0]
	} else if len(src.Sources) > 1 {
		dst.Sources = v1alpha1.ApplicationSources(src.Sources)
	}

	// Convert SyncPolicy
	if src.SyncPolicy != nil {
		dst.SyncPolicy = convertSyncPolicyToV1alpha1(src.SyncPolicy)
	}

	return dst
}

func convertSyncPolicyToV1alpha1(src *SyncPolicy) *v1alpha1.SyncPolicy {
	if src == nil {
		return nil
	}

	dst := &v1alpha1.SyncPolicy{
		Automated:                src.Automated,
		Retry:                    src.Retry,
		ManagedNamespaceMetadata: src.ManagedNamespaceMetadata,
	}

	// Convert structured SyncOptions back to []string
	if src.SyncOptions != nil {
		dst.SyncOptions = convertSyncOptionsToStrings(src.SyncOptions)
	}

	return dst
}

// convertSyncOptionsToStrings converts structured v1beta1 SyncOptions to v1alpha1 []string format
func convertSyncOptionsToStrings(opts *SyncOptions) v1alpha1.SyncOptions {
	if opts == nil {
		return nil
	}

	var result v1alpha1.SyncOptions

	// Validate
	if opts.Validate != nil {
		if *opts.Validate {
			// Validate=true is usually not explicitly set (it's the default)
			// but we include it for completeness if explicitly set
		} else {
			result = append(result, "Validate=false")
		}
	}

	// CreateNamespace
	if opts.CreateNamespace != nil && *opts.CreateNamespace {
		result = append(result, "CreateNamespace=true")
	}

	// PruneLast
	if opts.PruneLast != nil && *opts.PruneLast {
		result = append(result, "PruneLast=true")
	}

	// Replace
	if opts.Replace != nil {
		if *opts.Replace {
			result = append(result, "Replace=true")
		} else {
			result = append(result, "Replace=false")
		}
	}

	// Force
	if opts.Force != nil && *opts.Force {
		result = append(result, "Force=true")
	}

	// ServerSideApply
	if opts.ServerSideApply != nil {
		if *opts.ServerSideApply {
			result = append(result, "ServerSideApply=true")
		} else {
			result = append(result, "ServerSideApply=false")
		}
	}

	// ApplyOutOfSyncOnly
	if opts.ApplyOutOfSyncOnly != nil {
		if *opts.ApplyOutOfSyncOnly {
			result = append(result, "ApplyOutOfSyncOnly=true")
		} else {
			result = append(result, "ApplyOutOfSyncOnly=false")
		}
	}

	// SkipDryRunOnMissingResource
	if opts.SkipDryRunOnMissingResource != nil && *opts.SkipDryRunOnMissingResource {
		result = append(result, "SkipDryRunOnMissingResource=true")
	}

	// RespectIgnoreDifferences
	if opts.RespectIgnoreDifferences != nil && *opts.RespectIgnoreDifferences {
		result = append(result, "RespectIgnoreDifferences=true")
	}

	// FailOnSharedResource
	if opts.FailOnSharedResource != nil && *opts.FailOnSharedResource {
		result = append(result, "FailOnSharedResource=true")
	}

	// ClientSideApplyMigration
	if opts.ClientSideApplyMigration != nil {
		if *opts.ClientSideApplyMigration {
			result = append(result, "ClientSideApplyMigration=true")
		} else {
			result = append(result, "ClientSideApplyMigration=false")
		}
	}

	// Prune
	if opts.Prune != nil {
		switch *opts.Prune {
		case SyncOptionDisabled:
			result = append(result, "Prune=false")
		case SyncOptionConfirm:
			result = append(result, "Prune=confirm")
			// SyncOptionEnabled is the default, no need to add
		}
	}

	// Delete
	if opts.Delete != nil {
		switch *opts.Delete {
		case SyncOptionDisabled:
			result = append(result, "Delete=false")
		case SyncOptionConfirm:
			result = append(result, "Delete=confirm")
			// SyncOptionEnabled is the default, no need to add
		}
	}

	// PrunePropagationPolicy
	if opts.PrunePropagationPolicy != nil {
		result = append(result, "PrunePropagationPolicy="+string(*opts.PrunePropagationPolicy))
	}

	return result
}

// Helper functions
func boolPtr(b bool) *bool                                            { return &b }
func pruneDeletePtr(p SyncOptionPruneDelete) *SyncOptionPruneDelete   { return &p }
func propagationPtr(p PrunePropagationPolicy) *PrunePropagationPolicy { return &p }

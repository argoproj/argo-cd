package v1beta1

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// ConvertFromV1alpha1 converts a v1alpha1.Application to a v1beta1.Application.
// This is used by the conversion webhook when serving v1beta1 API requests.
func ConvertFromV1alpha1(src *v1alpha1.Application) *Application {
	// Deep-copy up front so the returned v1beta1 object never shares
	// slices, maps, or pointers with the caller-owned src.
	src = src.DeepCopy()
	dst := &Application{
		TypeMeta:   src.TypeMeta,
		ObjectMeta: src.ObjectMeta,
		// v1alpha1 keeps `operation` at the top level; v1beta1 relocates it under
		// status. The embedded status is otherwise the same type, no conversion needed.
		Status: ApplicationStatus{
			ApplicationStatus: src.Status,
			Operation:         src.Operation,
		},
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

	// Merge source into sources for non-hydrator apps only.
	// For hydrator apps, Sources must not be set as the CEL validation rule
	// "cannot have both sources and sourceHydrator defined" would fail.
	// If Sources is already populated, use it directly
	// If only Source is set, convert it to Sources[0]
	if src.SourceHydrator == nil {
		if len(src.Sources) > 0 {
			dst.Sources = ApplicationSources(src.Sources)
		} else if src.Source != nil {
			dst.Sources = ApplicationSources{*src.Source}
		}
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
			dst.Validate = new(true)
		case "Validate=false":
			dst.Validate = new(false)

		// CreateNamespace
		case "CreateNamespace=true":
			dst.CreateNamespace = new(true)

		// PruneLast
		case "PruneLast=true":
			dst.PruneLast = new(true)

		// Replace
		case "Replace=true":
			dst.Replace = new(true)
		case "Replace=false":
			dst.Replace = new(false)

		// Force
		case "Force=true":
			dst.Force = new(true)

		// ServerSideApply
		case "ServerSideApply=true":
			dst.ServerSideApply = new(true)
		case "ServerSideApply=false":
			dst.ServerSideApply = new(false)

		// ApplyOutOfSyncOnly
		case "ApplyOutOfSyncOnly=true":
			dst.ApplyOutOfSyncOnly = new(true)
		case "ApplyOutOfSyncOnly=false":
			dst.ApplyOutOfSyncOnly = new(false)

		// SkipDryRunOnMissingResource
		case "SkipDryRunOnMissingResource=true":
			dst.SkipDryRunOnMissingResource = new(true)

		// RespectIgnoreDifferences
		case "RespectIgnoreDifferences=true":
			dst.RespectIgnoreDifferences = new(true)

		// FailOnSharedResource
		case "FailOnSharedResource=true":
			dst.FailOnSharedResource = new(true)

		// ClientSideApplyMigration
		case "ClientSideApplyMigration=true":
			dst.ClientSideApplyMigration = new(true)
		case "ClientSideApplyMigration=false":
			dst.ClientSideApplyMigration = new(false)

		// Prune options
		case "Prune=false":
			dst.Prune = new(SyncOptionDisabled)
		case "Prune=confirm":
			dst.Prune = new(SyncOptionConfirm)

		// Delete options
		case "Delete=false":
			dst.Delete = new(SyncOptionDisabled)
		case "Delete=confirm":
			dst.Delete = new(SyncOptionConfirm)

		// PrunePropagationPolicy
		case "PrunePropagationPolicy=background":
			dst.PrunePropagationPolicy = new(PrunePropagationPolicyBackground)
		case "PrunePropagationPolicy=foreground":
			dst.PrunePropagationPolicy = new(PrunePropagationPolicyForeground)
		case "PrunePropagationPolicy=orphan":
			dst.PrunePropagationPolicy = new(PrunePropagationPolicyOrphan)

		default:
			// PrunePropagationPolicy accepts an open-ended value, so match it by prefix.
			if after, ok := strings.CutPrefix(opt, "PrunePropagationPolicy="); ok {
				val := after
				dst.PrunePropagationPolicy = new(PrunePropagationPolicy(val))
			}
			// Any other unrecognized option is dropped: v1beta1 SyncOptions is a
			// structured type and cannot hold arbitrary strings. The cases above must
			// cover every option in gitops-engine's sync/common and Argo CD's common
			// package — TestConvertSyncOptions_AllKnownOptionsRoundTrip guards against a
			// new option being added without a matching case here.
		}
	}

	return dst
}

// ConvertToV1alpha1 converts a v1beta1.Application to a v1alpha1.Application.
// This is used by the conversion webhook when storing objects (v1alpha1 is the storage version).
func ConvertToV1alpha1(src *Application) *v1alpha1.Application {
	// Deep-copy up front so the returned v1alpha1 object never shares
	// slices, maps, or pointers (e.g. dst.Source backing dst.Sources[0])
	// with the caller-owned src.
	src = src.DeepCopy()
	dst := &v1alpha1.Application{
		TypeMeta:   src.TypeMeta,
		ObjectMeta: src.ObjectMeta,
		// Move `operation` back to the top level for the v1alpha1 storage form.
		Operation: src.Status.Operation,
		Status:    src.Status.ApplicationStatus,
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
			result = append(result, "Validate=true")
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
		result = append(result, fmt.Sprintf("Replace=%v", *opts.Replace))
	}

	// Force
	if opts.Force != nil && *opts.Force {
		result = append(result, "Force=true")
	}

	// ServerSideApply
	if opts.ServerSideApply != nil {
		result = append(result, fmt.Sprintf("ServerSideApply=%v", *opts.ServerSideApply))
	}

	// ApplyOutOfSyncOnly
	if opts.ApplyOutOfSyncOnly != nil {
		result = append(result, fmt.Sprintf("ApplyOutOfSyncOnly=%v", *opts.ApplyOutOfSyncOnly))
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
		result = append(result, fmt.Sprintf("ClientSideApplyMigration=%v", *opts.ClientSideApplyMigration))
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

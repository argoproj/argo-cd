package v1beta1

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// SourceFormatAnnotation records, on a converted v1beta1 Application, which
// form the original v1alpha1 object used to express its source. v1alpha1 can
// express a single source three ways — `source`, `sources` with one element,
// or both populated — and HasMultipleSources() keys off len(sources), so the
// reverse conversion must restore the exact original form: anything else
// changes controller behavior and shows up as permanent spec drift vs git.
// The annotation is stripped again when converting back to v1alpha1.
const SourceFormatAnnotation = "argocd.argoproj.io/v1alpha1-source-format"

const (
	// SourceFormatSingular means the v1alpha1 object set only `source`.
	SourceFormatSingular = "singular"
	// SourceFormatBoth means the v1alpha1 object set both `source` and `sources`.
	SourceFormatBoth = "both"
)

// SyncOptionsAnnotation records, on a converted v1beta1 Application, the
// original v1alpha1 syncOptions string list verbatim (JSON-encoded). v1alpha1
// stores syncOptions as an ordered []string while v1beta1 models them as a
// structured object, so the structured form remembers neither the original
// ordering nor strings it has no field for. Without the marker, the reverse
// conversion would emit a fixed canonical order and drop unknown strings —
// and because the apiserver round-trips the whole object through conversion
// on every v1beta1 write (v1alpha1 is the storage version), a status-only
// write would then rewrite the stored spec, bump metadata.generation, and
// show up as permanent drift vs git. The reverse conversion replays the
// recorded list only while it still parses to the object's structured
// syncOptions (i.e. no v1beta1 client edited them) and always strips the
// annotation again.
const SyncOptionsAnnotation = "argocd.argoproj.io/v1alpha1-sync-options"

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

	// Record the original source form so ConvertToV1alpha1 can restore it
	// exactly (see SourceFormatAnnotation).
	switch {
	case src.Spec.Source != nil && len(src.Spec.Sources) > 0:
		setConversionAnnotation(dst, SourceFormatAnnotation, SourceFormatBoth)
	case src.Spec.Source != nil:
		setConversionAnnotation(dst, SourceFormatAnnotation, SourceFormatSingular)
	default:
		delete(dst.Annotations, SourceFormatAnnotation)
	}

	// Record the original syncOptions strings so ConvertToV1alpha1 can restore
	// them verbatim (see SyncOptionsAnnotation).
	delete(dst.Annotations, SyncOptionsAnnotation)
	if src.Spec.SyncPolicy != nil && len(src.Spec.SyncPolicy.SyncOptions) > 0 {
		// json.Marshal cannot fail on a []string.
		encoded, _ := json.Marshal([]string(src.Spec.SyncPolicy.SyncOptions))
		setConversionAnnotation(dst, SyncOptionsAnnotation, string(encoded))
	}

	return dst
}

func setConversionAnnotation(app *Application, key, value string) {
	if app.Annotations == nil {
		app.Annotations = map[string]string{}
	}
	app.Annotations[key] = value
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

	// Merge source into sources: v1beta1 only has the plural field. This is done
	// regardless of sourceHydrator — v1alpha1 legally stores both, and conversion
	// must be lossless. The original form (singular/plural/both) is recorded by
	// ConvertFromV1alpha1 in SourceFormatAnnotation so the reverse conversion can
	// restore it exactly.
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
			dst.Validate = new(true)
		case "Validate=false":
			dst.Validate = new(false)

		// CreateNamespace
		case "CreateNamespace=true":
			dst.CreateNamespace = new(true)
		case "CreateNamespace=false":
			dst.CreateNamespace = new(false)

		// PruneLast
		case "PruneLast=true":
			dst.PruneLast = new(true)
		case "PruneLast=false":
			dst.PruneLast = new(false)

		// Replace
		case "Replace=true":
			dst.Replace = new(true)
		case "Replace=false":
			dst.Replace = new(false)

		// Force
		case "Force=true":
			dst.Force = new(true)
		case "Force=false":
			dst.Force = new(false)

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
		case "SkipDryRunOnMissingResource=false":
			dst.SkipDryRunOnMissingResource = new(false)

		// RespectIgnoreDifferences
		case "RespectIgnoreDifferences=true":
			dst.RespectIgnoreDifferences = new(true)
		case "RespectIgnoreDifferences=false":
			dst.RespectIgnoreDifferences = new(false)

		// FailOnSharedResource
		case "FailOnSharedResource=true":
			dst.FailOnSharedResource = new(true)
		case "FailOnSharedResource=false":
			dst.FailOnSharedResource = new(false)

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
				dst.PrunePropagationPolicy = new(PrunePropagationPolicy(after))
			}
			// Any other unrecognized option string has no structured field here.
			// It is not lost on a storage round-trip, though: ConvertFromV1alpha1
			// records the original string list in SyncOptionsAnnotation and the
			// reverse conversion replays it verbatim, so unknown strings survive
			// conversion even without a structured representation.
			// TestConvertSyncOptions_AllKnownOptionsRoundTrip guards against an
			// option being added without a matching case here.
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

	// The source-format and sync-options markers are conversion metadata, not
	// user data: consume them and strip them from the v1alpha1 object. dst
	// shares the deep-copied ObjectMeta, so deleting from dst.Annotations is safe.
	sourceFormat := src.Annotations[SourceFormatAnnotation]
	recordedSyncOptions := src.Annotations[SyncOptionsAnnotation]
	delete(dst.Annotations, SourceFormatAnnotation)
	delete(dst.Annotations, SyncOptionsAnnotation)
	if len(dst.Annotations) == 0 {
		// Keep nil-vs-empty stable across a round-trip.
		dst.Annotations = nil
	}

	// Convert spec
	dst.Spec = convertSpecToV1alpha1(&src.Spec, sourceFormat)
	restoreSyncOptions(dst, src, recordedSyncOptions)

	return dst
}

// restoreSyncOptions replays the original v1alpha1 syncOptions strings
// (recorded in SyncOptionsAnnotation) over the canonical emission when they
// still represent the object's structured syncOptions. This makes the
// round-trip the identity for whatever v1alpha1 already stores: original
// ordering and strings with no structured field survive instead of being
// canonicalized away on a status-only write. When the structured options no
// longer parse back to the recorded list, a v1beta1 client made a real edit,
// and the canonical strings already emitted stand.
func restoreSyncOptions(dst *v1alpha1.Application, src *Application, recorded string) {
	if recorded == "" || dst.Spec.SyncPolicy == nil {
		return
	}
	if src.Spec.SyncPolicy == nil || src.Spec.SyncPolicy.SyncOptions == nil {
		return
	}
	var original []string
	if err := json.Unmarshal([]byte(recorded), &original); err != nil || len(original) == 0 {
		return
	}
	if !reflect.DeepEqual(convertSyncOptionsFromStrings(original), src.Spec.SyncPolicy.SyncOptions) {
		return
	}
	dst.Spec.SyncPolicy.SyncOptions = original
}

func convertSpecToV1alpha1(src *ApplicationSpec, sourceFormat string) v1alpha1.ApplicationSpec {
	dst := v1alpha1.ApplicationSpec{
		Destination:          src.Destination,
		Project:              src.Project,
		IgnoreDifferences:    v1alpha1.IgnoreDifferences(src.IgnoreDifferences),
		Info:                 src.Info,
		RevisionHistoryLimit: src.RevisionHistoryLimit,
		SourceHydrator:       src.SourceHydrator,
	}

	// Restore the source in the exact form the original v1alpha1 object used
	// (recorded in SourceFormatAnnotation). With no marker — e.g. an app created
	// natively via v1beta1 — sources stay plural, which round-trips stably.
	// Collapsing a single-element sources list into the singular field here
	// would flip HasMultipleSources() and rewrite stored specs on every
	// conversion round-trip.
	switch {
	case sourceFormat == SourceFormatSingular && len(src.Sources) == 1:
		dst.Source = &src.Sources[0]
	case sourceFormat == SourceFormatBoth && len(src.Sources) > 0:
		dst.Source = &src.Sources[0]
		dst.Sources = v1alpha1.ApplicationSources(src.Sources)
	case len(src.Sources) > 0:
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
	if opts.CreateNamespace != nil {
		result = append(result, fmt.Sprintf("CreateNamespace=%v", *opts.CreateNamespace))
	}

	// PruneLast
	if opts.PruneLast != nil {
		result = append(result, fmt.Sprintf("PruneLast=%v", *opts.PruneLast))
	}

	// Replace
	if opts.Replace != nil {
		result = append(result, fmt.Sprintf("Replace=%v", *opts.Replace))
	}

	// Force
	if opts.Force != nil {
		result = append(result, fmt.Sprintf("Force=%v", *opts.Force))
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
	if opts.SkipDryRunOnMissingResource != nil {
		result = append(result, fmt.Sprintf("SkipDryRunOnMissingResource=%v", *opts.SkipDryRunOnMissingResource))
	}

	// RespectIgnoreDifferences
	if opts.RespectIgnoreDifferences != nil {
		result = append(result, fmt.Sprintf("RespectIgnoreDifferences=%v", *opts.RespectIgnoreDifferences))
	}

	// FailOnSharedResource
	if opts.FailOnSharedResource != nil {
		result = append(result, fmt.Sprintf("FailOnSharedResource=%v", *opts.FailOnSharedResource))
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

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"golang.org/x/sync/errgroup"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/ignore"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	resourceutil "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/resource"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	argocommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/repository"
	"github.com/argoproj/argo-cd/v3/util/argo"
	argodiff "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/io"
	logutils "github.com/argoproj/argo-cd/v3/util/log"
	"github.com/argoproj/argo-cd/v3/util/manifeststream"
)

// targetManifestProvider is a function that retrieves target manifests for diff
type manifestProvider func(ctx context.Context) ([]*unstructured.Unstructured, error)

// comparisonObject represents a resource pair for comparison
type comparisonObject struct {
	key    kube.ResourceKey
	live   *unstructured.Unstructured
	target *unstructured.Unstructured
}

// diffStrategy is a function that performs diff on a batch of resources
// Returns DiffResult from the gitops-engine where NormalizedLive is the live state and PredictedLive is the target state
type diffStrategy func(ctx context.Context, items []comparisonObject) ([]*diff.DiffResult, error)

type resourceInfoProvider struct {
	namespacedByGk map[schema.GroupKind]bool
}

// Infer if obj is namespaced or not from corresponding live objects list. If corresponding live object has namespace then target object is also namespaced.
// If live object is missing then it does not matter if target is namespaced or not.
func (p *resourceInfoProvider) IsNamespaced(gk schema.GroupKind) (bool, error) {
	return p.namespacedByGk[gk], nil
}

// getInfoProviderFromState builds a resourceInfoProvider from live state items
// It infers whether resources are namespaced by checking if they have a namespace in live state
func getInfoProviderFromState(state *application.ManagedResourcesResponse) kube.ResourceInfoProvider {
	if state == nil {
		return &resourceInfoProvider{}
	}

	namespacedByGk := make(map[schema.GroupKind]bool)
	for _, item := range state.GetItems() {
		if item != nil {
			namespacedByGk[schema.GroupKind{Group: item.Group, Kind: item.Kind}] = item.Namespace != ""
		}
	}
	return &resourceInfoProvider{namespacedByGk: namespacedByGk}
}

// manifestsToUnstructured converts manifest strings to unstructured objects
func manifestsToUnstructured(manifests []string) ([]*unstructured.Unstructured, error) {
	result := make([]*unstructured.Unstructured, 0, len(manifests))
	for _, manifest := range manifests {
		obj, err := argoappv1.UnmarshalToUnstructured(manifest)
		if err != nil {
			return nil, err
		}
		result = append(result, obj)
	}
	return result, nil
}

// getObjectMap builds a map of objects by resource key, filtering hooks, ignored resources, and secrets
func getObjectMap(objects []*unstructured.Unstructured) map[kube.ResourceKey]*unstructured.Unstructured {
	objectMap := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for i := range objects {
		obj := objects[i]
		if obj == nil {
			continue
		}

		// Skip hooks and ignored resources
		if hook.IsHook(obj) || ignore.Ignore(obj) {
			continue
		}

		key := kube.GetResourceKey(obj)
		objectMap[key] = obj
	}
	return objectMap
}

// getComparisonObjects pairs target and live manifests by resource key
func getComparisonObjects(
	targetManifests []*unstructured.Unstructured,
	liveManifests []*unstructured.Unstructured,
) []comparisonObject {
	// Build map of target objects by key
	targetByKey := getObjectMap(targetManifests)
	liveByKey := getObjectMap(liveManifests)

	// Build result list by pairing live and target objects
	items := make([]comparisonObject, 0)

	// Process live objects and match with targets
	for key := range liveByKey {
		items = append(items, comparisonObject{
			key:    key,
			live:   liveByKey[key],
			target: targetByKey[key],
		})
		delete(targetByKey, key)
	}
	// Add remaining target objects that don't have live counterparts
	for key := range targetByKey {
		items = append(items, comparisonObject{
			key:    key,
			live:   nil,
			target: targetByKey[key],
		})
	}

	return items
}

// Deprecated: Prefer server-side generation since local side generation does not support plugins
func getLocalObjects(
	ctx context.Context,
	app *argoappv1.Application,
	proj *argoappv1.AppProject,
	local string,
	localRepoRoot string,
	argoSettings *settings.Settings,
	clusterInfo *argoappv1.ClusterInfo,
) []*unstructured.Unstructured {
	manifestStrings := getLocalObjectsString(ctx, app, proj, local, localRepoRoot, argoSettings, clusterInfo)
	objs := make([]*unstructured.Unstructured, 0, len(manifestStrings))
	for i := range manifestStrings {
		obj := &unstructured.Unstructured{}
		err := json.Unmarshal([]byte(manifestStrings[i]), obj)
		errors.CheckError(err)

		if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
			// Secrets are not supported in local diff, so we skip them.
			// diff.HideSecretData is not used here because it requires server-side configurations to be reliable.
			fmt.Fprintf(os.Stderr, "Warning: Secret %s/%s is not supported in local diff and will be ignored\n", obj.GetNamespace(), obj.GetName())
			continue
		}

		objs = append(objs, obj)
	}
	return objs
}

// Deprecated: Prefer server-side generation since local side generation does not support plugins
func getLocalObjectsString(
	ctx context.Context,
	app *argoappv1.Application,
	proj *argoappv1.AppProject,
	local string,
	localRepoRoot string,
	argoSettings *settings.Settings,
	clusterInfo *argoappv1.ClusterInfo,
) []string {
	source := app.Spec.GetSource()
	res, err := repository.GenerateManifests(ctx, local, localRepoRoot, source.TargetRevision, &repoapiclient.ManifestRequest{
		Repo:                            &argoappv1.Repository{Repo: source.RepoURL},
		AppLabelKey:                     argoSettings.AppLabelKey,
		AppName:                         app.InstanceName(argoSettings.ControllerNamespace),
		Namespace:                       app.Spec.Destination.Namespace,
		ApplicationSource:               &source,
		KustomizeOptions:                argoSettings.KustomizeOptions,
		KubeVersion:                     clusterInfo.ServerVersion,
		ApiVersions:                     clusterInfo.APIVersions,
		TrackingMethod:                  argoSettings.TrackingMethod,
		ProjectName:                     proj.Name,
		ProjectSourceRepos:              proj.Spec.SourceRepos,
		AnnotationManifestGeneratePaths: app.GetAnnotation(argoappv1.AnnotationKeyManifestGeneratePaths),
	}, true, &git.NoopCredsStore{}, resource.MustParse("0"), nil)
	errors.CheckError(err)

	return res.Manifests
}

// Diff Strategy Factory Functions

// newServerSideDiffStrategy creates a server-side diff strategy with all dependencies
func newServerSideDiffStrategy(
	app *argoappv1.Application,
	appIf application.ApplicationServiceClient,
	appName string,
	appNs string,
	concurrency int,
	maxBatchKB int,
) diffStrategy {
	return func(ctx context.Context, items []comparisonObject) ([]*diff.DiffResult, error) {
		if len(items) == 0 {
			return []*diff.DiffResult{}, nil
		}

		// For server-side diff, we need to create aligned arrays
		liveResources := make([]*argoappv1.ResourceDiff, 0, len(items))
		targetManifests := make([]string, 0, len(items))

		for _, item := range items {
			liveResource := &argoappv1.ResourceDiff{
				Group:     item.key.Group,
				Kind:      item.key.Kind,
				Namespace: item.key.Namespace,
				Name:      item.key.Name,
			}
			if item.live != nil {
				jsonBytes, err := json.Marshal(item.live)
				if err != nil {
					return nil, fmt.Errorf("error marshaling live object: %w", err)
				}
				liveResource.LiveState = string(jsonBytes)
			}

			var targetManifest string
			if item.target != nil {
				jsonBytes, err := json.Marshal(item.target)
				if err != nil {
					return nil, fmt.Errorf("error marshaling target object: %w", err)
				}
				targetManifest = string(jsonBytes)
			}

			liveResources = append(liveResources, liveResource)
			targetManifests = append(targetManifests, targetManifest)
		}

		maxBatchSize := maxBatchKB * 1024
		var batches []struct{ start, end int }
		for i := 0; i < len(liveResources); {
			start := i
			size := 0
			for i < len(liveResources) {
				resourceSize := len(liveResources[i].LiveState) + len(targetManifests[i])
				if size+resourceSize > maxBatchSize && i > start {
					break
				}
				size += resourceSize
				i++
			}
			batches = append(batches, struct{ start, end int }{start, i})
		}

		g, errGroupCtx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)

		batchResults := make([][]*argoappv1.ResourceDiff, len(batches))

		for idx, batch := range batches {
			i := idx
			b := batch
			g.Go(func() error {
				serverSideDiffQuery := &application.ApplicationServerSideDiffQuery{
					AppName:         &appName,
					AppNamespace:    &appNs,
					Project:         &app.Spec.Project,
					LiveResources:   liveResources[b.start:b.end],
					TargetManifests: targetManifests[b.start:b.end],
				}
				serverSideDiffRes, err := appIf.ServerSideDiff(errGroupCtx, serverSideDiffQuery)
				if err != nil {
					return err
				}
				batchResults[i] = serverSideDiffRes.Items
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}

		results := make([]*diff.DiffResult, 0)
		for _, batchItems := range batchResults {
			for _, resultItem := range batchItems {
				results = append(results, &diff.DiffResult{
					Modified:       resultItem.Modified,
					NormalizedLive: []byte(resultItem.LiveState),
					PredictedLive:  []byte(resultItem.TargetState),
				})
			}
		}

		return results, nil
	}
}

// newClientSideDiffStrategy creates a client-side diff strategy with all dependencies
func newClientSideDiffStrategy(
	app *argoappv1.Application,
	argoSettings *settings.Settings,
	ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts,
) (diffStrategy, error) {
	// Build resource overrides map
	overrides := make(map[string]argoappv1.ResourceOverride)
	for k := range argoSettings.ResourceOverrides {
		val := argoSettings.ResourceOverrides[k]
		overrides[k] = *val
	}

	ignoreAggregatedRoles := false
	diffConfig, err := argodiff.NewDiffConfigBuilder().
		WithDiffSettings(app.Spec.IgnoreDifferences, overrides, ignoreAggregatedRoles, ignoreNormalizerOpts).
		WithTracking(argoSettings.AppLabelKey, argoSettings.TrackingMethod).
		WithNoCache().
		WithLogger(logutils.NewLogrusLogger(logutils.NewWithCurrentConfig())).
		Build()
	if err != nil {
		return nil, err
	}

	return func(_ context.Context, items []comparisonObject) ([]*diff.DiffResult, error) {
		results := make([]*diff.DiffResult, len(items))

		for i, item := range items {
			diffRes, err := argodiff.StateDiff(item.live, item.target, diffConfig)
			if err != nil {
				return nil, err
			}

			results[i] = &diffRes
		}

		return results, nil
	}, nil
}

// Manifest Provider Functions

// newMultiSourceRevisionProvider creates a provider for multi-source apps with specific revisions
func newMultiSourceRevisionProvider(
	appIf application.ApplicationServiceClient,
	appName string,
	appNs string,
	revisions []string,
	sourcePositions []int64,
	hardRefresh bool,
) manifestProvider {
	return func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		manifestQuery := application.ApplicationManifestQuery{
			Name:            &appName,
			AppNamespace:    &appNs,
			Revisions:       revisions,
			SourcePositions: sourcePositions,
			NoCache:         &hardRefresh,
		}
		targetManifests, err := appIf.GetManifests(ctx, &manifestQuery)
		if err != nil {
			return nil, err
		}
		return manifestsToUnstructured(targetManifests.Manifests)
	}
}

// newSingleRevisionProvider creates a provider for apps with a single revision
func newSingleRevisionProvider(
	appIf application.ApplicationServiceClient,
	appName string,
	appNs string,
	revision string,
	hardRefresh bool,
) manifestProvider {
	return func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		manifestQuery := application.ApplicationManifestQuery{
			Name:         &appName,
			Revision:     &revision,
			AppNamespace: &appNs,
			NoCache:      &hardRefresh,
		}
		targetManifests, err := appIf.GetManifests(ctx, &manifestQuery)
		if err != nil {
			return nil, err
		}
		return manifestsToUnstructured(targetManifests.Manifests)
	}
}

// newLocalServerSideProvider creates a provider for local manifests with server-side generation.
// This is the PREFERRED approach as it delegates manifest generation to the server,
// ensuring consistency with the server's manifest generation logic and reducing client-side complexity.
func newLocalServerSideProvider(
	appIf application.ApplicationServiceClient,
	appName string,
	appNs string,
	localPath string,
	localIncludes []string,
) manifestProvider {
	return func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		client, err := appIf.GetManifestsWithFiles(ctx, grpc_retry.Disable())
		if err != nil {
			return nil, err
		}

		err = manifeststream.SendApplicationManifestQueryWithFiles(ctx, client, appName, appNs, localPath, localIncludes)
		if err != nil {
			return nil, err
		}

		targetManifests, err := client.CloseAndRecv()
		if err != nil {
			return nil, err
		}

		return manifestsToUnstructured(targetManifests.Manifests)
	}
}

// newLocalClientSideProvider creates a provider for local manifests with client-side generation.
//
// Deprecated: Prefer newLocalServerSideProvider which performs manifest generation on the server,
// reducing client-side complexity and improving consistency with the server's manifest generation logic.
func newLocalClientSideProvider(
	clusterIf clusterpkg.ClusterServiceClient,
	argoSettings *settings.Settings,
	app *argoappv1.Application,
	proj *argoappv1.AppProject,
	localPath string,
	localRepoRoot string,
) manifestProvider {
	return func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		cluster, err := clusterIf.Get(ctx, &clusterpkg.ClusterQuery{
			Name:   app.Spec.Destination.Name,
			Server: app.Spec.Destination.Server,
		})
		if err != nil {
			return nil, err
		}

		return getLocalObjects(
			ctx,
			app,
			proj,
			localPath,
			localRepoRoot,
			argoSettings,
			&cluster.Info,
		), nil
	}
}

// newDefaultTargetProvider creates a provider that extracts targets from ManagedResources
func newDefaultTargetProvider(liveState *application.ManagedResourcesResponse) manifestProvider {
	return func(_ context.Context) ([]*unstructured.Unstructured, error) {
		targetManifests := make([]*unstructured.Unstructured, 0, len(liveState.Items))
		for i := range liveState.Items {
			res := liveState.Items[i]
			target := &unstructured.Unstructured{}
			err := json.Unmarshal([]byte(res.TargetState), &target)
			if err != nil {
				return nil, err
			}
			targetManifests = append(targetManifests, target)
		}
		return targetManifests, nil
	}
}

// newLiveManifestProvider creates a provider for live manifests from ManagedResources
func newLiveManifestProvider(liveState *application.ManagedResourcesResponse, excludeSecret bool) manifestProvider {
	return func(_ context.Context) ([]*unstructured.Unstructured, error) {
		liveManifests := make([]*unstructured.Unstructured, 0, len(liveState.Items))
		for i := range liveState.Items {
			res := liveState.Items[i]
			if excludeSecret && res.Kind == kube.SecretKind && res.Group == "" {
				continue
			}

			live := &unstructured.Unstructured{}
			err := json.Unmarshal([]byte(res.NormalizedLiveState), &live)
			if err != nil {
				return nil, err
			}
			liveManifests = append(liveManifests, live)
		}
		return liveManifests, nil
	}
}

// normalizeTargetManifestsProvider wraps a manifestProvider to normalize target objects
// This ensures namespace normalization and tracking annotation updates after deduplication
func newNormalizeTargetManifestsProvider(
	provider manifestProvider,
	app *argoappv1.Application,
	argoSettings *settings.Settings,
	infoProvider kube.ResourceInfoProvider,
) manifestProvider {
	return func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		manifests, err := provider(ctx)
		if err != nil {
			return nil, err
		}

		// Normalize target objects (namespace normalization, deduplication, and tracking re-application)
		resourceTracking := argo.NewResourceTracking()
		normalized, conditions, err := controller.NormalizeTargetObjects(
			app.Spec.Destination.Namespace,
			manifests,
			infoProvider,
			func(u *unstructured.Unstructured) error {
				return resourceTracking.SetAppInstance(
					u,
					argoSettings.AppLabelKey,
					app.InstanceName(argoSettings.ControllerNamespace),
					app.Spec.Destination.Namespace,
					argoappv1.TrackingMethod(argoSettings.TrackingMethod),
					argoSettings.GetInstallationID(),
				)
			},
		)
		if err != nil {
			return nil, err
		}

		// Log any conditions (warnings about duplicates)
		for _, condition := range conditions {
			log.Warnf("%s: %s", condition.Type, condition.Message)
		}

		return normalized, nil
	}
}

// compareManifests computes the diff using a target manifest provider
// Returns a list of comparisonObject containing all resources (added, removed, and modified)
func compareManifests(
	ctx context.Context,
	getTargetManifests manifestProvider,
	getLiveManifests manifestProvider,
	performDiff diffStrategy,
) ([]comparisonObject, error) {
	// Get live objects
	liveManifests, err := getLiveManifests(ctx)
	if err != nil {
		return nil, err
	}

	// Get target manifests
	targetManifests, err := getTargetManifests(ctx)
	if err != nil {
		return nil, err
	}

	// Build object map pairing live and target
	items := getComparisonObjects(targetManifests, liveManifests)

	// Perform diff on potentially modified resources
	diffResults, err := performDiff(ctx, items)
	if err != nil {
		return nil, err
	}

	results := make([]comparisonObject, 0)
	for _, diffRes := range diffResults {
		liveState := string(diffRes.NormalizedLive)
		targetState := string(diffRes.PredictedLive)

		hasLiveState := liveState != "null" && liveState != ""
		hasTargetState := targetState != "null" && targetState != ""
		if (!diffRes.Modified && hasLiveState && hasTargetState) || (!hasLiveState && !hasTargetState) {
			// If the item is not modified and it is neither an added or removed resource, skip it.
			// If we dont have any live or target state, skip it.
			continue
		}

		live, err := argoappv1.UnmarshalToUnstructured(liveState)
		if err != nil {
			return nil, err
		}

		target, err := argoappv1.UnmarshalToUnstructured(targetState)
		if err != nil {
			return nil, err
		}

		var key kube.ResourceKey
		if live != nil {
			key = kube.GetResourceKey(live)
		} else {
			key = kube.GetResourceKey(target)
		}

		results = append(results, comparisonObject{
			key:    key,
			live:   live,
			target: target,
		})
	}

	return results, nil
}

// NewApplicationDiffCommand returns a new instance of an `argocd app diff` command
func NewApplicationDiffCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		refresh                   bool
		hardRefresh               bool
		exitCode                  bool
		diffExitCode              int
		local                     string
		revision                  string
		localRepoRoot             string
		serverSideGenerate        bool
		serverSideDiff            bool
		serverSideDiffConcurrency int
		serverSideDiffMaxBatchKB  int
		localIncludes             []string
		appNamespace              string
		revisions                 []string
		sourcePositions           []int64
		sourceNames               []string
		ignoreNormalizerOpts      normalizers.IgnoreNormalizerOpts
	)
	shortDesc := "Perform a diff against the target and live state."
	command := &cobra.Command{
		Use:   "diff APPNAME",
		Short: shortDesc,
		Long:  shortDesc + "\nUses 'diff' to render the difference. KUBECTL_EXTERNAL_DIFF environment variable can be used to select your own diff tool.\nReturns the following exit codes: 2 on general errors, 1 when a diff is found, and 0 when no diff is found\nKubernetes Secrets are ignored from this diff.",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(2)
			}

			if len(sourceNames) > 0 && len(sourcePositions) > 0 {
				errors.Fatal(errors.ErrorGeneric, "Only one of source-positions and source-names can be specified.")
			}

			if len(sourcePositions) > 0 && len(revisions) != len(sourcePositions) {
				errors.Fatal(errors.ErrorGeneric, "While using --revisions and --source-positions, length of values for both flags should be same.")
			}

			if len(sourceNames) > 0 && len(revisions) != len(sourceNames) {
				errors.Fatal(errors.ErrorGeneric, "While using --revisions and --source-names, length of values for both flags should be same.")
			}

			if serverSideDiffConcurrency == 0 {
				errors.Fatal(errors.ErrorGeneric, "invalid value for --server-side-diff-concurrency: 0 is not allowed (use -1 for unlimited, or a positive number to limit concurrency)")
			}

			clientset := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := clientset.NewApplicationClientOrDie()
			defer io.Close(conn)
			appName, appNs := argo.ParseFromQualifiedName(args[0], appNamespace)
			app, err := appIf.Get(ctx, &application.ApplicationQuery{
				Name:         &appName,
				Refresh:      getRefreshType(refresh, hardRefresh),
				AppNamespace: &appNs,
			})
			errors.CheckError(err)

			if len(sourceNames) > 0 {
				sourceNameToPosition := getSourceNameToPositionMap(app)

				for _, name := range sourceNames {
					pos, ok := sourceNameToPosition[name]
					if !ok {
						log.Fatalf("Unknown source name '%s'", name)
					}
					sourcePositions = append(sourcePositions, pos)
				}
			}

			liveState, err := appIf.ManagedResources(ctx, &application.ResourcesQuery{ApplicationName: &appName, AppNamespace: &appNs})
			errors.CheckError(err)
			conn, settingsIf := clientset.NewSettingsClientOrDie()
			defer io.Close(conn)
			argoSettings, err := settingsIf.Get(ctx, &settings.SettingsQuery{})
			errors.CheckError(err)

			hasServerSideDiffAnnotation := resourceutil.HasAnnotationOption(app, argocommon.AnnotationCompareOptions, "ServerSideDiff=true")

			// Use annotation if flag not explicitly set
			if !c.Flags().Changed("server-side-diff") {
				serverSideDiff = hasServerSideDiffAnnotation
			} else if serverSideDiff && !hasServerSideDiffAnnotation {
				// Flag explicitly set to true, but app annotation is not set
				fmt.Fprint(os.Stderr, "Warning: Application does not have ServerSideDiff=true annotation.\n")
			}

			// Server side diff with local requires server side generate to be set as there will be a mismatch with client-generated manifests.
			if serverSideDiff && local != "" && !serverSideGenerate {
				log.Fatal("--server-side-diff with --local requires --server-side-generate.")
			}

			proj := getProject(ctx, c, clientOpts, app.Spec.Project)

			// Build resource info provider from live state to determine if resources are namespaced
			infoProvider := getInfoProviderFromState(liveState)

			// Create target manifest provider based on flags
			var getTargetManifests manifestProvider
			excludeSecret := false

			switch {
			case app.Spec.HasMultipleSources() && len(revisions) > 0 && len(sourcePositions) > 0:
				numOfSources := int64(len(app.Spec.GetSources()))
				for _, pos := range sourcePositions {
					if pos <= 0 || pos > numOfSources {
						log.Fatal("source-position cannot be less than 1 or more than number of sources in the app. Counting starts at 1.")
					}
				}
				getTargetManifests = newMultiSourceRevisionProvider(appIf, appName, appNs, revisions, sourcePositions, hardRefresh)

			case revision != "":
				getTargetManifests = newSingleRevisionProvider(appIf, appName, appNs, revision, hardRefresh)

			case local != "":
				if serverSideGenerate {
					getTargetManifests = newLocalServerSideProvider(appIf, appName, appNs, local, localIncludes)
				} else {
					fmt.Fprint(os.Stderr, "Warning: local diff without --server-side-generate is deprecated and does not work with plugins. Server-side generation will be the default in v2.7.")
					conn, clusterIf := clientset.NewClusterClientOrDie()
					defer io.Close(conn)
					getTargetManifests = newLocalClientSideProvider(clusterIf, argoSettings, app, proj.Project, local, localRepoRoot)
					// Local diff does not support to hide the configurable annotations in the secrets.
					// To not have constant partial diffs, we exclude secrets from the diff.
					excludeSecret = true
				}

			default:
				getTargetManifests = newDefaultTargetProvider(liveState)
			}

			// Wrap target manifest provider with normalization since the manifest are have not been applied to kubernetes
			getTargetManifests = newNormalizeTargetManifestsProvider(getTargetManifests, app, argoSettings, infoProvider)

			// Create live manifest provider
			getLiveManifests := newLiveManifestProvider(liveState, excludeSecret)

			// Create diff strategy based on --server-side-diff flag
			var diffHandler diffStrategy
			if serverSideDiff {
				diffHandler = newServerSideDiffStrategy(app, appIf, appName, appNs, serverSideDiffConcurrency, serverSideDiffMaxBatchKB)
			} else {
				clientSideDiff, err := newClientSideDiffStrategy(app, argoSettings, ignoreNormalizerOpts)
				errors.CheckError(err)
				diffHandler = clientSideDiff
			}

			// Compute diff
			results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, diffHandler)
			errors.CheckError(err)

			sort.Slice(results, func(i, j int) bool {
				return results[i].key.String() < results[j].key.String()
			})
			for _, result := range results {
				printResourceDiff(result.key.Group, result.key.Kind, result.key.Namespace, result.key.Name, result.live, result.target)
			}

			foundDiffs := len(results) > 0
			if foundDiffs && exitCode {
				os.Exit(diffExitCode)
			}
		},
	}
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh application data when retrieving")
	command.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "Refresh application data as well as target manifests cache")
	command.Flags().BoolVar(&exitCode, "exit-code", true, "Return non-zero exit code when there is a diff. May also return non-zero exit code if there is an error.")
	command.Flags().IntVar(&diffExitCode, "diff-exit-code", 1, "Return specified exit code when there is a diff. Typical error code is 20 but use another exit code if you want to differentiate from the generic exit code (20) returned by all CLI commands.")
	command.Flags().StringVar(&local, "local", "", "Compare live app to a local manifests")
	command.Flags().StringVar(&revision, "revision", "", "Compare live app to a particular revision")
	command.Flags().StringVar(&localRepoRoot, "local-repo-root", "/", "Path to the repository root. Used together with --local allows setting the repository root")
	command.Flags().BoolVar(&serverSideGenerate, "server-side-generate", false, "Used with --local, this will send your manifests to the server for diffing")
	command.Flags().BoolVar(&serverSideDiff, "server-side-diff", false, "Use server-side diff to calculate the diff. This will default to true if the ServerSideDiff annotation is set on the application.")
	addServerSideDiffPerfFlags(command, &serverSideDiffConcurrency, &serverSideDiffMaxBatchKB)
	command.Flags().StringArrayVar(&localIncludes, "local-include", []string{"*.yaml", "*.yml", "*.json"}, "Used with --server-side-generate, specify patterns of filenames to send. Matching is based on filename and not path.")
	command.Flags().StringVarP(&appNamespace, "app-namespace", "N", "", "Only render the difference in namespace")
	command.Flags().StringArrayVar(&revisions, "revisions", []string{}, "Show manifests at specific revisions for source position in source-positions")
	command.Flags().Int64SliceVar(&sourcePositions, "source-positions", []int64{}, "List of source positions. Default is empty array. Counting start at 1.")
	command.Flags().StringArrayVar(&sourceNames, "source-names", []string{}, "List of source names. Default is an empty array.")
	command.Flags().DurationVar(&ignoreNormalizerOpts.JQExecutionTimeout, "ignore-normalizer-jq-execution-timeout", normalizers.DefaultJQExecutionTimeout, "Set ignore normalizer JQ execution timeout")
	return command
}

// addServerSideDiffPerfFlags adds server-side diff performance tuning flags to a command
func addServerSideDiffPerfFlags(command *cobra.Command, serverSideDiffConcurrency *int, serverSideDiffMaxBatchKB *int) {
	command.Flags().IntVar(serverSideDiffConcurrency, "server-side-diff-concurrency", -1, "Max concurrent batches for server-side diff. -1 = unlimited, 1 = sequential, 2+ = concurrent (0 = invalid)")
	command.Flags().IntVar(serverSideDiffMaxBatchKB, "server-side-diff-max-batch-kb", 250, "Max batch size in KB for server-side diff. Smaller values are safer for proxies")
}

// printResourceDiff prints the diff header and calls cli.PrintDiff for a resource
func printResourceDiff(group, kind, namespace, name string, live, target *unstructured.Unstructured) {
	fmt.Printf("\n===== %s/%s %s/%s ======\n", group, kind, namespace, name)
	_ = cli.PrintDiff(name, live, target)
}

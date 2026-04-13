package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/ignore"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	resourceutil "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/resource"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	argocommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	argodiff "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/errors"
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

		// Skip secrets - argo-cd doesn't have access to k8s secret data
		if key.Kind == kube.SecretKind && key.Group == "" {
			continue
		}

		objectMap[key] = obj
	}
	return objectMap
}

// getComparisonObjects pairs target and live manifests by resource key
// This consolidates the logic from groupObjsByKey and groupObjsForDiff
func getComparisonObjects(
	targetManifests []*unstructured.Unstructured,
	liveManifests []*unstructured.Unstructured,
	app *argoappv1.Application,
) []comparisonObject {
	// Build map of namespace info from live objects
	namespacedByGk := make(map[schema.GroupKind]bool)
	for i := range liveManifests {
		if liveManifests[i] != nil {
			key := kube.GetResourceKey(liveManifests[i])
			namespacedByGk[schema.GroupKind{Group: key.Group, Kind: key.Kind}] = key.Namespace != ""
		}
	}

	// Deduplicate target objects
	targetManifests, _, err := controller.DeduplicateTargetObjects(
		app.Spec.Destination.Namespace,
		targetManifests,
		&resourceInfoProvider{namespacedByGk: namespacedByGk},
	)
	errors.CheckError(err)

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

// createClientSideDiffStrategy creates a strategy that performs client-side diff using argodiff.StateDiff
func createClientSideDiffStrategy(diffConfig argodiff.DiffConfig) diffStrategy {
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
	}
}

// createServerSideDiffStrategy creates a strategy that performs server-side diff using the API
func createServerSideDiffStrategy(
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
				// Convert server-side diff result to diff.DiffResult
				// NormalizedLive = LiveState, PredictedLive = TargetState
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

// newLocalServerSideProvider creates a provider for local manifests with server-side generation
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

// newLocalClientSideProvider creates a provider for local manifests with client-side generation
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
			argoSettings.AppLabelKey,
			cluster.Info.ServerVersion,
			cluster.Info.APIVersions,
			argoSettings.KustomizeOptions,
			argoSettings.TrackingMethod,
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

// newTrackingWrapper wraps a provider to add tracking labels to target manifests
func newTrackingWrapper(
	baseProvider manifestProvider,
	app *argoappv1.Application,
	argoSettings *settings.Settings,
) manifestProvider {
	return func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		targetManifests, err := baseProvider(ctx)
		if err != nil {
			return nil, err
		}

		resourceTracking := argo.NewResourceTracking()
		appName := app.InstanceName(argoSettings.ControllerNamespace)
		namespace := app.Spec.Destination.Namespace

		for i := range targetManifests {
			if targetManifests[i] == nil || kube.IsCRD(targetManifests[i]) {
				continue
			}

			err := resourceTracking.SetAppInstance(
				targetManifests[i],
				argoSettings.AppLabelKey,
				appName,
				namespace,
				argoappv1.TrackingMethod(argoSettings.GetTrackingMethod()),
				argoSettings.GetInstallationID(),
			)
			if err != nil {
				return nil, err
			}
		}
		return targetManifests, nil
	}
}

// newLiveManifestProvider creates a provider for live manifests from ManagedResources
func newLiveManifestProvider(liveState *application.ManagedResourcesResponse) manifestProvider {
	return func(_ context.Context) ([]*unstructured.Unstructured, error) {
		liveObjects, err := cmdutil.LiveObjects(liveState.Items)
		if err != nil {
			return nil, err
		}
		return liveObjects, nil
	}
}

// computeDiff computes the diff using a target manifest provider
// Returns a list of comparisonObject containing all resources (added, removed, and modified)
func computeDiff(
	ctx context.Context,
	app *argoappv1.Application,
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
	items := getComparisonObjects(targetManifests, liveManifests, app)

	results := make([]comparisonObject, 0)
	var potentiallyModified []comparisonObject
	for _, item := range items {
		if item.target != nil && item.live != nil {
			// Potentially modified, need to compare diffs
			potentiallyModified = append(potentiallyModified, item)
		} else {
			// Either added or removed, we already know it changed
			results = append(results, item)
		}
	}

	// Perform diff on potentially modified resources
	if len(potentiallyModified) > 0 {
		diffResults, err := performDiff(ctx, potentiallyModified)
		if err != nil {
			return nil, err
		}

		for i, diffRes := range diffResults {
			if !diffRes.Modified {
				// only include modified resources
				continue
			}

			var live, target *unstructured.Unstructured

			if len(diffRes.NormalizedLive) > 0 {
				live = &unstructured.Unstructured{}
				err = json.Unmarshal(diffRes.NormalizedLive, live)
				if err != nil {
					return nil, err
				}
			}

			if len(diffRes.PredictedLive) > 0 {
				target = &unstructured.Unstructured{}
				err = json.Unmarshal(diffRes.PredictedLive, target)
				if err != nil {
					return nil, err
				}
			}

			results = append(results, comparisonObject{
				key:    potentiallyModified[i].key,
				live:   live,
				target: target,
			})
		}
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
				fmt.Fprintf(os.Stderr, "Warning: Application does not have ServerSideDiff=true annotation.\n")
			}

			// Server side diff with local requires server side generate to be set as there will be a mismatch with client-generated manifests.
			if serverSideDiff && local != "" && !serverSideGenerate {
				log.Fatal("--server-side-diff with --local requires --server-side-generate.")
			}

			proj := getProject(ctx, c, clientOpts, app.Spec.Project)

			// Create target manifest provider based on flags
			var getTargetManifests manifestProvider

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
					fmt.Fprintf(os.Stderr, "Warning: local diff without --server-side-generate is deprecated and does not work with plugins. Server-side generation will be the default in v2.7.")
					conn, clusterIf := clientset.NewClusterClientOrDie()
					defer io.Close(conn)
					getTargetManifests = newLocalClientSideProvider(clusterIf, argoSettings, app, proj.Project, local, localRepoRoot)
				}

			default:
				getTargetManifests = newDefaultTargetProvider(liveState)
			}

			// Wrap with tracking
			getTargetManifestsWithTracking := newTrackingWrapper(getTargetManifests, app, argoSettings)

			// Create live manifest provider
			getLiveManifests := newLiveManifestProvider(liveState)

			// Create diff strategy based on --server-side-diff flag
			var performDiff diffStrategy
			if serverSideDiff {
				performDiff = createServerSideDiffStrategy(app, appIf, appName, appNs, serverSideDiffConcurrency, serverSideDiffMaxBatchKB)
			} else {
				// Build diff config once for client-side diff
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
				errors.CheckError(err)

				performDiff = createClientSideDiffStrategy(diffConfig)
			}

			// Compute diff
			results, err := computeDiff(ctx, app, getTargetManifestsWithTracking, getLiveManifests, performDiff)
			errors.CheckError(err)

			// Print added resources
			for _, result := range results {
				if result.target != nil && result.live == nil {
					printResourceDiff(result.key.Group, result.key.Kind, result.key.Namespace, result.key.Name, result.live, result.target)
				}
			}

			// Print removed resources
			for _, result := range results {
				if result.target == nil && result.live != nil {
					printResourceDiff(result.key.Group, result.key.Kind, result.key.Namespace, result.key.Name, result.live, result.target)
				}
			}

			// Print modified resources
			for _, result := range results {
				if result.target != nil && result.live != nil {
					printResourceDiff(result.key.Group, result.key.Kind, result.key.Namespace, result.key.Name, result.live, result.target)
				}
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

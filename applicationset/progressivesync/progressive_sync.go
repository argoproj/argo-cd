package progressivesync

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ReverseDeletionOrder      = "Reverse"
	AllAtOnceDeletionOrder    = "AllAtOnce"
	revisionAndSpecChangedMsg = "Application has pending changes (revision and spec differ), setting status to Waiting"
	revisionChangedMsg        = "Application has pending changes, setting status to Waiting"
	specChangedMsg            = "Application has pending changes (spec differs), setting status to Waiting"
)

type deleteInOrder struct {
	AppName string
	Step    int
}

// Dependencies is the interface for dependencies of the Manager.
// It serves two purposes: 1) it prevents progressive sync from having direct access
// to the ApplicationSet controller, and 2) it allows for easy mocking in tests.
type Dependencies interface {
	// SetAppSetApplicationStatus persists ApplicationSet status.ApplicationStatus field
	SetAppSetApplicationStatus(
		ctx context.Context,
		logCtx *log.Entry,
		applicationSet *argov1alpha1.ApplicationSet,
		applicationStatuses []argov1alpha1.ApplicationSetApplicationStatus,
	) error

	// SetApplicationSetStatusCondition persists ApplicationSet conditions
	SetApplicationSetStatusCondition(
		ctx context.Context,
		applicationSet *argov1alpha1.ApplicationSet,
		condition argov1alpha1.ApplicationSetCondition,
		parametersGenerated bool,
	) error
}

type Manager struct {
	Client       client.Client
	dependencies Dependencies
}

// NewManager creates a new manager with dependencies
func NewManager(client client.Client, dependencies Dependencies) *Manager {
	return &Manager{
		Client:       client,
		dependencies: dependencies,
	}
}

func (m *Manager) PerformProgressiveSyncs(ctx context.Context, logCtx *log.Entry, appset argov1alpha1.ApplicationSet, applications []argov1alpha1.Application, desiredApplications []argov1alpha1.Application) (map[string]bool, error) {
	appDependencyList, appStepMap := buildAppDependencyList(logCtx, appset, desiredApplications)

	_, err := m.UpdateApplicationSetApplicationStatus(ctx, logCtx, &appset, applications, desiredApplications, appStepMap)
	if err != nil {
		return nil, fmt.Errorf("failed to update applicationset app status: %w", err)
	}

	logCtx.Infof("ApplicationSet %v step list:", appset.Name)
	for stepIndex, applicationNames := range appDependencyList {
		logCtx.Infof("step %v: %+v", stepIndex+1, applicationNames)
	}

	appsToSync := getAppsToSync(appset, appDependencyList, applications)
	logCtx.Infof("Application allowed to sync before maxUpdate?: %+v", appsToSync)

	_, err = m.UpdateApplicationSetApplicationStatusProgress(ctx, logCtx, &appset, appsToSync, appStepMap)
	if err != nil {
		return nil, fmt.Errorf("failed to update applicationset application status progress: %w", err)
	}

	_ = m.updateApplicationSetApplicationStatusConditions(ctx, &appset)

	return appsToSync, nil
}

func (m *Manager) PerformReverseDeletion(ctx context.Context, logCtx *log.Entry, appset argov1alpha1.ApplicationSet, currentApps []argov1alpha1.Application) (time.Duration, error) {
	requeueTime := 10 * time.Second
	stepLength := len(appset.Spec.Strategy.RollingSync.Steps)

	// map applications by name using current applications
	appMap := make(map[string]*argov1alpha1.Application)
	for _, app := range currentApps {
		appMap[app.Name] = &app
	}

	// Get Rolling Sync Step Maps
	_, appStepMap := buildAppDependencyList(logCtx, appset, currentApps)
	// reverse the AppStepMap to perform deletion
	var reverseDeleteAppSteps []deleteInOrder
	for appName, appStep := range appStepMap {
		reverseDeleteAppSteps = append(reverseDeleteAppSteps, deleteInOrder{appName, stepLength - appStep - 1})
	}

	sort.Slice(reverseDeleteAppSteps, func(i, j int) bool {
		return reverseDeleteAppSteps[i].Step < reverseDeleteAppSteps[j].Step
	})

	for _, step := range reverseDeleteAppSteps {
		logCtx.Infof("step %v : app %v", step.Step, step.AppName)
		app := appMap[step.AppName]
		retrievedApp := argov1alpha1.Application{}
		if err := m.Client.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, &retrievedApp); err != nil {
			if apierrors.IsNotFound(err) {
				logCtx.Infof("application %s successfully deleted", step.AppName)
				continue
			}
		}
		// Check if the application is already being deleted
		if retrievedApp.DeletionTimestamp != nil {
			logCtx.Infof("application %s has been marked for deletion, but object not removed yet", step.AppName)
			if time.Since(retrievedApp.DeletionTimestamp.Time) > 2*time.Minute {
				return 0, errors.New("application has not been deleted in over 2 minutes")
			}
		}
		// The application has not been deleted yet, trigger its deletion
		if err := m.Client.Delete(ctx, &retrievedApp); err != nil {
			return 0, err
		}
		return requeueTime, nil
	}
	logCtx.Infof("completed reverse deletion for ApplicationSet %v", appset.Name)
	return 0, nil
}

// this list tracks which Applications belong to each RollingUpdate step
func buildAppDependencyList(logCtx *log.Entry, applicationSet argov1alpha1.ApplicationSet, applications []argov1alpha1.Application) ([][]string, map[string]int) {
	if applicationSet.Spec.Strategy == nil || applicationSet.Spec.Strategy.Type == "" || applicationSet.Spec.Strategy.Type == "AllAtOnce" {
		return [][]string{}, map[string]int{}
	}

	steps := []argov1alpha1.ApplicationSetRolloutStep{}
	if RollingSyncStrategyEnabled(&applicationSet) {
		steps = applicationSet.Spec.Strategy.RollingSync.Steps
	}

	appDependencyList := make([][]string, 0)
	for range steps {
		appDependencyList = append(appDependencyList, make([]string, 0))
	}

	appStepMap := map[string]int{}

	// use applicationLabelSelectors to filter generated Applications into steps and status by name
	for _, app := range applications {
		for i, step := range steps {
			selected := true // default to true, assuming the current Application is a match for the given step matchExpression

			for _, matchExpression := range step.MatchExpressions {
				if val, ok := app.Labels[matchExpression.Key]; ok {
					valueMatched := labelMatchedExpression(logCtx, val, matchExpression)

					if !valueMatched { // none of the matchExpression values was a match with the Application's labels
						selected = false
						break
					}
				} else if matchExpression.Operator == "In" {
					selected = false // no matching label key with "In" operator means this Application will not be included in the current step
					break
				}
			}

			if selected {
				appDependencyList[i] = append(appDependencyList[i], app.Name)
				if val, ok := appStepMap[app.Name]; ok {
					logCtx.Warnf("AppSet '%v' has a invalid matchExpression that selects Application '%v' label twice, in steps %v and %v", applicationSet.Name, app.Name, val+1, i+1)
				} else {
					appStepMap[app.Name] = i
				}
			}
		}
	}

	return appDependencyList, appStepMap
}

func labelMatchedExpression(logCtx *log.Entry, val string, matchExpression argov1alpha1.ApplicationMatchExpression) bool {
	if matchExpression.Operator != "In" && matchExpression.Operator != "NotIn" {
		logCtx.Errorf("skipping AppSet rollingUpdate step Application selection, invalid matchExpression operator provided: %q ", matchExpression.Operator)
		return false
	}

	// if operator == In, default to false
	// if operator == NotIn, default to true
	valueMatched := matchExpression.Operator == "NotIn"

	if slices.Contains(matchExpression.Values, val) {
		// first "In" match returns true
		// first "NotIn" match returns false
		return matchExpression.Operator == "In"
	}
	return valueMatched
}

func getAppStep(appName string, appStepMap map[string]int) int {
	// if an application is not selected by any match expression, it defaults to step -1
	step := -1
	if appStep, ok := appStepMap[appName]; ok {
		// 1-based indexing
		step = appStep + 1
	}
	return step
}

// check the status of each Application's status and promote Applications to the next status if needed
// update AppSet status in-memory, controller will persist it
func (m *Manager) UpdateApplicationSetApplicationStatus(ctx context.Context, logCtx *log.Entry, applicationSet *argov1alpha1.ApplicationSet, applications []argov1alpha1.Application, desiredApplications []argov1alpha1.Application, appStepMap map[string]int) ([]argov1alpha1.ApplicationSetApplicationStatus, error) {
	now := metav1.Now()
	appStatuses := make([]argov1alpha1.ApplicationSetApplicationStatus, 0, len(applications))

	// Build a map of desired applications for quick lookup
	desiredAppsMap := make(map[string]*argov1alpha1.Application)
	for i := range desiredApplications {
		desiredAppsMap[desiredApplications[i].Name] = &desiredApplications[i]
	}

	for _, app := range applications {
		appHealthStatus := app.Status.Health.Status
		appSyncStatus := app.Status.Sync.Status

		currentAppStatus := argov1alpha1.ApplicationSetApplicationStatus{}
		idx := utils.FindApplicationStatusIndex(applicationSet.Status.ApplicationStatus, app.Name)
		if idx == -1 {
			// AppStatus not found, set default status of "Waiting"
			currentAppStatus = argov1alpha1.ApplicationSetApplicationStatus{
				Application:        app.Name,
				TargetRevisions:    app.Status.GetRevisions(),
				LastTransitionTime: &now,
				Message:            "No Application status found, defaulting status to Waiting",
				Status:             argov1alpha1.ProgressiveSyncWaiting,
				Step:               strconv.Itoa(getAppStep(app.Name, appStepMap)),
			}
		} else {
			// we have an existing AppStatus
			currentAppStatus = applicationSet.Status.ApplicationStatus[idx]
		}

		statusLogCtx := logCtx.WithFields(log.Fields{
			"app.name":               currentAppStatus.Application,
			"app.health":             appHealthStatus,
			"app.sync":               appSyncStatus,
			"status.status":          currentAppStatus.Status,
			"status.message":         currentAppStatus.Message,
			"status.step":            currentAppStatus.Step,
			"status.targetRevisions": strings.Join(currentAppStatus.TargetRevisions, ","),
		})

		newAppStatus := currentAppStatus.DeepCopy()
		newAppStatus.Step = strconv.Itoa(getAppStep(newAppStatus.Application, appStepMap))

		revisionsChanged := !reflect.DeepEqual(currentAppStatus.TargetRevisions, app.Status.GetRevisions())

		// Check if the desired Application spec differs from the current Application spec
		specChanged := false
		if desiredApp, ok := desiredAppsMap[app.Name]; ok {
			// Compare the desired spec with the current spec to detect non-Git changes
			// This will catch changes to generator parameters like image tags, helm values, etc.
			specChanged = !cmp.Equal(desiredApp.Spec, app.Spec, cmpopts.EquateEmpty(), cmpopts.EquateComparable(argov1alpha1.ApplicationDestination{}))
		}

		if revisionsChanged || specChanged {
			newAppStatus.TargetRevisions = app.Status.GetRevisions()

			switch {
			case revisionsChanged && specChanged:
				newAppStatus.Message = revisionAndSpecChangedMsg
			case revisionsChanged:
				newAppStatus.Message = revisionChangedMsg
			default:
				newAppStatus.Message = specChangedMsg
			}
			newAppStatus.Status = argov1alpha1.ProgressiveSyncWaiting
			newAppStatus.LastTransitionTime = &now
		}

		if newAppStatus.Status == argov1alpha1.ProgressiveSyncWaiting {
			// App has changed to waiting because the TargetRevisions changed or it is a new selected app
			// This does not mean we should always sync the app. The app may not be OutOfSync
			// and may not require a sync if it does not have differences.
			if appSyncStatus == argov1alpha1.SyncStatusCodeSynced {
				if app.Status.Health.Status == health.HealthStatusHealthy {
					newAppStatus.LastTransitionTime = &now
					newAppStatus.Status = argov1alpha1.ProgressiveSyncHealthy
					newAppStatus.Message = "Application resource has synced, updating status to Healthy"
				} else {
					newAppStatus.LastTransitionTime = &now
					newAppStatus.Status = argov1alpha1.ProgressiveSyncProgressing
					newAppStatus.Message = "Application resource has synced, updating status to Progressing"
				}
			}
		} else {
			// The target revision is the same, so we need to evaluate the current revision progress
			if currentAppStatus.Status == argov1alpha1.ProgressiveSyncPending {
				// No need to evaluate status health further if the application did not change since our last transition
				if app.Status.ReconciledAt == nil || (newAppStatus.LastTransitionTime != nil && app.Status.ReconciledAt.After(newAppStatus.LastTransitionTime.Time)) {
					// Validate that at least one sync was trigerred after the pending transition time
					if app.Status.OperationState != nil && app.Status.OperationState.StartedAt.After(currentAppStatus.LastTransitionTime.Time) {
						statusLogCtx = statusLogCtx.WithField("app.operation", app.Status.OperationState.Phase)
						newAppStatus.LastTransitionTime = &now
						newAppStatus.Status = argov1alpha1.ProgressiveSyncProgressing

						switch {
						case app.Status.OperationState.Phase.Successful():
							newAppStatus.Message = "Application resource completed a sync successfully, updating status from Pending to Progressing"
						case app.Status.OperationState.Phase.Completed():
							newAppStatus.Message = "Application resource completed a sync, updating status from Pending to Progressing"
						default:
							// If a sync fails or has errors, the Application should be configured with retry. It is not the appset's job to retry failed syncs
							newAppStatus.Message = "Application resource became Progressing, updating status from Pending to Progressing"
						}
					} else if isApplicationWithError(app) {
						// Validate if the application has errors preventing it to be reconciled and perform syncs
						// If it does, we move it to progressing.
						newAppStatus.LastTransitionTime = &now
						newAppStatus.Status = argov1alpha1.ProgressiveSyncProgressing
						newAppStatus.Message = "Application resource has error and cannot sync, updating status to Progressing"
					}
				}
			}

			if currentAppStatus.Status == argov1alpha1.ProgressiveSyncProgressing {
				// If the status has reached progressing, we know a sync has been triggered. No matter the result of that operation,
				// we want an the app to reach the Healthy state for the current revision.
				if appHealthStatus == health.HealthStatusHealthy && appSyncStatus == argov1alpha1.SyncStatusCodeSynced {
					newAppStatus.LastTransitionTime = &now
					newAppStatus.Status = argov1alpha1.ProgressiveSyncHealthy
					newAppStatus.Message = "Application resource became Healthy, updating status from Progressing to Healthy"
				}
			}
		}

		if newAppStatus.LastTransitionTime == &now {
			statusLogCtx.WithFields(log.Fields{
				"new_status.status":          newAppStatus.Status,
				"new_status.message":         newAppStatus.Message,
				"new_status.step":            newAppStatus.Step,
				"new_status.targetRevisions": strings.Join(newAppStatus.TargetRevisions, ","),
			}).Info("Progressive sync application changed status")
		}
		appStatuses = append(appStatuses, *newAppStatus)
	}

	err := m.dependencies.SetAppSetApplicationStatus(ctx, logCtx, applicationSet, appStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to set AppSet application statuses: %w", err)
	}

	return appStatuses, nil
}

// getAppsToSync returns a Map of Applications that should be synced in this progressive sync wave
func getAppsToSync(applicationSet argov1alpha1.ApplicationSet, appDependencyList [][]string, currentApplications []argov1alpha1.Application) map[string]bool {
	appSyncMap := map[string]bool{}
	currentAppsMap := map[string]bool{}

	for _, app := range currentApplications {
		currentAppsMap[app.Name] = true
	}

	for stepIndex := range appDependencyList {
		// set the syncEnabled boolean for every Application in the current step
		for _, appName := range appDependencyList[stepIndex] {
			appSyncMap[appName] = true
		}

		// evaluate if we need to sync next waves
		syncNextWave := true
		for _, appName := range appDependencyList[stepIndex] {
			// Check if application is created and managed by this AppSet, if it is not created yet, we cannot progress
			if _, ok := currentAppsMap[appName]; !ok {
				syncNextWave = false
				break
			}

			idx := utils.FindApplicationStatusIndex(applicationSet.Status.ApplicationStatus, appName)
			if idx == -1 {
				// No Application status found, likely because the Application is being newly created
				// This mean this wave is not yet completed
				syncNextWave = false
				break
			}

			appStatus := applicationSet.Status.ApplicationStatus[idx]
			if appStatus.Status != argov1alpha1.ProgressiveSyncHealthy {
				// At least one application in this wave is not yet healthy. We cannot proceed to the next wave
				syncNextWave = false
				break
			}
		}
		if !syncNextWave {
			break
		}
	}

	return appSyncMap
}

func IsRollingSyncStrategy(appset *argov1alpha1.ApplicationSet) bool {
	// It's only RollingSync if the type specifically sets it
	return appset.Spec.Strategy != nil && appset.Spec.Strategy.Type == "RollingSync" && appset.Spec.Strategy.RollingSync != nil
}

func RollingSyncStrategyEnabled(appset *argov1alpha1.ApplicationSet) bool {
	// ProgressiveSync is enabled if the strategy is set to `RollingSync` + steps slice is not empty
	return IsRollingSyncStrategy(appset) && len(appset.Spec.Strategy.RollingSync.Steps) > 0
}

func IsDeletionOrderReversed(appset *argov1alpha1.ApplicationSet) bool {
	// When progressive sync is enabled + deletionOrder is set to Reverse (case-insensitive)
	return RollingSyncStrategyEnabled(appset) && strings.EqualFold(appset.Spec.Strategy.DeletionOrder, ReverseDeletionOrder)
}

func isApplicationWithError(app argov1alpha1.Application) bool {
	for _, condition := range app.Status.Conditions {
		if condition.Type == argov1alpha1.ApplicationConditionInvalidSpecError {
			return true
		}
		if condition.Type == argov1alpha1.ApplicationConditionUnknownError {
			return true
		}
	}
	return false
}

// check Applications that are in Waiting status and promote them to Pending if needed
func (m *Manager) UpdateApplicationSetApplicationStatusProgress(ctx context.Context, logCtx *log.Entry, applicationSet *argov1alpha1.ApplicationSet, appsToSync map[string]bool, appStepMap map[string]int) ([]argov1alpha1.ApplicationSetApplicationStatus, error) {
	now := metav1.Now()

	appStatuses := make([]argov1alpha1.ApplicationSetApplicationStatus, 0, len(applicationSet.Status.ApplicationStatus))

	// if we have no RollingUpdate steps, clear out the existing ApplicationStatus entries
	if RollingSyncStrategyEnabled(applicationSet) {
		length := len(applicationSet.Spec.Strategy.RollingSync.Steps)

		updateCountMap := make([]int, length)
		totalCountMap := make([]int, length)

		// populate updateCountMap with counts of existing Pending and Progressing Applications
		for _, appStatus := range applicationSet.Status.ApplicationStatus {
			totalCountMap[appStepMap[appStatus.Application]]++

			if appStatus.Status == argov1alpha1.ProgressiveSyncPending || appStatus.Status == argov1alpha1.ProgressiveSyncProgressing {
				updateCountMap[appStepMap[appStatus.Application]]++
			}
		}

		for _, appStatus := range applicationSet.Status.ApplicationStatus {
			statusLogCtx := logCtx.WithFields(log.Fields{
				"app.name":               appStatus.Application,
				"status.status":          appStatus.Status,
				"status.message":         appStatus.Message,
				"status.step":            appStatus.Step,
				"status.targetRevisions": strings.Join(appStatus.TargetRevisions, ","),
			})

			maxUpdateAllowed := true
			maxUpdate := &intstr.IntOrString{}
			if RollingSyncStrategyEnabled(applicationSet) {
				maxUpdate = applicationSet.Spec.Strategy.RollingSync.Steps[appStepMap[appStatus.Application]].MaxUpdate
			}

			// by default allow all applications to update if maxUpdate is unset
			if maxUpdate != nil {
				maxUpdateVal, err := intstr.GetScaledValueFromIntOrPercent(maxUpdate, totalCountMap[appStepMap[appStatus.Application]], false)
				if err != nil {
					statusLogCtx.Warnf("AppSet has a invalid maxUpdate value '%+v', ignoring maxUpdate logic for this step: %v", maxUpdate, err)
				}

				// ensure that percentage values greater than 0% always result in at least 1 Application being selected
				if maxUpdate.Type == intstr.String && maxUpdate.StrVal != "0%" && maxUpdateVal < 1 {
					maxUpdateVal = 1
				}

				if updateCountMap[appStepMap[appStatus.Application]] >= maxUpdateVal {
					maxUpdateAllowed = false
					statusLogCtx.Infof("Application is not allowed to update yet, %v/%v Applications already updating in step %v", updateCountMap[appStepMap[appStatus.Application]], maxUpdateVal, getAppStep(appStatus.Application, appStepMap))
				}
			}

			if appStatus.Status == argov1alpha1.ProgressiveSyncWaiting && appsToSync[appStatus.Application] && maxUpdateAllowed {
				appStatus.LastTransitionTime = &now
				appStatus.Status = argov1alpha1.ProgressiveSyncPending
				appStatus.Message = "Application moved to Pending status, watching for the Application resource to start Progressing"

				statusLogCtx.WithFields(log.Fields{
					"new_status.status":          appStatus.Status,
					"new_status.message":         appStatus.Message,
					"new_status.step":            appStatus.Step,
					"new_status.targetRevisions": strings.Join(appStatus.TargetRevisions, ","),
				}).Info("Progressive sync application changed status")

				updateCountMap[appStepMap[appStatus.Application]]++
			}

			appStatuses = append(appStatuses, appStatus)
		}
	}

	err := m.dependencies.SetAppSetApplicationStatus(ctx, logCtx, applicationSet, appStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to set AppSet app status: %w", err)
	}

	return appStatuses, nil
}

func (m *Manager) updateApplicationSetApplicationStatusConditions(ctx context.Context, applicationSet *argov1alpha1.ApplicationSet) []argov1alpha1.ApplicationSetCondition {
	if !IsRollingSyncStrategy(applicationSet) {
		return applicationSet.Status.Conditions
	}

	completedWaves := map[string]bool{}
	for _, appStatus := range applicationSet.Status.ApplicationStatus {
		if v, ok := completedWaves[appStatus.Step]; !ok {
			completedWaves[appStatus.Step] = appStatus.Status == argov1alpha1.ProgressiveSyncHealthy
		} else {
			completedWaves[appStatus.Step] = v && appStatus.Status == argov1alpha1.ProgressiveSyncHealthy
		}
	}

	isProgressing := false
	progressingStep := ""
	for i := range applicationSet.Spec.Strategy.RollingSync.Steps {
		step := strconv.Itoa(i + 1)
		isCompleted, ok := completedWaves[step]
		if !ok {
			// Step has no applications, so it is completed
			continue
		}
		if !isCompleted {
			isProgressing = true
			progressingStep = step
			break
		}
	}

	if isProgressing {
		_ = m.dependencies.SetApplicationSetStatusCondition(ctx,
			applicationSet,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionRolloutProgressing,
				Message: "ApplicationSet is performing rollout of step " + progressingStep,
				Reason:  argov1alpha1.ApplicationSetReasonApplicationSetModified,
				Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
			}, true,
		)
	} else {
		_ = m.dependencies.SetApplicationSetStatusCondition(ctx,
			applicationSet,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionRolloutProgressing,
				Message: "ApplicationSet Rollout has completed",
				Reason:  argov1alpha1.ApplicationSetReasonApplicationSetRolloutComplete,
				Status:  argov1alpha1.ApplicationSetConditionStatusFalse,
			}, true,
		)
	}
	return applicationSet.Status.Conditions
}

func (m *Manager) SyncDesiredApplications(logCtx *log.Entry, applicationSet *argov1alpha1.ApplicationSet, appsToSync map[string]bool, desiredApplications []argov1alpha1.Application) []argov1alpha1.Application {
	rolloutApps := []argov1alpha1.Application{}
	for i := range desiredApplications {
		pruneEnabled := false

		// ensure that Applications generated with RollingSync do not have an automated sync policy, since the AppSet controller will handle triggering the sync operation instead
		if desiredApplications[i].Spec.SyncPolicy != nil && desiredApplications[i].Spec.SyncPolicy.IsAutomatedSyncEnabled() {
			pruneEnabled = desiredApplications[i].Spec.SyncPolicy.Automated.GetPrune()
			desiredApplications[i].Spec.SyncPolicy.Automated.Enabled = new(false)
		}

		appSetStatusPending := false
		idx := utils.FindApplicationStatusIndex(applicationSet.Status.ApplicationStatus, desiredApplications[i].Name)
		if idx > -1 && applicationSet.Status.ApplicationStatus[idx].Status == argov1alpha1.ProgressiveSyncPending {
			// only trigger a sync for Applications that are in Pending status, since this is governed by maxUpdate
			appSetStatusPending = true
		}

		// check appsToSync to determine which Applications are ready to be updated and which should be skipped
		if appsToSync[desiredApplications[i].Name] && appSetStatusPending {
			logCtx.Infof("triggering sync for application: %v, prune enabled: %v", desiredApplications[i].Name, pruneEnabled)
			desiredApplications[i] = syncApplication(desiredApplications[i], pruneEnabled)
		}

		rolloutApps = append(rolloutApps, desiredApplications[i])
	}
	return rolloutApps
}

// used by the RollingSync Progressive Sync strategy to trigger a sync of a particular Application resource
func syncApplication(application argov1alpha1.Application, prune bool) argov1alpha1.Application {
	operation := argov1alpha1.Operation{
		InitiatedBy: argov1alpha1.OperationInitiator{
			Username:  "applicationset-controller",
			Automated: true,
		},
		Info: []*argov1alpha1.Info{
			{
				Name:  "Reason",
				Value: "ApplicationSet RollingSync triggered a sync of this Application resource",
			},
		},
		Sync: &argov1alpha1.SyncOperation{},
		// Set a retry limit of 5, aligning with the default in Argo CD's appcontroller auto-sync behavior.
		// This provides consistency for retry behavior across controllers.
		// See: https://github.com/argoproj/argo-cd/blob/af9ebac0bb35dc16eb034c1cefaf7c92d1029927/controller/appcontroller.go#L2126
		Retry: argov1alpha1.RetryStrategy{Limit: 5},
	}

	if application.Spec.SyncPolicy != nil {
		if application.Spec.SyncPolicy.Retry != nil {
			operation.Retry = *application.Spec.SyncPolicy.Retry
		}
		if application.Spec.SyncPolicy.SyncOptions != nil {
			operation.Sync.SyncOptions = application.Spec.SyncPolicy.SyncOptions
		}
		operation.Sync.Prune = prune
	}
	application.Operation = &operation

	return application
}

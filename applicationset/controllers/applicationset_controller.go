/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/db"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	argoutil "github.com/argoproj/argo-cd/v2/util/argo"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

const (
	// Rather than importing the whole argocd-notifications controller, just copying the const here
	//   https://github.com/argoproj-labs/argocd-notifications/blob/33d345fa838829bb50fca5c08523aba380d2c12b/pkg/controller/subscriptions.go#L12
	//   https://github.com/argoproj-labs/argocd-notifications/blob/33d345fa838829bb50fca5c08523aba380d2c12b/pkg/controller/state.go#L17
	NotifiedAnnotationKey             = "notified.notifications.argoproj.io"
	ReconcileRequeueOnValidationError = time.Minute * 3

	// LabelKeyAppSetInstance is the label key to use to uniquely identify the apps of an applicationset
	// The ArgoCD applicationset name is used as the instance name
	LabelKeyAppSetInstance = "argocd.argoproj.io/application-set-name"
)

var (
	defaultPreservedAnnotations = []string{
		NotifiedAnnotationKey,
		argov1alpha1.AnnotationKeyRefresh,
	}
)

// ApplicationSetReconciler reconciles a ApplicationSet object
type ApplicationSetReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	Generators           map[string]generators.Generator
	ArgoDB               db.ArgoDB
	ArgoAppClientset     appclientset.Interface
	KubeClientset        kubernetes.Interface
	Policy               argov1alpha1.ApplicationsSyncPolicy
	EnablePolicyOverride bool
	utils.Renderer

	EnableProgressiveSyncs bool
}

// +kubebuilder:rbac:groups=argoproj.io,resources=applicationsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argoproj.io,resources=applicationsets/status,verbs=get;update;patch

func (r *ApplicationSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logCtx := log.WithField("applicationset", req.NamespacedName)

	var applicationSetInfo argov1alpha1.ApplicationSet
	parametersGenerated := false

	if err := r.Get(ctx, req.NamespacedName, &applicationSetInfo); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logCtx.WithError(err).Infof("unable to get ApplicationSet: '%v' ", err)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Do not attempt to further reconcile the ApplicationSet if it is being deleted.
	if applicationSetInfo.ObjectMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	// Log a warning if there are unrecognized generators
	_ = utils.CheckInvalidGenerators(&applicationSetInfo)
	// desiredApplications is the main list of all expected Applications from all generators in this appset.
	desiredApplications, applicationSetReason, err := r.generateApplications(applicationSetInfo)
	if err != nil {
		_ = r.setApplicationSetStatusCondition(ctx,
			&applicationSetInfo,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
				Message: err.Error(),
				Reason:  string(applicationSetReason),
				Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
			}, parametersGenerated,
		)
		return ctrl.Result{}, err
	}

	parametersGenerated = true

	validateErrors, err := r.validateGeneratedApplications(ctx, desiredApplications, applicationSetInfo, req.Namespace)
	if err != nil {
		// While some generators may return an error that requires user intervention,
		// other generators reference external resources that may change to cause
		// the error to no longer occur. We thus log the error and requeue
		// with a timeout to give this another shot at a later time.
		//
		// Changes to watched resources will cause this to be reconciled sooner than
		// the RequeueAfter time.
		logCtx.Errorf("error occurred during application validation: %s", err.Error())

		_ = r.setApplicationSetStatusCondition(ctx,
			&applicationSetInfo,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
				Message: err.Error(),
				Reason:  argov1alpha1.ApplicationSetReasonApplicationValidationError,
				Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
			}, parametersGenerated,
		)
		return ctrl.Result{RequeueAfter: ReconcileRequeueOnValidationError}, nil
	}

	// appMap is a name->app collection of Applications in this ApplicationSet.
	appMap := map[string]argov1alpha1.Application{}
	// appSyncMap tracks which apps will be synced during this reconciliation.
	appSyncMap := map[string]bool{}

	if r.EnableProgressiveSyncs {
		if applicationSetInfo.Spec.Strategy == nil && len(applicationSetInfo.Status.ApplicationStatus) > 0 {
			log.Infof("Removing %v unnecessary AppStatus entries from ApplicationSet %v", len(applicationSetInfo.Status.ApplicationStatus), applicationSetInfo.Name)

			err := r.setAppSetApplicationStatus(ctx, &applicationSetInfo, []argov1alpha1.ApplicationSetApplicationStatus{})
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to clear previous AppSet application statuses for %v: %w", applicationSetInfo.Name, err)
			}
		} else {
			applications, err := r.getCurrentApplications(ctx, applicationSetInfo)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get current applications for application set: %w", err)
			}

			for _, app := range applications {
				appMap[app.Name] = app
			}

			appSyncMap, err = r.performProgressiveSyncs(ctx, applicationSetInfo, applications, desiredApplications, appMap)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to perform progressive sync reconciliation for application set: %w", err)
			}
		}
	}

	var validApps []argov1alpha1.Application
	for i := range desiredApplications {
		if validateErrors[i] == nil {
			validApps = append(validApps, desiredApplications[i])
		}
	}

	if len(validateErrors) > 0 {
		var message string
		for _, v := range validateErrors {
			message = v.Error()
			logCtx.Errorf("validation error found during application validation: %s", message)
		}
		if len(validateErrors) > 1 {
			// Only the last message gets added to the appset status, to keep the size reasonable.
			message = fmt.Sprintf("%s (and %d more)", message, len(validateErrors)-1)
		}
		_ = r.setApplicationSetStatusCondition(ctx,
			&applicationSetInfo,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
				Message: message,
				Reason:  argov1alpha1.ApplicationSetReasonApplicationValidationError,
				Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
			}, parametersGenerated,
		)
	}

	if r.EnableProgressiveSyncs {
		// trigger appropriate application syncs if RollingSync strategy is enabled
		if progressiveSyncsStrategyEnabled(&applicationSetInfo, "RollingSync") {
			validApps, err = r.syncValidApplications(ctx, &applicationSetInfo, appSyncMap, appMap, validApps)

			if err != nil {
				_ = r.setApplicationSetStatusCondition(ctx,
					&applicationSetInfo,
					argov1alpha1.ApplicationSetCondition{
						Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
						Message: err.Error(),
						Reason:  argov1alpha1.ApplicationSetReasonSyncApplicationError,
						Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
					}, parametersGenerated,
				)
				return ctrl.Result{}, err
			}
		}
	}

	if utils.DefaultPolicy(applicationSetInfo.Spec.SyncPolicy, r.Policy, r.EnablePolicyOverride).AllowUpdate() {
		err = r.createOrUpdateInCluster(ctx, applicationSetInfo, validApps)
		if err != nil {
			_ = r.setApplicationSetStatusCondition(ctx,
				&applicationSetInfo,
				argov1alpha1.ApplicationSetCondition{
					Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
					Message: err.Error(),
					Reason:  argov1alpha1.ApplicationSetReasonUpdateApplicationError,
					Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
				}, parametersGenerated,
			)
			return ctrl.Result{}, err
		}
	} else {
		err = r.createInCluster(ctx, applicationSetInfo, validApps)
		if err != nil {
			_ = r.setApplicationSetStatusCondition(ctx,
				&applicationSetInfo,
				argov1alpha1.ApplicationSetCondition{
					Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
					Message: err.Error(),
					Reason:  argov1alpha1.ApplicationSetReasonCreateApplicationError,
					Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
				}, parametersGenerated,
			)
			return ctrl.Result{}, err
		}
	}

	if utils.DefaultPolicy(applicationSetInfo.Spec.SyncPolicy, r.Policy, r.EnablePolicyOverride).AllowDelete() {
		err = r.deleteInCluster(ctx, applicationSetInfo, desiredApplications)
		if err != nil {
			_ = r.setApplicationSetStatusCondition(ctx,
				&applicationSetInfo,
				argov1alpha1.ApplicationSetCondition{
					Type:    argov1alpha1.ApplicationSetConditionResourcesUpToDate,
					Message: err.Error(),
					Reason:  argov1alpha1.ApplicationSetReasonDeleteApplicationError,
					Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
				}, parametersGenerated,
			)
			return ctrl.Result{}, err
		}
	}

	if applicationSetInfo.RefreshRequired() {
		delete(applicationSetInfo.Annotations, common.AnnotationApplicationSetRefresh)
		err := r.Client.Update(ctx, &applicationSetInfo)
		if err != nil {
			logCtx.Warnf("error occurred while updating ApplicationSet: %v", err)
			_ = r.setApplicationSetStatusCondition(ctx,
				&applicationSetInfo,
				argov1alpha1.ApplicationSetCondition{
					Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
					Message: err.Error(),
					Reason:  argov1alpha1.ApplicationSetReasonRefreshApplicationError,
					Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
				}, parametersGenerated,
			)
			return ctrl.Result{}, err
		}
	}

	requeueAfter := r.getMinRequeueAfter(&applicationSetInfo)
	logCtx.WithField("requeueAfter", requeueAfter).Info("end reconcile")

	if len(validateErrors) == 0 {
		if err := r.setApplicationSetStatusCondition(ctx,
			&applicationSetInfo,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionResourcesUpToDate,
				Message: "All applications have been generated successfully",
				Reason:  argov1alpha1.ApplicationSetReasonApplicationSetUpToDate,
				Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
			}, parametersGenerated,
		); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{
		RequeueAfter: requeueAfter,
	}, nil
}

func getParametersGeneratedCondition(parametersGenerated bool, message string) argov1alpha1.ApplicationSetCondition {
	var paramtersGeneratedCondition argov1alpha1.ApplicationSetCondition
	if parametersGenerated {
		paramtersGeneratedCondition = argov1alpha1.ApplicationSetCondition{
			Type:    argov1alpha1.ApplicationSetConditionParametersGenerated,
			Message: "Successfully generated parameters for all Applications",
			Reason:  argov1alpha1.ApplicationSetReasonParametersGenerated,
			Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
		}
	} else {
		paramtersGeneratedCondition = argov1alpha1.ApplicationSetCondition{
			Type:    argov1alpha1.ApplicationSetConditionParametersGenerated,
			Message: message,
			Reason:  argov1alpha1.ApplicationSetReasonErrorOccurred,
			Status:  argov1alpha1.ApplicationSetConditionStatusFalse,
		}
	}
	return paramtersGeneratedCondition
}

func getResourceUpToDateCondition(errorOccurred bool, message string, reason string) argov1alpha1.ApplicationSetCondition {
	var resourceUpToDateCondition argov1alpha1.ApplicationSetCondition
	if errorOccurred {
		resourceUpToDateCondition = argov1alpha1.ApplicationSetCondition{
			Type:    argov1alpha1.ApplicationSetConditionResourcesUpToDate,
			Message: message,
			Reason:  reason,
			Status:  argov1alpha1.ApplicationSetConditionStatusFalse,
		}
	} else {
		resourceUpToDateCondition = argov1alpha1.ApplicationSetCondition{
			Type:    argov1alpha1.ApplicationSetConditionResourcesUpToDate,
			Message: "ApplicationSet up to date",
			Reason:  argov1alpha1.ApplicationSetReasonApplicationSetUpToDate,
			Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
		}
	}
	return resourceUpToDateCondition
}

func (r *ApplicationSetReconciler) setApplicationSetStatusCondition(ctx context.Context, applicationSet *argov1alpha1.ApplicationSet, condition argov1alpha1.ApplicationSetCondition, paramtersGenerated bool) error {
	// check if error occurred during reconcile process
	errOccurred := condition.Type == argov1alpha1.ApplicationSetConditionErrorOccurred

	var errOccurredCondition argov1alpha1.ApplicationSetCondition

	if errOccurred {
		errOccurredCondition = condition
	} else {
		errOccurredCondition = argov1alpha1.ApplicationSetCondition{
			Type:    argov1alpha1.ApplicationSetConditionErrorOccurred,
			Message: "Successfully generated parameters for all Applications",
			Reason:  argov1alpha1.ApplicationSetReasonApplicationSetUpToDate,
			Status:  argov1alpha1.ApplicationSetConditionStatusFalse,
		}
	}

	paramtersGeneratedCondition := getParametersGeneratedCondition(paramtersGenerated, condition.Message)
	resourceUpToDateCondition := getResourceUpToDateCondition(errOccurred, condition.Message, condition.Reason)

	newConditions := []argov1alpha1.ApplicationSetCondition{errOccurredCondition, paramtersGeneratedCondition, resourceUpToDateCondition}

	needToUpdateConditions := false
	for _, condition := range newConditions {
		// do nothing if appset already has same condition
		for _, c := range applicationSet.Status.Conditions {
			if c.Type == condition.Type && (c.Reason != condition.Reason || c.Status != condition.Status || c.Message != condition.Message) {
				needToUpdateConditions = true
				break
			}
		}
	}
	evaluatedTypes := map[argov1alpha1.ApplicationSetConditionType]bool{
		argov1alpha1.ApplicationSetConditionErrorOccurred:       true,
		argov1alpha1.ApplicationSetConditionParametersGenerated: true,
		argov1alpha1.ApplicationSetConditionResourcesUpToDate:   true,
	}

	if needToUpdateConditions || len(applicationSet.Status.Conditions) < 3 {
		// fetch updated Application Set object before updating it
		namespacedName := types.NamespacedName{Namespace: applicationSet.Namespace, Name: applicationSet.Name}
		if err := r.Get(ctx, namespacedName, applicationSet); err != nil {
			if client.IgnoreNotFound(err) != nil {
				return nil
			}
			return fmt.Errorf("error fetching updated application set: %v", err)
		}

		applicationSet.Status.SetConditions(
			newConditions, evaluatedTypes,
		)

		// Update the newly fetched object with new set of conditions
		err := r.Client.Status().Update(ctx, applicationSet)
		if err != nil && !apierr.IsNotFound(err) {
			return fmt.Errorf("unable to set application set condition: %v", err)
		}
	}

	return nil
}

// validateGeneratedApplications uses the Argo CD validation functions to verify the correctness of the
// generated applications.
func (r *ApplicationSetReconciler) validateGeneratedApplications(ctx context.Context, desiredApplications []argov1alpha1.Application, applicationSetInfo argov1alpha1.ApplicationSet, namespace string) (map[int]error, error) {
	errorsByIndex := map[int]error{}
	namesSet := map[string]bool{}
	for i, app := range desiredApplications {

		if !namesSet[app.Name] {
			namesSet[app.Name] = true
		} else {
			errorsByIndex[i] = fmt.Errorf("ApplicationSet %s contains applications with duplicate name: %s", applicationSetInfo.Name, app.Name)
			continue
		}

		proj, err := r.ArgoAppClientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, app.Spec.GetProject(), metav1.GetOptions{})
		if err != nil {
			if apierr.IsNotFound(err) {
				errorsByIndex[i] = fmt.Errorf("application references project %s which does not exist", app.Spec.Project)
				continue
			}
			return nil, err
		}

		if err := utils.ValidateDestination(ctx, &app.Spec.Destination, r.KubeClientset, namespace); err != nil {
			errorsByIndex[i] = fmt.Errorf("application destination spec is invalid: %s", err.Error())
			continue
		}

		conditions, err := argoutil.ValidatePermissions(ctx, &app.Spec, proj, r.ArgoDB)
		if err != nil {
			return nil, err
		}
		if len(conditions) > 0 {
			errorsByIndex[i] = fmt.Errorf("application spec is invalid: %s", argoutil.FormatAppConditions(conditions))
			continue
		}

	}

	return errorsByIndex, nil
}

func (r *ApplicationSetReconciler) getMinRequeueAfter(applicationSetInfo *argov1alpha1.ApplicationSet) time.Duration {
	var res time.Duration
	for _, requestedGenerator := range applicationSetInfo.Spec.Generators {

		relevantGenerators := generators.GetRelevantGenerators(&requestedGenerator, r.Generators)

		for _, g := range relevantGenerators {
			t := g.GetRequeueAfter(&requestedGenerator)

			if res == 0 {
				res = t
			} else if t != 0 && t < res {
				res = t
			}
		}
	}

	return res
}

func getTempApplication(applicationSetTemplate argov1alpha1.ApplicationSetTemplate) *argov1alpha1.Application {
	var tmplApplication argov1alpha1.Application
	tmplApplication.Annotations = applicationSetTemplate.Annotations
	tmplApplication.Labels = applicationSetTemplate.Labels
	tmplApplication.Namespace = applicationSetTemplate.Namespace
	tmplApplication.Name = applicationSetTemplate.Name
	tmplApplication.Spec = applicationSetTemplate.Spec
	tmplApplication.Finalizers = applicationSetTemplate.Finalizers

	return &tmplApplication
}

func (r *ApplicationSetReconciler) generateApplications(applicationSetInfo argov1alpha1.ApplicationSet) ([]argov1alpha1.Application, argov1alpha1.ApplicationSetReasonType, error) {
	var res []argov1alpha1.Application

	var firstError error
	var applicationSetReason argov1alpha1.ApplicationSetReasonType

	for _, requestedGenerator := range applicationSetInfo.Spec.Generators {
		t, err := generators.Transform(requestedGenerator, r.Generators, applicationSetInfo.Spec.Template, &applicationSetInfo, map[string]interface{}{})
		if err != nil {
			log.WithError(err).WithField("generator", requestedGenerator).
				Error("error generating application from params")
			if firstError == nil {
				firstError = err
				applicationSetReason = argov1alpha1.ApplicationSetReasonApplicationParamsGenerationError
			}
			continue
		}

		for _, a := range t {
			tmplApplication := getTempApplication(a.Template)
			if tmplApplication.Labels == nil {
				tmplApplication.Labels = make(map[string]string)
			}
			tmplApplication.Labels[LabelKeyAppSetInstance] = applicationSetInfo.Name

			for _, p := range a.Params {
				app, err := r.Renderer.RenderTemplateParams(tmplApplication, applicationSetInfo.Spec.SyncPolicy, p, applicationSetInfo.Spec.GoTemplate, applicationSetInfo.Spec.GoTemplateOptions)
				if err != nil {
					log.WithError(err).WithField("params", a.Params).WithField("generator", requestedGenerator).
						Error("error generating application from params")

					if firstError == nil {
						firstError = err
						applicationSetReason = argov1alpha1.ApplicationSetReasonRenderTemplateParamsError
					}
					continue
				}
				res = append(res, *app)
			}
		}

		log.WithField("generator", requestedGenerator).Infof("generated %d applications", len(res))
		log.WithField("generator", requestedGenerator).Debugf("apps from generator: %+v", res)
	}

	return res, applicationSetReason, firstError
}

func (r *ApplicationSetReconciler) SetupWithManager(mgr ctrl.Manager, enableProgressiveSyncs bool, maxConcurrentReconciliations int) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &argov1alpha1.Application{}, ".metadata.controller", func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		app := rawObj.(*argov1alpha1.Application)
		owner := metav1.GetControllerOf(app)
		if owner == nil {
			return nil
		}
		// ...make sure it's a application set...
		if owner.APIVersion != argov1alpha1.SchemeGroupVersion.String() || owner.Kind != "ApplicationSet" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return fmt.Errorf("error setting up with manager: %w", err)
	}

	ownsHandler := getOwnsHandlerPredicates(enableProgressiveSyncs)

	return ctrl.NewControllerManagedBy(mgr).WithOptions(controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciliations,
	}).For(&argov1alpha1.ApplicationSet{}).
		Owns(&argov1alpha1.Application{}, builder.WithPredicates(ownsHandler)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&clusterSecretEventHandler{
				Client: mgr.GetClient(),
				Log:    log.WithField("type", "createSecretEventHandler"),
			}).
		// TODO: also watch Applications and respond on changes if we own them.
		Complete(r)
}

// createOrUpdateInCluster will create / update application resources in the cluster.
// - For new applications, it will call create
// - For existing application, it will call update
// The function also adds owner reference to all applications, and uses it to delete them.
func (r *ApplicationSetReconciler) createOrUpdateInCluster(ctx context.Context, applicationSet argov1alpha1.ApplicationSet, desiredApplications []argov1alpha1.Application) error {

	var firstError error
	// Creates or updates the application in appList
	for _, generatedApp := range desiredApplications {

		appLog := log.WithFields(log.Fields{"app": generatedApp.Name, "appSet": applicationSet.Name})
		generatedApp.Namespace = applicationSet.Namespace

		found := &argov1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generatedApp.Name,
				Namespace: generatedApp.Namespace,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
		}

		action, err := utils.CreateOrUpdate(ctx, r.Client, found, func() error {
			// Copy only the Application/ObjectMeta fields that are significant, from the generatedApp
			found.Spec = generatedApp.Spec

			// allow setting the Operation field to trigger a sync operation on an Application
			if generatedApp.Operation != nil {
				found.Operation = generatedApp.Operation
			}

			preservedAnnotations := make([]string, 0)
			if applicationSet.Spec.PreservedFields != nil {
				preservedAnnotations = append(preservedAnnotations, applicationSet.Spec.PreservedFields.Annotations...)
			}
			// Preserve specially treated argo cd annotations:
			// * https://github.com/argoproj/applicationset/issues/180
			// * https://github.com/argoproj/argo-cd/issues/10500
			preservedAnnotations = append(preservedAnnotations, defaultPreservedAnnotations...)

			for _, key := range preservedAnnotations {
				if state, exists := found.ObjectMeta.Annotations[key]; exists {
					if generatedApp.Annotations == nil {
						generatedApp.Annotations = map[string]string{}
					}
					generatedApp.Annotations[key] = state
				}
			}
			found.ObjectMeta.Annotations = generatedApp.Annotations

			found.ObjectMeta.Finalizers = generatedApp.Finalizers
			found.ObjectMeta.Labels = generatedApp.Labels
			return controllerutil.SetControllerReference(&applicationSet, found, r.Scheme)
		})

		if err != nil {
			appLog.WithError(err).WithField("action", action).Errorf("failed to %s Application", action)
			if firstError == nil {
				firstError = err
			}
			continue
		}

		r.Recorder.Eventf(&applicationSet, corev1.EventTypeNormal, fmt.Sprint(action), "%s Application %q", action, generatedApp.Name)
		appLog.Logf(log.InfoLevel, "%s Application", action)
	}
	return firstError
}

// createInCluster will filter from the desiredApplications only the application that needs to be created
// Then it will call createOrUpdateInCluster to do the actual create
func (r *ApplicationSetReconciler) createInCluster(ctx context.Context, applicationSet argov1alpha1.ApplicationSet, desiredApplications []argov1alpha1.Application) error {

	var createApps []argov1alpha1.Application
	current, err := r.getCurrentApplications(ctx, applicationSet)
	if err != nil {
		return fmt.Errorf("error getting current applications: %w", err)
	}

	m := make(map[string]bool) // Will holds the app names that are current in the cluster

	for _, app := range current {
		m[app.Name] = true
	}

	// filter applications that are not in m[string]bool (new to the cluster)
	for _, app := range desiredApplications {
		_, exists := m[app.Name]

		if !exists {
			createApps = append(createApps, app)
		}
	}

	return r.createOrUpdateInCluster(ctx, applicationSet, createApps)
}

func (r *ApplicationSetReconciler) getCurrentApplications(_ context.Context, applicationSet argov1alpha1.ApplicationSet) ([]argov1alpha1.Application, error) {
	// TODO: Should this use the context param?
	var current argov1alpha1.ApplicationList
	err := r.Client.List(context.Background(), &current, client.MatchingFields{".metadata.controller": applicationSet.Name})

	if err != nil {
		return nil, err
	}

	return current.Items, nil
}

// deleteInCluster will delete Applications that are currently on the cluster, but not in appList.
// The function must be called after all generators had been called and generated applications
func (r *ApplicationSetReconciler) deleteInCluster(ctx context.Context, applicationSet argov1alpha1.ApplicationSet, desiredApplications []argov1alpha1.Application) error {
	// settingsMgr := settings.NewSettingsManager(context.TODO(), r.KubeClientset, applicationSet.Namespace)
	// argoDB := db.NewDB(applicationSet.Namespace, settingsMgr, r.KubeClientset)
	// clusterList, err := argoDB.ListClusters(ctx)
	clusterList, err := utils.ListClusters(ctx, r.KubeClientset, applicationSet.Namespace)
	if err != nil {
		return fmt.Errorf("error listing clusters: %w", err)
	}

	// Save current applications to be able to delete the ones that are not in appList
	current, err := r.getCurrentApplications(ctx, applicationSet)
	if err != nil {
		return fmt.Errorf("error getting current applications: %w", err)
	}

	m := make(map[string]bool) // Will holds the app names in appList for the deletion process

	for _, app := range desiredApplications {
		m[app.Name] = true
	}

	// Delete apps that are not in m[string]bool
	var firstError error
	for _, app := range current {
		appLog := log.WithFields(log.Fields{"app": app.Name, "appSet": applicationSet.Name})
		_, exists := m[app.Name]

		if !exists {

			// Removes the Argo CD resources finalizer if the application contains an invalid target (eg missing cluster)
			err := r.removeFinalizerOnInvalidDestination(ctx, applicationSet, &app, clusterList, appLog)
			if err != nil {
				appLog.WithError(err).Error("failed to update Application")
				if firstError != nil {
					firstError = err
				}
				continue
			}

			err = r.Client.Delete(ctx, &app)
			if err != nil {
				appLog.WithError(err).Error("failed to delete Application")
				if firstError != nil {
					firstError = err
				}
				continue
			}
			r.Recorder.Eventf(&applicationSet, corev1.EventTypeNormal, "Deleted", "Deleted Application %q", app.Name)
			appLog.Log(log.InfoLevel, "Deleted application")
		}
	}
	return firstError
}

// removeFinalizerOnInvalidDestination removes the Argo CD resources finalizer if the application contains an invalid target (eg missing cluster)
func (r *ApplicationSetReconciler) removeFinalizerOnInvalidDestination(ctx context.Context, applicationSet argov1alpha1.ApplicationSet, app *argov1alpha1.Application, clusterList *argov1alpha1.ClusterList, appLog *log.Entry) error {

	// Only check if the finalizers need to be removed IF there are finalizers to remove
	if len(app.Finalizers) == 0 {
		return nil
	}

	var validDestination bool

	// Detect if the destination is invalid (name doesn't correspond to a matching cluster)
	if err := utils.ValidateDestination(ctx, &app.Spec.Destination, r.KubeClientset, applicationSet.Namespace); err != nil {
		appLog.Warnf("The destination cluster for %s couldn't be found: %v", app.Name, err)
		validDestination = false
	} else {

		// Detect if the destination's server field does not match an existing cluster

		matchingCluster := false
		for _, cluster := range clusterList.Items {

			// Server fields must match. Note that ValidateDestination ensures that the server field is set, if applicable.
			if app.Spec.Destination.Server != cluster.Server {
				continue
			}

			// The name must match, if it is not empty
			if app.Spec.Destination.Name != "" && cluster.Name != app.Spec.Destination.Name {
				continue
			}

			matchingCluster = true
			break
		}

		if !matchingCluster {
			appLog.Warnf("A match for the destination cluster for %s, by server url, couldn't be found.", app.Name)
		}

		validDestination = matchingCluster
	}
	// If the destination is invalid (for example the cluster is no longer defined), then remove
	// the application finalizers to avoid triggering Argo CD bug #5817
	if !validDestination {

		// Filter out the Argo CD finalizer from the finalizer list
		var newFinalizers []string
		for _, existingFinalizer := range app.Finalizers {
			if existingFinalizer != argov1alpha1.ResourcesFinalizerName { // only remove this one
				newFinalizers = append(newFinalizers, existingFinalizer)
			}
		}

		// If the finalizer length changed (due to filtering out an Argo finalizer), update the finalizer list on the app
		if len(newFinalizers) != len(app.Finalizers) {
			app.Finalizers = newFinalizers

			r.Recorder.Eventf(&applicationSet, corev1.EventTypeNormal, "Updated", "Updated Application %q finalizer before deletion, because application has an invalid destination", app.Name)
			appLog.Log(log.InfoLevel, "Updating application finalizer before deletion, because application has an invalid destination")

			err := r.Client.Update(ctx, app, &client.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("error updating finalizers: %w", err)
			}
		}
	}

	return nil
}

func (r *ApplicationSetReconciler) performProgressiveSyncs(ctx context.Context, appset argov1alpha1.ApplicationSet, applications []argov1alpha1.Application, desiredApplications []argov1alpha1.Application, appMap map[string]argov1alpha1.Application) (map[string]bool, error) {

	appDependencyList, appStepMap, err := r.buildAppDependencyList(ctx, appset, desiredApplications)
	if err != nil {
		return nil, fmt.Errorf("failed to build app dependency list: %w", err)
	}

	_, err = r.updateApplicationSetApplicationStatus(ctx, &appset, applications, appStepMap)
	if err != nil {
		return nil, fmt.Errorf("failed to update applicationset app status: %w", err)
	}

	log.Infof("ApplicationSet %v step list:", appset.Name)
	for i, step := range appDependencyList {
		log.Infof("step %v: %+v", i+1, step)
	}

	appSyncMap, err := r.buildAppSyncMap(ctx, appset, appDependencyList, appMap)
	if err != nil {
		return nil, fmt.Errorf("failed to build app sync map: %w", err)
	}

	log.Infof("Application allowed to sync before maxUpdate?: %+v", appSyncMap)

	_, err = r.updateApplicationSetApplicationStatusProgress(ctx, &appset, appSyncMap, appStepMap, appMap)
	if err != nil {
		return nil, fmt.Errorf("failed to update applicationset application status progress: %w", err)
	}

	_, err = r.updateApplicationSetApplicationStatusConditions(ctx, &appset)
	if err != nil {
		return nil, fmt.Errorf("failed to update applicationset application status conditions: %w", err)
	}

	return appSyncMap, nil
}

// this list tracks which Applications belong to each RollingUpdate step
func (r *ApplicationSetReconciler) buildAppDependencyList(ctx context.Context, applicationSet argov1alpha1.ApplicationSet, applications []argov1alpha1.Application) ([][]string, map[string]int, error) {

	if applicationSet.Spec.Strategy == nil || applicationSet.Spec.Strategy.Type == "" || applicationSet.Spec.Strategy.Type == "AllAtOnce" {
		return [][]string{}, map[string]int{}, nil
	}

	steps := []argov1alpha1.ApplicationSetRolloutStep{}
	if progressiveSyncsStrategyEnabled(&applicationSet, "RollingSync") {
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
					valueMatched := labelMatchedExpression(val, matchExpression)

					if !valueMatched { // none of the matchExpression values was a match with the Application'ss labels
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
					log.Warnf("AppSet '%v' has a invalid matchExpression that selects Application '%v' label twice, in steps %v and %v", applicationSet.Name, app.Name, val+1, i+1)
				} else {
					appStepMap[app.Name] = i
				}
			}
		}
	}

	return appDependencyList, appStepMap, nil
}

func labelMatchedExpression(val string, matchExpression argov1alpha1.ApplicationMatchExpression) bool {
	if matchExpression.Operator != "In" && matchExpression.Operator != "NotIn" {
		log.Errorf("skipping AppSet rollingUpdate step Application selection, invalid matchExpression operator provided: %q ", matchExpression.Operator)
		return false
	}

	// if operator == In, default to false
	// if operator == NotIn, default to true
	valueMatched := matchExpression.Operator == "NotIn"

	for _, value := range matchExpression.Values {
		if val == value {
			// first "In" match returns true
			// first "NotIn" match returns false
			return matchExpression.Operator == "In"
		}
	}
	return valueMatched
}

// this map is used to determine which stage of Applications are ready to be updated in the reconciler loop
func (r *ApplicationSetReconciler) buildAppSyncMap(ctx context.Context, applicationSet argov1alpha1.ApplicationSet, appDependencyList [][]string, appMap map[string]argov1alpha1.Application) (map[string]bool, error) {
	appSyncMap := map[string]bool{}
	syncEnabled := true

	// healthy stages and the first non-healthy stage should have sync enabled
	// every stage after should have sync disabled

	for i := range appDependencyList {
		// set the syncEnabled boolean for every Application in the current step
		for _, appName := range appDependencyList[i] {
			appSyncMap[appName] = syncEnabled
		}

		// detect if we need to halt before progressing to the next step
		for _, appName := range appDependencyList[i] {

			idx := findApplicationStatusIndex(applicationSet.Status.ApplicationStatus, appName)
			if idx == -1 {
				// no Application status found, likely because the Application is being newly created
				syncEnabled = false
				break
			}

			appStatus := applicationSet.Status.ApplicationStatus[idx]

			if app, ok := appMap[appName]; ok {

				syncEnabled = appSyncEnabledForNextStep(&applicationSet, app, appStatus)
				if !syncEnabled {
					break
				}
			} else {
				// application name not found in the list of applications managed by this ApplicationSet, maybe because it's being deleted
				syncEnabled = false
				break
			}
		}
	}

	return appSyncMap, nil
}

func appSyncEnabledForNextStep(appset *argov1alpha1.ApplicationSet, app argov1alpha1.Application, appStatus argov1alpha1.ApplicationSetApplicationStatus) bool {

	if progressiveSyncsStrategyEnabled(appset, "RollingSync") {
		// we still need to complete the current step if the Application is not yet Healthy or there are still pending Application changes
		return isApplicationHealthy(app) && appStatus.Status == "Healthy"
	}

	return true
}

func progressiveSyncsStrategyEnabled(appset *argov1alpha1.ApplicationSet, strategyType string) bool {
	if appset.Spec.Strategy == nil || appset.Spec.Strategy.Type != strategyType {
		return false
	}

	if strategyType == "RollingSync" && appset.Spec.Strategy.RollingSync == nil {
		return false
	}

	return true
}

func isApplicationHealthy(app argov1alpha1.Application) bool {
	healthStatusString, syncStatusString, operationPhaseString := statusStrings(app)

	if healthStatusString == "Healthy" && syncStatusString != "OutOfSync" && (operationPhaseString == "Succeeded" || operationPhaseString == "") {
		return true
	}
	return false
}

func statusStrings(app argov1alpha1.Application) (string, string, string) {
	healthStatusString := string(app.Status.Health.Status)
	syncStatusString := string(app.Status.Sync.Status)
	operationPhaseString := ""
	if app.Status.OperationState != nil {
		operationPhaseString = string(app.Status.OperationState.Phase)
	}

	return healthStatusString, syncStatusString, operationPhaseString
}

// check the status of each Application's status and promote Applications to the next status if needed
func (r *ApplicationSetReconciler) updateApplicationSetApplicationStatus(ctx context.Context, applicationSet *argov1alpha1.ApplicationSet, applications []argov1alpha1.Application, appStepMap map[string]int) ([]argov1alpha1.ApplicationSetApplicationStatus, error) {

	now := metav1.Now()
	appStatuses := make([]argov1alpha1.ApplicationSetApplicationStatus, 0, len(applications))

	for _, app := range applications {

		healthStatusString, syncStatusString, operationPhaseString := statusStrings(app)

		idx := findApplicationStatusIndex(applicationSet.Status.ApplicationStatus, app.Name)

		currentAppStatus := argov1alpha1.ApplicationSetApplicationStatus{}

		if idx == -1 {
			// AppStatus not found, set default status of "Waiting"
			currentAppStatus = argov1alpha1.ApplicationSetApplicationStatus{
				Application:        app.Name,
				LastTransitionTime: &now,
				Message:            "No Application status found, defaulting status to Waiting.",
				Status:             "Waiting",
				Step:               fmt.Sprint(appStepMap[app.Name] + 1),
			}
		} else {
			// we have an existing AppStatus
			currentAppStatus = applicationSet.Status.ApplicationStatus[idx]
		}

		appOutdated := false
		if progressiveSyncsStrategyEnabled(applicationSet, "RollingSync") {
			appOutdated = syncStatusString == "OutOfSync"
		}

		if appOutdated && currentAppStatus.Status != "Waiting" && currentAppStatus.Status != "Pending" {
			log.Infof("Application %v is outdated, updating its ApplicationSet status to Waiting", app.Name)
			currentAppStatus.LastTransitionTime = &now
			currentAppStatus.Status = "Waiting"
			currentAppStatus.Message = "Application has pending changes, setting status to Waiting."
			currentAppStatus.Step = fmt.Sprint(appStepMap[currentAppStatus.Application] + 1)
		}

		if currentAppStatus.Status == "Pending" {
			if operationPhaseString == "Succeeded" && app.Status.OperationState.StartedAt.After(currentAppStatus.LastTransitionTime.Time) {
				log.Infof("Application %v has completed a sync successfully, updating its ApplicationSet status to Progressing", app.Name)
				currentAppStatus.LastTransitionTime = &now
				currentAppStatus.Status = "Progressing"
				currentAppStatus.Message = "Application resource completed a sync successfully, updating status from Pending to Progressing."
				currentAppStatus.Step = fmt.Sprint(appStepMap[currentAppStatus.Application] + 1)
			} else if operationPhaseString == "Running" || healthStatusString == "Progressing" {
				log.Infof("Application %v has entered Progressing status, updating its ApplicationSet status to Progressing", app.Name)
				currentAppStatus.LastTransitionTime = &now
				currentAppStatus.Status = "Progressing"
				currentAppStatus.Message = "Application resource became Progressing, updating status from Pending to Progressing."
				currentAppStatus.Step = fmt.Sprint(appStepMap[currentAppStatus.Application] + 1)
			}
		}

		if currentAppStatus.Status == "Waiting" && isApplicationHealthy(app) {
			log.Infof("Application %v is already synced and healthy, updating its ApplicationSet status to Healthy", app.Name)
			currentAppStatus.LastTransitionTime = &now
			currentAppStatus.Status = healthStatusString
			currentAppStatus.Message = "Application resource is already Healthy, updating status from Waiting to Healthy."
			currentAppStatus.Step = fmt.Sprint(appStepMap[currentAppStatus.Application] + 1)
		}

		if currentAppStatus.Status == "Progressing" && isApplicationHealthy(app) {
			log.Infof("Application %v has completed Progressing status, updating its ApplicationSet status to Healthy", app.Name)
			currentAppStatus.LastTransitionTime = &now
			currentAppStatus.Status = healthStatusString
			currentAppStatus.Message = "Application resource became Healthy, updating status from Progressing to Healthy."
			currentAppStatus.Step = fmt.Sprint(appStepMap[currentAppStatus.Application] + 1)
		}

		appStatuses = append(appStatuses, currentAppStatus)
	}

	err := r.setAppSetApplicationStatus(ctx, applicationSet, appStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to set AppSet application statuses: %w", err)
	}

	return appStatuses, nil
}

// check Applications that are in Waiting status and promote them to Pending if needed
func (r *ApplicationSetReconciler) updateApplicationSetApplicationStatusProgress(ctx context.Context, applicationSet *argov1alpha1.ApplicationSet, appSyncMap map[string]bool, appStepMap map[string]int, appMap map[string]argov1alpha1.Application) ([]argov1alpha1.ApplicationSetApplicationStatus, error) {
	now := metav1.Now()

	appStatuses := make([]argov1alpha1.ApplicationSetApplicationStatus, 0, len(applicationSet.Status.ApplicationStatus))

	// if we have no RollingUpdate steps, clear out the existing ApplicationStatus entries
	if applicationSet.Spec.Strategy != nil && applicationSet.Spec.Strategy.Type != "" && applicationSet.Spec.Strategy.Type != "AllAtOnce" {
		updateCountMap := []int{}
		totalCountMap := []int{}

		length := 0
		if progressiveSyncsStrategyEnabled(applicationSet, "RollingSync") {
			length = len(applicationSet.Spec.Strategy.RollingSync.Steps)
		}
		for s := 0; s < length; s++ {
			updateCountMap = append(updateCountMap, 0)
			totalCountMap = append(totalCountMap, 0)
		}

		// populate updateCountMap with counts of existing Pending and Progressing Applications
		for _, appStatus := range applicationSet.Status.ApplicationStatus {
			totalCountMap[appStepMap[appStatus.Application]] += 1

			if progressiveSyncsStrategyEnabled(applicationSet, "RollingSync") {
				if appStatus.Status == "Pending" || appStatus.Status == "Progressing" {
					updateCountMap[appStepMap[appStatus.Application]] += 1
				}
			}
		}

		for _, appStatus := range applicationSet.Status.ApplicationStatus {

			maxUpdateAllowed := true
			maxUpdate := &intstr.IntOrString{}
			if progressiveSyncsStrategyEnabled(applicationSet, "RollingSync") {
				maxUpdate = applicationSet.Spec.Strategy.RollingSync.Steps[appStepMap[appStatus.Application]].MaxUpdate
			}

			// by default allow all applications to update if maxUpdate is unset
			if maxUpdate != nil {
				maxUpdateVal, err := intstr.GetScaledValueFromIntOrPercent(maxUpdate, totalCountMap[appStepMap[appStatus.Application]], false)
				if err != nil {
					log.Warnf("AppSet '%v' has a invalid maxUpdate value '%+v', ignoring maxUpdate logic for this step: %v", applicationSet.Name, maxUpdate, err)
				}

				// ensure that percentage values greater than 0% always result in at least 1 Application being selected
				if maxUpdate.Type == intstr.String && maxUpdate.StrVal != "0%" && maxUpdateVal < 1 {
					maxUpdateVal = 1
				}

				if updateCountMap[appStepMap[appStatus.Application]] >= maxUpdateVal {
					maxUpdateAllowed = false
					log.Infof("Application %v is not allowed to update yet, %v/%v Applications already updating in step %v in AppSet %v", appStatus.Application, updateCountMap[appStepMap[appStatus.Application]], maxUpdateVal, appStepMap[appStatus.Application]+1, applicationSet.Name)
				}

			}

			if appStatus.Status == "Waiting" && appSyncMap[appStatus.Application] && maxUpdateAllowed {
				log.Infof("Application %v moved to Pending status, watching for the Application to start Progressing", appStatus.Application)
				appStatus.LastTransitionTime = &now
				appStatus.Status = "Pending"
				appStatus.Message = "Application moved to Pending status, watching for the Application resource to start Progressing."
				appStatus.Step = fmt.Sprint(appStepMap[appStatus.Application] + 1)

				updateCountMap[appStepMap[appStatus.Application]] += 1
			}

			appStatuses = append(appStatuses, appStatus)
		}
	}

	err := r.setAppSetApplicationStatus(ctx, applicationSet, appStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to set AppSet app status: %w", err)
	}

	return appStatuses, nil
}

func (r *ApplicationSetReconciler) updateApplicationSetApplicationStatusConditions(ctx context.Context, applicationSet *argov1alpha1.ApplicationSet) ([]argov1alpha1.ApplicationSetCondition, error) {

	appSetProgressing := false
	for _, appStatus := range applicationSet.Status.ApplicationStatus {
		if appStatus.Status != "Healthy" {
			appSetProgressing = true
			break
		}
	}

	appSetConditionProgressing := false
	for _, appSetCondition := range applicationSet.Status.Conditions {
		if appSetCondition.Type == argov1alpha1.ApplicationSetConditionRolloutProgressing && appSetCondition.Status == argov1alpha1.ApplicationSetConditionStatusTrue {
			appSetConditionProgressing = true
			break
		}
	}

	if appSetProgressing && !appSetConditionProgressing {
		_ = r.setApplicationSetStatusCondition(ctx,
			applicationSet,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionRolloutProgressing,
				Message: "ApplicationSet Rollout Rollout started",
				Reason:  argov1alpha1.ApplicationSetReasonApplicationSetModified,
				Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
			}, false,
		)
	} else if !appSetProgressing && appSetConditionProgressing {
		_ = r.setApplicationSetStatusCondition(ctx,
			applicationSet,
			argov1alpha1.ApplicationSetCondition{
				Type:    argov1alpha1.ApplicationSetConditionRolloutProgressing,
				Message: "ApplicationSet Rollout Rollout complete",
				Reason:  argov1alpha1.ApplicationSetReasonApplicationSetRolloutComplete,
				Status:  argov1alpha1.ApplicationSetConditionStatusFalse,
			}, false,
		)
	}

	return applicationSet.Status.Conditions, nil
}

func findApplicationStatusIndex(appStatuses []argov1alpha1.ApplicationSetApplicationStatus, application string) int {
	for i := range appStatuses {
		if appStatuses[i].Application == application {
			return i
		}
	}
	return -1
}

// setApplicationSetApplicationStatus updates the ApplicatonSet's status field
// with any new/changed Application statuses.
func (r *ApplicationSetReconciler) setAppSetApplicationStatus(ctx context.Context, applicationSet *argov1alpha1.ApplicationSet, applicationStatuses []argov1alpha1.ApplicationSetApplicationStatus) error {
	needToUpdateStatus := false

	if len(applicationStatuses) != len(applicationSet.Status.ApplicationStatus) {
		needToUpdateStatus = true
	} else {
		for i := range applicationStatuses {
			appStatus := applicationStatuses[i]
			idx := findApplicationStatusIndex(applicationSet.Status.ApplicationStatus, appStatus.Application)
			if idx == -1 {
				needToUpdateStatus = true
				break
			}
			currentStatus := applicationSet.Status.ApplicationStatus[idx]
			if currentStatus.Message != appStatus.Message || currentStatus.Status != appStatus.Status || currentStatus.Step != appStatus.Step {
				needToUpdateStatus = true
				break
			}
		}
	}

	if needToUpdateStatus {
		namespacedName := types.NamespacedName{Namespace: applicationSet.Namespace, Name: applicationSet.Name}

		// rebuild ApplicationStatus from scratch, we don't need any previous status history
		applicationSet.Status.ApplicationStatus = []argov1alpha1.ApplicationSetApplicationStatus{}
		for i := range applicationStatuses {
			applicationSet.Status.SetApplicationStatus(applicationStatuses[i])
		}

		// Update the newly fetched object with new set of ApplicationStatus
		err := r.Client.Status().Update(ctx, applicationSet)
		if err != nil {

			log.Errorf("unable to set application set status: %v", err)
			return fmt.Errorf("unable to set application set status: %v", err)
		}

		if err := r.Get(ctx, namespacedName, applicationSet); err != nil {
			if client.IgnoreNotFound(err) != nil {
				return nil
			}
			return fmt.Errorf("error fetching updated application set: %v", err)
		}
	}

	return nil
}

func (r *ApplicationSetReconciler) syncValidApplications(ctx context.Context, applicationSet *argov1alpha1.ApplicationSet, appSyncMap map[string]bool, appMap map[string]argov1alpha1.Application, validApps []argov1alpha1.Application) ([]argov1alpha1.Application, error) {
	rolloutApps := []argov1alpha1.Application{}
	for i := range validApps {
		pruneEnabled := false

		// ensure that Applications generated with RollingSync do not have an automated sync policy, since the AppSet controller will handle triggering the sync operation instead
		if validApps[i].Spec.SyncPolicy != nil && validApps[i].Spec.SyncPolicy.Automated != nil {
			pruneEnabled = validApps[i].Spec.SyncPolicy.Automated.Prune
			validApps[i].Spec.SyncPolicy.Automated = nil
		}

		appSetStatusPending := false
		idx := findApplicationStatusIndex(applicationSet.Status.ApplicationStatus, validApps[i].Name)
		if idx > -1 && applicationSet.Status.ApplicationStatus[idx].Status == "Pending" {
			// only trigger a sync for Applications that are in Pending status, since this is governed by maxUpdate
			appSetStatusPending = true
		}

		// check appSyncMap to determine which Applications are ready to be updated and which should be skipped
		if appSyncMap[validApps[i].Name] && appMap[validApps[i].Name].Status.Sync.Status == "OutOfSync" && appSetStatusPending {
			log.Infof("triggering sync for application: %v, prune enabled: %v", validApps[i].Name, pruneEnabled)
			validApps[i], _ = syncApplication(validApps[i], pruneEnabled)
		}
		rolloutApps = append(rolloutApps, validApps[i])
	}
	return rolloutApps, nil
}

// used by the RollingSync Progressive Sync strategy to trigger a sync of a particular Application resource
func syncApplication(application argov1alpha1.Application, prune bool) (argov1alpha1.Application, error) {

	operation := argov1alpha1.Operation{
		InitiatedBy: argov1alpha1.OperationInitiator{
			Username:  "applicationset-controller",
			Automated: true,
		},
		Info: []*argov1alpha1.Info{
			{
				Name:  "Reason",
				Value: "ApplicationSet RollingSync triggered a sync of this Application resource.",
			},
		},
		Sync: &argov1alpha1.SyncOperation{},
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

	return application, nil
}

func getOwnsHandlerPredicates(enableProgressiveSyncs bool) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// if we are the owner and there is a create event, we most likely created it and do not need to
			// re-reconcile
			log.Debugln("received create event from owning an application")
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log.Debugln("received delete event from owning an application")
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log.Debugln("received update event from owning an application")
			appOld, isApp := e.ObjectOld.(*argov1alpha1.Application)
			if !isApp {
				return false
			}
			appNew, isApp := e.ObjectNew.(*argov1alpha1.Application)
			if !isApp {
				return false
			}
			requeue := shouldRequeueApplicationSet(appOld, appNew, enableProgressiveSyncs)
			log.Debugf("requeue: %t caused by application %s\n", requeue, appNew.Name)
			return requeue
		},
		GenericFunc: func(e event.GenericEvent) bool {
			log.Debugln("received generic event from owning an application")
			return true
		},
	}
}

// shouldRequeueApplicationSet determines when we want to requeue an ApplicationSet for reconciling based on an owned
// application change
// The applicationset controller owns a subset of the Application CR.
// We do not need to re-reconcile if parts of the application change outside the applicationset's control.
// An example being, Application.ApplicationStatus.ReconciledAt which gets updated by the application controller.
// Additionally, Application.ObjectMeta.ResourceVersion and Application.ObjectMeta.Generation which are set by K8s.
func shouldRequeueApplicationSet(appOld *argov1alpha1.Application, appNew *argov1alpha1.Application, enableProgressiveSyncs bool) bool {
	if appOld == nil || appNew == nil {
		return false
	}

	// the applicationset controller owns the application spec, labels, annotations, and finalizers on the applications
	if !reflect.DeepEqual(appOld.Spec, appNew.Spec) ||
		!reflect.DeepEqual(appOld.ObjectMeta.GetAnnotations(), appNew.ObjectMeta.GetAnnotations()) ||
		!reflect.DeepEqual(appOld.ObjectMeta.GetLabels(), appNew.ObjectMeta.GetLabels()) ||
		!reflect.DeepEqual(appOld.ObjectMeta.GetFinalizers(), appNew.ObjectMeta.GetFinalizers()) {
		return true
	}

	// progressive syncs use the application status for updates. if they differ, requeue to trigger the next progression
	if enableProgressiveSyncs {
		if appOld.Status.Health.Status != appNew.Status.Health.Status || appOld.Status.Sync.Status != appNew.Status.Sync.Status {
			return true
		}

		if appOld.Status.OperationState != nil && appNew.Status.OperationState != nil {
			if appOld.Status.OperationState.Phase != appNew.Status.OperationState.Phase ||
				appOld.Status.OperationState.StartedAt != appNew.Status.OperationState.StartedAt {
				return true
			}
		}
	}

	return false
}

var _ handler.EventHandler = &clusterSecretEventHandler{}

package applicationset

import (
	"context"
	"errors"
	"fmt"
	appsettemplate "github.com/argoproj/argo-cd/v3/applicationset/controllers/template"
	"github.com/argoproj/argo-cd/v3/applicationset/generators"
	"github.com/argoproj/argo-cd/v3/applicationset/services"
	appsetutils "github.com/argoproj/argo-cd/v3/applicationset/utils"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/server/application"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/collections"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/github_app"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/argo-cd/v3/util/session"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
	"time"
)

// generateApplicationSetApps generates the list of Argo CD Applications for a given ApplicationSet.
// It uses the configured generators and SCM services to render the templates.
func (s *Server) generateApplicationSetApps(ctx context.Context, logEntry *log.Entry, appset v1alpha1.ApplicationSet) ([]v1alpha1.Application, error) {
	argoCDDB := s.db

	scmConfig := generators.NewSCMConfig(s.ScmRootCAPath, s.AllowedScmProviders, s.EnableScmProviders, s.EnableGitHubAPIMetrics, github_app.NewAuthCredentials(argoCDDB.(db.RepoCredsDB)), true)
	argoCDService := services.NewArgoCDService(s.db, s.GitSubmoduleEnabled, s.repoClientSet, s.EnableNewGitFileGlobbing)
	appSetGenerators := generators.GetGenerators(ctx, s.client, s.k8sClient, s.ns, argoCDService, s.dynamicClient, scmConfig)

	apps, _, err := appsettemplate.GenerateApplications(logEntry, appset, appSetGenerators, &appsetutils.Render{}, s.client)
	if err != nil {
		return nil, fmt.Errorf("error generating applications: %w", err)
	}
	return apps, nil
}

// updateAppSet updates the given ApplicationSet resource in the cluster.
// If merge is true, labels and annotations are merged; otherwise, they are replaced.
// Handles conflict retries automatically up to 10 attempts.
// Also enforces RBAC checks if the project is changed.
func (s *Server) updateAppSet(ctx context.Context, appset *v1alpha1.ApplicationSet, newAppset *v1alpha1.ApplicationSet, merge bool) (*v1alpha1.ApplicationSet, error) {
	if appset != nil && appset.Spec.Template.Spec.Project != newAppset.Spec.Template.Spec.Project {
		// When changing projects, caller must have applicationset create and update privileges in new project
		// NOTE: the update check was already verified in the caller to this function
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionCreate, newAppset.RBACName(s.ns)); err != nil {
			return nil, err
		}
		// They also need 'update' privileges in the old project
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionUpdate, appset.RBACName(s.ns)); err != nil {
			return nil, err
		}
	}

	for i := 0; i < 10; i++ {
		appset.Spec = newAppset.Spec
		if merge {
			appset.Labels = collections.Merge(appset.Labels, newAppset.Labels)
			appset.Annotations = collections.Merge(appset.Annotations, newAppset.Annotations)
		} else {
			appset.Labels = newAppset.Labels
			appset.Annotations = newAppset.Annotations
		}
		appset.Finalizers = newAppset.Finalizers
		res, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(appset.Namespace).Update(ctx, appset, metav1.UpdateOptions{})
		if err == nil {
			s.logAppSetEvent(ctx, appset, argo.EventReasonResourceUpdated, "updated ApplicationSets spec")
			s.waitSync(res)
			return res, nil
		}
		if !apierrors.IsConflict(err) {
			return nil, err
		}

		appset, err = s.appclientset.ArgoprojV1alpha1().ApplicationSets(appset.Namespace).Get(ctx, appset.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting ApplicationSets: %w", err)
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to update ApplicationSets. Too many conflicts")
}

// buildApplicationSetTree constructs a hierarchical tree structure of the ApplicationSet and its Applications.
func (s *Server) buildApplicationSetTree(a *v1alpha1.ApplicationSet) (*v1alpha1.ApplicationSetTree, error) {
	var tree v1alpha1.ApplicationSetTree

	gvk := v1alpha1.ApplicationSetSchemaGroupVersionKind
	parentRefs := []v1alpha1.ResourceRef{
		{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind, Name: a.Name, Namespace: a.Namespace, UID: string(a.UID)},
	}

	apps := a.Status.Resources
	for _, app := range apps {
		tree.Nodes = append(tree.Nodes, v1alpha1.ResourceNode{
			Health: app.Health,
			ResourceRef: v1alpha1.ResourceRef{
				Name:      app.Name,
				Group:     app.Group,
				Version:   app.Version,
				Kind:      app.Kind,
				Namespace: a.Namespace,
			},
			ParentRefs: parentRefs,
		})
	}
	tree.Normalize()

	return &tree, nil
}

// validateAppSet validates the ApplicationSet resource.
// Returns the project name if valid. Returns an error if the project is templated or generators are invalid.
func (s *Server) validateAppSet(appset *v1alpha1.ApplicationSet) (string, error) {
	if appset == nil {
		return "", errors.New("ApplicationSet cannot be validated for nil value")
	}

	projectName := appset.Spec.Template.Spec.Project

	if strings.Contains(projectName, "{{") {
		return "", errors.New("the Argo CD API does not currently support creating ApplicationSets with templated `project` fields")
	}

	if err := appsetutils.CheckInvalidGenerators(appset); err != nil {
		return "", err
	}

	return projectName, nil
}

// checkCreatePermissions verifies that the caller has permission to create an ApplicationSet
// and that the referenced project exists.
func (s *Server) checkCreatePermissions(ctx context.Context, appset *v1alpha1.ApplicationSet, projectName string) error {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionCreate, appset.RBACName(s.ns)); err != nil {
		return err
	}

	_, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, projectName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "ApplicationSet references project %s which does not exist", projectName)
		}
		return fmt.Errorf("error getting ApplicationSet's project %q: %w", projectName, err)
	}

	return nil
}

// waitSync is a helper to wait until the application informer cache is synced after create/update.
// It waits until the app in the informer, has a resource version greater than the version in the
// supplied app, or after 2 seconds, whichever comes first. Returns true if synced.
// We use an informer cache for read operations (Get, List). Since the cache is only
// eventually consistent, it is possible that it doesn't reflect an application change immediately
// after a mutating API call (create/update). This function should be called after a creates &
// update to give a probable (but not guaranteed) chance of being up-to-date after the create/update.
func (s *Server) waitSync(appset *v1alpha1.ApplicationSet) {
	logCtx := log.WithField("applicationset", appset.Name)
	deadline := time.Now().Add(application.InformerSyncTimeout)
	minVersion, err := strconv.Atoi(appset.ResourceVersion)
	if err != nil {
		logCtx.Warnf("waitSync failed: could not parse resource version %s", appset.ResourceVersion)
		time.Sleep(50 * time.Millisecond) // sleep anyway
		return
	}
	for {
		if currAppset, err := s.appsetLister.ApplicationSets(appset.Namespace).Get(appset.Name); err == nil {
			currVersion, err := strconv.Atoi(currAppset.ResourceVersion)
			if err == nil && currVersion >= minVersion {
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	logCtx.Warnf("waitSync failed: timed out")
}

// logAppSetEvent logs an event for the ApplicationSet, associating it with the user from the context.
func (s *Server) logAppSetEvent(ctx context.Context, a *v1alpha1.ApplicationSet, reason string, action string) {
	eventInfo := argo.EventInfo{Type: corev1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppSetEvent(a, eventInfo, message, user)
}

// appsetNamespaceOrDefault returns the provided namespace or defaults to the server's configured namespace if empty.
func (s *Server) appsetNamespaceOrDefault(appNs string) string {
	if appNs == "" {
		return s.ns
	}
	return appNs
}

// isNamespaceEnabled returns true if the given namespace is enabled for this server context.
func (s *Server) isNamespaceEnabled(namespace string) bool {
	return security.IsNamespaceEnabled(namespace, s.ns, s.enabledNamespaces)
}

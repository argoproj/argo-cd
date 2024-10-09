package project

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/sync"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	listersv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/deeplinks"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	jwtutil "github.com/argoproj/argo-cd/v2/util/jwt"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/session"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	// JWTTokenSubFormat format of the JWT token subject that Argo CD vends out.
	JWTTokenSubFormat = "proj:%s:%s"
)

// Server provides a Project service
type Server struct {
	ns            string
	enf           *rbac.Enforcer
	policyEnf     *rbacpolicy.RBACPolicyEnforcer
	appclientset  appclientset.Interface
	kubeclientset kubernetes.Interface
	auditLogger   *argo.AuditLogger
	projectLock   sync.KeyLock
	sessionMgr    *session.SessionManager
	projInformer  cache.SharedIndexInformer
	settingsMgr   *settings.SettingsManager
	db            db.ArgoDB
}

// NewServer returns a new instance of the Project service
func NewServer(ns string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface, enf *rbac.Enforcer, projectLock sync.KeyLock, sessionMgr *session.SessionManager, policyEnf *rbacpolicy.RBACPolicyEnforcer,
	projInformer cache.SharedIndexInformer, settingsMgr *settings.SettingsManager, db db.ArgoDB, enableK8sEvent []string,
) *Server {
	auditLogger := argo.NewAuditLogger(ns, kubeclientset, "argocd-server", enableK8sEvent)
	return &Server{
		enf: enf, policyEnf: policyEnf, appclientset: appclientset, kubeclientset: kubeclientset, ns: ns, projectLock: projectLock, auditLogger: auditLogger, sessionMgr: sessionMgr,
		projInformer: projInformer, settingsMgr: settingsMgr, db: db,
	}
}

func validateProject(proj *v1alpha1.AppProject) error {
	err := proj.ValidateProject()
	if err != nil {
		return err
	}
	err = rbac.ValidatePolicy(proj.ProjectPoliciesString())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "policy syntax error: %s", err.Error())
	}
	return nil
}

// CreateToken creates a new token to access a project
func (s *Server) CreateToken(ctx context.Context, q *project.ProjectTokenCreateRequest) (*project.ProjectTokenResponse, error) {
	var resp *project.ProjectTokenResponse
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var createErr error
		resp, createErr = s.createToken(ctx, q)
		return createErr
	})
	return resp, err
}

func (s *Server) createToken(ctx context.Context, q *project.ProjectTokenCreateRequest) (*project.ProjectTokenResponse, error) {
	prj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Project, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = validateProject(prj)
	if err != nil {
		return nil, fmt.Errorf("error validating project: %w", err)
	}

	s.projectLock.Lock(q.Project)
	defer s.projectLock.Unlock(q.Project)

	role, _, err := prj.GetRoleByName(q.Role)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "project '%s' does not have role '%s'", q.Project, q.Role)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project); err != nil {
		if !jwtutil.IsMember(jwtutil.Claims(ctx.Value("claims")), role.Groups, s.policyEnf.GetScopes()) {
			return nil, err
		}
	}
	id := q.Id
	if err := prj.ValidateJWTTokenID(q.Role, q.Id); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if id == "" {
		uniqueId, _ := uuid.NewRandom()
		id = uniqueId.String()
	}
	subject := fmt.Sprintf(JWTTokenSubFormat, q.Project, q.Role)
	jwtToken, err := s.sessionMgr.Create(subject, q.ExpiresIn, id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.RegisteredClaims{}
	_, _, err = parser.ParseUnverified(jwtToken, &claims)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	var issuedAt, expiresAt int64
	if claims.IssuedAt != nil {
		issuedAt = claims.IssuedAt.Unix()
	}
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Unix()
	}
	id = claims.ID

	prj.NormalizeJWTTokens()

	items := append(prj.Status.JWTTokensByRole[q.Role].Items, v1alpha1.JWTToken{IssuedAt: issuedAt, ExpiresAt: expiresAt, ID: id})
	if _, found := prj.Status.JWTTokensByRole[q.Role]; found {
		prj.Status.JWTTokensByRole[q.Role] = v1alpha1.JWTTokens{Items: items}
	} else {
		tokensMap := make(map[string]v1alpha1.JWTTokens)
		tokensMap[q.Role] = v1alpha1.JWTTokens{Items: items}
		prj.Status.JWTTokensByRole = tokensMap
	}

	prj.NormalizeJWTTokens()

	_, err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(ctx, prj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	s.logEvent(prj, ctx, argo.EventReasonResourceCreated, "created token")
	return &project.ProjectTokenResponse{Token: jwtToken}, nil
}

func (s *Server) ListLinks(ctx context.Context, q *project.ListProjectLinksRequest) (*application.LinksResponse, error) {
	projName := q.GetName()

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, projName); err != nil {
		log.WithFields(map[string]interface{}{
			"project": projName,
		}).Warnf("unauthorized access to project, error=%v", err.Error())
		return nil, fmt.Errorf("unauthorized access to project %v", projName)
	}

	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, projName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// sanitize project jwt tokens
	proj.Status = v1alpha1.AppProjectStatus{}

	obj, err := kube.ToUnstructured(proj)
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}

	deepLinks, err := s.settingsMgr.GetDeepLinks(settings.ProjectDeepLinks)
	if err != nil {
		return nil, fmt.Errorf("failed to read application deep links from configmap: %w", err)
	}

	deeplinksObj := deeplinks.CreateDeepLinksObject(nil, nil, nil, obj)
	finalList, errorList := deeplinks.EvaluateDeepLinksResponse(deeplinksObj, obj.GetName(), deepLinks)
	if len(errorList) > 0 {
		log.Errorf("errorList while evaluating project deep links, %v", strings.Join(errorList, ", "))
	}

	return finalList, nil
}

// DeleteToken deletes a token in a project
func (s *Server) DeleteToken(ctx context.Context, q *project.ProjectTokenDeleteRequest) (*project.EmptyResponse, error) {
	var resp *project.EmptyResponse
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var deleteErr error
		resp, deleteErr = s.deleteToken(ctx, q)
		return deleteErr
	})
	return resp, err
}

func (s *Server) deleteToken(ctx context.Context, q *project.ProjectTokenDeleteRequest) (*project.EmptyResponse, error) {
	prj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Project, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = validateProject(prj)
	if err != nil {
		return nil, fmt.Errorf("error validating project: %w", err)
	}

	s.projectLock.Lock(q.Project)
	defer s.projectLock.Unlock(q.Project)

	role, roleIndex, err := prj.GetRoleByName(q.Role)
	if err != nil {
		return &project.EmptyResponse{}, nil
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project); err != nil {
		if !jwtutil.IsMember(jwtutil.Claims(ctx.Value("claims")), role.Groups, s.policyEnf.GetScopes()) {
			return nil, err
		}
	}

	err = prj.RemoveJWTToken(roleIndex, q.Iat, q.Id)
	if err != nil {
		return &project.EmptyResponse{}, nil
	}

	_, err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(ctx, prj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	s.logEvent(prj, ctx, argo.EventReasonResourceDeleted, "deleted token")

	return &project.EmptyResponse{}, nil
}

// Create a new project
func (s *Server) Create(ctx context.Context, q *project.ProjectCreateRequest) (*v1alpha1.AppProject, error) {
	if q.Project == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload 'project' in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionCreate, q.Project.Name); err != nil {
		return nil, err
	}
	q.Project.NormalizePolicies()
	err := validateProject(q.Project)
	if err != nil {
		return nil, fmt.Errorf("error validating project: %w", err)
	}
	res, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Create(ctx, q.Project, metav1.CreateOptions{})
	if apierr.IsAlreadyExists(err) {
		existing, getErr := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Project.Name, metav1.GetOptions{})
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing project details: %v", getErr)
		}
		if q.GetUpsert() {
			if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.GetProject().Name); err != nil {
				return nil, err
			}
			existing.Spec = q.GetProject().Spec
			res, err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(ctx, existing, metav1.UpdateOptions{})
		} else {
			if !reflect.DeepEqual(existing.Spec, q.GetProject().Spec) {
				return nil, status.Error(codes.InvalidArgument, argo.GenerateSpecIsDifferentErrorMessage("project", existing.Spec, q.GetProject().Spec))
			}
			return existing, nil
		}
	}
	if err == nil {
		s.logEvent(res, ctx, argo.EventReasonResourceCreated, "created project")
	}
	return res, err
}

// List returns list of projects
func (s *Server) List(ctx context.Context, q *project.ProjectQuery) (*v1alpha1.AppProjectList, error) {
	list, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).List(ctx, metav1.ListOptions{})
	if list != nil {
		newItems := make([]v1alpha1.AppProject, 0)
		for i := range list.Items {
			project := list.Items[i]
			if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, project.Name) {
				newItems = append(newItems, project)
			}
		}
		list.Items = newItems
	}
	return list, err
}

// GetDetailedProject returns a project with scoped resources
func (s *Server) GetDetailedProject(ctx context.Context, q *project.ProjectQuery) (*project.DetailedProjectsResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name); err != nil {
		return nil, err
	}
	proj, repositories, clusters, err := argo.GetAppProjectWithScopedResources(q.Name, listersv1alpha1.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db, ctx)
	if err != nil {
		return nil, err
	}
	proj.NormalizeJWTTokens()
	globalProjects := argo.GetGlobalProjects(proj, listersv1alpha1.NewAppProjectLister(s.projInformer.GetIndexer()), s.settingsMgr)

	return &project.DetailedProjectsResponse{
		GlobalProjects: globalProjects,
		Project:        proj,
		Repositories:   repositories,
		Clusters:       clusters,
	}, err
}

// Get returns a project by name
func (s *Server) Get(ctx context.Context, q *project.ProjectQuery) (*v1alpha1.AppProject, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name); err != nil {
		return nil, err
	}
	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	proj.NormalizeJWTTokens()
	return proj, err
}

// GetGlobalProjects returns global projects
func (s *Server) GetGlobalProjects(ctx context.Context, q *project.ProjectQuery) (*project.GlobalProjectsResponse, error) {
	projOrig, err := s.Get(ctx, q)
	if err != nil {
		return nil, err
	}

	globalProjects := argo.GetGlobalProjects(projOrig, listersv1alpha1.NewAppProjectLister(s.projInformer.GetIndexer()), s.settingsMgr)

	res := &project.GlobalProjectsResponse{}
	res.Items = globalProjects
	return res, nil
}

// Update updates a project
func (s *Server) Update(ctx context.Context, q *project.ProjectUpdateRequest) (*v1alpha1.AppProject, error) {
	if q.Project == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload 'project' in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project.Name); err != nil {
		return nil, err
	}
	q.Project.NormalizePolicies()
	q.Project.NormalizeJWTTokens()
	err := validateProject(q.Project)
	if err != nil {
		return nil, err
	}
	s.projectLock.Lock(q.Project.Name)
	defer s.projectLock.Unlock(q.Project.Name)

	oldProj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Project.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, cluster := range difference(q.Project.Spec.DestinationClusters(), oldProj.Spec.DestinationClusters()) {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, cluster); err != nil {
			return nil, err
		}
	}

	for _, repoUrl := range difference(q.Project.Spec.SourceRepos, oldProj.Spec.SourceRepos) {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, repoUrl); err != nil {
			return nil, err
		}
	}

	clusterResourceWhitelistsEqual := reflect.DeepEqual(q.Project.Spec.ClusterResourceWhitelist, oldProj.Spec.ClusterResourceWhitelist)
	clusterResourceBlacklistsEqual := reflect.DeepEqual(q.Project.Spec.ClusterResourceBlacklist, oldProj.Spec.ClusterResourceBlacklist)
	namespacesResourceBlacklistsEqual := reflect.DeepEqual(q.Project.Spec.NamespaceResourceBlacklist, oldProj.Spec.NamespaceResourceBlacklist)
	namespacesResourceWhitelistsEqual := reflect.DeepEqual(q.Project.Spec.NamespaceResourceWhitelist, oldProj.Spec.NamespaceResourceWhitelist)
	if !clusterResourceWhitelistsEqual || !clusterResourceBlacklistsEqual || !namespacesResourceBlacklistsEqual || !namespacesResourceWhitelistsEqual {
		for _, cluster := range q.Project.Spec.DestinationClusters() {
			if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, cluster); err != nil {
				return nil, err
			}
		}
	}

	appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var srcValidatedApps []v1alpha1.Application
	var dstValidatedApps []v1alpha1.Application
	getProjectClusters := func(project string) ([]*v1alpha1.Cluster, error) {
		return s.db.GetProjectClusters(ctx, project)
	}

	for _, a := range argo.FilterByProjects(appsList.Items, []string{q.Project.Name}) {
		if oldProj.IsSourcePermitted(a.Spec.GetSource()) {
			srcValidatedApps = append(srcValidatedApps, a)
		}

		dstPermitted, err := oldProj.IsDestinationPermitted(a.Spec.Destination, getProjectClusters)
		if err != nil {
			return nil, err
		}

		if dstPermitted {
			dstValidatedApps = append(dstValidatedApps, a)
		}
	}

	invalidSrcCount := 0
	invalidDstCount := 0

	for _, a := range srcValidatedApps {
		if !q.Project.IsSourcePermitted(a.Spec.GetSource()) {
			invalidSrcCount++
		}
	}
	for _, a := range dstValidatedApps {
		dstPermitted, err := q.Project.IsDestinationPermitted(a.Spec.Destination, getProjectClusters)
		if err != nil {
			return nil, err
		}

		if !dstPermitted {
			invalidDstCount++
		}
	}

	var parts []string
	if invalidSrcCount > 0 {
		parts = append(parts, fmt.Sprintf("%d applications source became invalid", invalidSrcCount))
	}
	if invalidDstCount > 0 {
		parts = append(parts, fmt.Sprintf("%d applications destination became invalid", invalidDstCount))
	}
	if len(parts) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "as a result of project update %s", strings.Join(parts, " and "))
	}

	res, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(ctx, q.Project, metav1.UpdateOptions{})
	if err == nil {
		s.logEvent(res, ctx, argo.EventReasonResourceUpdated, "updated project")
	}
	return res, err
}

// Delete deletes a project
func (s *Server) Delete(ctx context.Context, q *project.ProjectQuery) (*project.EmptyResponse, error) {
	if q.Name == v1alpha1.DefaultAppProjectName {
		return nil, status.Errorf(codes.InvalidArgument, "name '%s' is reserved and cannot be deleted", q.Name)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionDelete, q.Name); err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Name)
	defer s.projectLock.Unlock(q.Name)

	p, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	apps := argo.FilterByProjects(appsList.Items, []string{q.Name})
	if len(apps) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "project is referenced by %d applications", len(apps))
	}
	err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Delete(ctx, q.Name, metav1.DeleteOptions{})
	if err == nil {
		s.logEvent(p, ctx, argo.EventReasonResourceDeleted, "deleted project")
	}
	return &project.EmptyResponse{}, err
}

func (s *Server) ListEvents(ctx context.Context, q *project.ProjectQuery) (*v1.EventList, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name); err != nil {
		return nil, err
	}
	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	fieldSelector := fields.SelectorFromSet(map[string]string{
		"involvedObject.name":      proj.Name,
		"involvedObject.uid":       string(proj.UID),
		"involvedObject.namespace": proj.Namespace,
	}).String()
	return s.kubeclientset.CoreV1().Events(s.ns).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
}

func (s *Server) logEvent(a *v1alpha1.AppProject, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppProjEvent(a, eventInfo, message, user)
}

func (s *Server) GetSyncWindowsState(ctx context.Context, q *project.SyncWindowsQuery) (*project.SyncWindowsResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name); err != nil {
		return nil, err
	}
	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	res := &project.SyncWindowsResponse{}

	windows := proj.Spec.SyncWindows.Active()
	if windows.HasWindows() {
		res.Windows = *windows
	} else {
		res.Windows = []*v1alpha1.SyncWindow{}
	}

	return res, nil
}

func (s *Server) NormalizeProjs() error {
	projList, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return status.Errorf(codes.Internal, "Error retrieving project list: %s", err.Error())
	}
	for _, proj := range projList.Items {
		for i := 0; i < 3; i++ {
			if proj.NormalizeJWTTokens() {
				_, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(context.Background(), &proj, metav1.UpdateOptions{})
				if err == nil {
					log.Infof("Successfully normalized project %s.", proj.Name)
					break
				}
				if !apierr.IsConflict(err) {
					log.Warnf("Failed normalize project %s", proj.Name)
					break
				}
				projGet, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(context.Background(), proj.Name, metav1.GetOptions{})
				if err != nil {
					return status.Errorf(codes.Internal, "Error retrieving project: %s", err.Error())
				}
				proj = *projGet
				if i == 2 {
					return status.Errorf(codes.Internal, "Failed normalize project %s", proj.Name)
				}
			} else {
				break
			}
		}
	}
	return nil
}

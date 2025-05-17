package rbac

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v3/util/assets"
	claimsutil "github.com/argoproj/argo-cd/v3/util/claims"
	"github.com/argoproj/argo-cd/v3/util/glob"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/util"
	"github.com/casbin/govaluate"
	"github.com/golang-jwt/jwt/v5"
	gocache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	ConfigMapPolicyCSVKey     = "policy.csv"
	ConfigMapPolicyDefaultKey = "policy.default"
	ConfigMapScopesKey        = "scopes"
	ConfigMapMatchModeKey     = "policy.matchMode"
	GlobMatchMode             = "glob"
	RegexMatchMode            = "regex"

	defaultRBACSyncPeriod = 10 * time.Minute
)

// CasbinEnforcer represents methods that must be implemented by a Casbin enforces
type CasbinEnforcer interface {
	EnableLog(bool)
	Enforce(rvals ...any) (bool, error)
	LoadPolicy() error
	EnableEnforce(bool)
	AddFunction(name string, function govaluate.ExpressionFunction)
	GetGroupingPolicy() ([][]string, error)
	GetAllRoles() ([]string, error)
	GetImplicitPermissionsForUser(user string, domain ...string) ([][]string, error)
}

const (
	// please add new items to Resources
	ResourceClusters          = "clusters"
	ResourceProjects          = "projects"
	ResourceApplications      = "applications"
	ResourceApplicationSets   = "applicationsets"
	ResourceRepositories      = "repositories"
	ResourceWriteRepositories = "write-repositories"
	ResourceCertificates      = "certificates"
	ResourceAccounts          = "accounts"
	ResourceGPGKeys           = "gpgkeys"
	ResourceLogs              = "logs"
	ResourceExec              = "exec"
	ResourceExtensions        = "extensions"

	// please add new items to Actions
	ActionGet      = "get"
	ActionCreate   = "create"
	ActionUpdate   = "update"
	ActionDelete   = "delete"
	ActionSync     = "sync"
	ActionOverride = "override"
	ActionAction   = "action"
	ActionInvoke   = "invoke"
)

var (
	DefaultScopes = []string{"groups"}
	Resources     = []string{
		ResourceClusters,
		ResourceProjects,
		ResourceApplications,
		ResourceApplicationSets,
		ResourceRepositories,
		ResourceWriteRepositories,
		ResourceCertificates,
		ResourceAccounts,
		ResourceGPGKeys,
		ResourceLogs,
		ResourceExec,
		ResourceExtensions,
	}
	Actions = []string{
		ActionGet,
		ActionCreate,
		ActionUpdate,
		ActionDelete,
		ActionSync,
		ActionOverride,
		ActionAction,
		ActionInvoke,
	}
)

var ProjectScoped = map[string]bool{
	ResourceApplications:    true,
	ResourceApplicationSets: true,
	ResourceLogs:            true,
	ResourceExec:            true,
	ResourceClusters:        true,
	ResourceRepositories:    true,
}

// Enforcer is a wrapper around an Casbin enforcer that:
// * is backed by a kubernetes config map
// * has a predefined RBAC model
// * supports a built-in policy
// * supports a user-defined policy
// * supports a custom JWT claims enforce function
type Enforcer struct {
	lock               sync.Mutex
	enforcerCache      *gocache.Cache
	adapter            *argocdAdapter
	enableLog          bool
	enabled            bool
	clientset          kubernetes.Interface
	namespace          string
	configmap          string
	claimsEnforcerFunc ClaimsEnforcerFunc
	model              model.Model
	defaultRole        string
	matchMode          string
}

// cachedEnforcer holds the Casbin enforcer instances and optional custom project policy
type cachedEnforcer struct {
	enforcer CasbinEnforcer
	policy   string
}

func (e *Enforcer) invalidateCache(actions ...func()) {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, action := range actions {
		action()
	}
	e.enforcerCache.Flush()
}

func (e *Enforcer) getCasbinEnforcer(project string, policy string) CasbinEnforcer {
	res, err := e.tryGetCasbinEnforcer(project, policy)
	if err != nil {
		panic(err)
	}
	return res
}

// tryGetCasbinEnforcer returns the cached enforcer for the given optional project and project policy.
func (e *Enforcer) tryGetCasbinEnforcer(project string, policy string) (CasbinEnforcer, error) {
	e.lock.Lock()
	defer e.lock.Unlock()
	var cached *cachedEnforcer
	val, ok := e.enforcerCache.Get(project)
	if ok {
		if c, ok := val.(*cachedEnforcer); ok && c.policy == policy {
			cached = c
		}
	}
	if cached != nil {
		return cached.enforcer, nil
	}
	matchFunc := globMatchFunc
	if e.matchMode == RegexMatchMode {
		matchFunc = util.RegexMatchFunc
	}

	var err error
	var enforcer CasbinEnforcer
	if policy != "" {
		if enforcer, err = newEnforcerSafe(matchFunc, e.model, newAdapter(e.adapter.builtinPolicy, e.adapter.userDefinedPolicy, policy)); err != nil {
			// fallback to default policy if project policy is invalid
			log.Errorf("Failed to load project '%s' policy", project)
			enforcer, err = newEnforcerSafe(matchFunc, e.model, e.adapter)
		}
	} else {
		enforcer, err = newEnforcerSafe(matchFunc, e.model, e.adapter)
	}
	if err != nil {
		return nil, err
	}

	enforcer.AddFunction("globOrRegexMatch", matchFunc)
	enforcer.EnableLog(e.enableLog)
	enforcer.EnableEnforce(e.enabled)
	e.enforcerCache.SetDefault(project, &cachedEnforcer{enforcer: enforcer, policy: policy})
	return enforcer, nil
}

// ClaimsEnforcerFunc is func template to enforce a JWT claims. The subject is replaced
type ClaimsEnforcerFunc func(claims jwt.Claims, rvals ...any) bool

func newEnforcerSafe(matchFunction govaluate.ExpressionFunction, params ...any) (e CasbinEnforcer, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
			e = nil
		}
	}()
	enfs, err := casbin.NewCachedEnforcer(params...)
	if err != nil {
		return nil, err
	}
	enfs.AddFunction("globOrRegexMatch", matchFunction)
	return enfs, nil
}

func NewEnforcer(clientset kubernetes.Interface, namespace, configmap string, claimsEnforcer ClaimsEnforcerFunc) *Enforcer {
	adapter := newAdapter("", "", "")
	builtInModel := newBuiltInModel()
	return &Enforcer{
		enforcerCache:      gocache.New(time.Hour, time.Hour),
		adapter:            adapter,
		clientset:          clientset,
		namespace:          namespace,
		configmap:          configmap,
		model:              builtInModel,
		claimsEnforcerFunc: claimsEnforcer,
		enabled:            true,
	}
}

// EnableLog executes casbin.Enforcer functionality.
func (e *Enforcer) EnableLog(s bool) {
	e.invalidateCache(func() {
		e.enableLog = s
	})
}

// EnableEnforce executes casbin.Enforcer functionality and will invalidate cache if required.
func (e *Enforcer) EnableEnforce(s bool) {
	e.invalidateCache(func() {
		e.enabled = s
	})
}

// LoadPolicy executes casbin.Enforcer functionality and will invalidate cache if required.
func (e *Enforcer) LoadPolicy() error {
	_, err := e.tryGetCasbinEnforcer("", "")
	return err
}

// CheckUserDefinedRoleReferentialIntegrity iterates over roles and policies to validate the existence of a matching policy subject for every defined role
func CheckUserDefinedRoleReferentialIntegrity(e CasbinEnforcer) error {
	allRoles, err := e.GetAllRoles()
	if err != nil {
		return err
	}
	notFound := make([]string, 0)
	for _, roleName := range allRoles {
		permissions, err := e.GetImplicitPermissionsForUser(roleName)
		if err != nil {
			return err
		}
		if len(permissions) == 0 {
			notFound = append(notFound, roleName)
		}
	}
	if len(notFound) > 0 {
		return fmt.Errorf("user defined roles not found in policies: %s", strings.Join(notFound, ","))
	}
	return nil
}

// Glob match func
func globMatchFunc(args ...any) (any, error) {
	if len(args) < 2 {
		return false, nil
	}
	val, ok := args[0].(string)
	if !ok {
		return false, nil
	}

	pattern, ok := args[1].(string)
	if !ok {
		return false, nil
	}

	return glob.Match(pattern, val), nil
}

// SetMatchMode set match mode on runtime, glob match or regex match
func (e *Enforcer) SetMatchMode(mode string) {
	e.invalidateCache(func() {
		if mode == RegexMatchMode {
			e.matchMode = RegexMatchMode
		} else {
			e.matchMode = GlobMatchMode
		}
	})
}

// SetDefaultRole sets a default role to use during enforcement. Will fall back to this role if
// normal enforcement fails
func (e *Enforcer) SetDefaultRole(roleName string) {
	e.defaultRole = roleName
}

// SetClaimsEnforcerFunc sets a claims enforce function during enforcement. The claims enforce function
// can extract claims from JWT token and do the proper enforcement based on user, group or any information
// available in the input parameter list
func (e *Enforcer) SetClaimsEnforcerFunc(claimsEnforcer ClaimsEnforcerFunc) {
	e.claimsEnforcerFunc = claimsEnforcer
}

// Enforce is a wrapper around casbin.Enforce to additionally enforce a default role and a custom
// claims function
func (e *Enforcer) Enforce(rvals ...any) bool {
	return enforce(e.getCasbinEnforcer("", ""), e.defaultRole, e.claimsEnforcerFunc, rvals...)
}

// EnforceErr is a convenience helper to wrap a failed enforcement with a detailed error about the request
func (e *Enforcer) EnforceErr(rvals ...any) error {
	if !e.Enforce(rvals...) {
		errMsg := "permission denied"

		if len(rvals) > 0 {
			rvalsStrs := make([]string, len(rvals)-1)
			for i, rval := range rvals[1:] {
				rvalsStrs[i] = fmt.Sprintf("%s", rval)
			}
			if s, ok := rvals[0].(jwt.Claims); ok {
				claims, err := jwtutil.MapClaims(s)
				if err == nil {
					argoClaims, err := claimsutil.MapClaimsToArgoClaims(claims)
					if err == nil {
						if argoClaims.GetUserIdentifier() != "" {
							rvalsStrs = append(rvalsStrs, "sub: "+argoClaims.GetUserIdentifier())
						}
						if issuedAtTime, err := jwtutil.IssuedAtTime(claims); err == nil {
							rvalsStrs = append(rvalsStrs, "iat: "+issuedAtTime.Format(time.RFC3339))
						}
					}
				}
			}
			errMsg = fmt.Sprintf("%s: %s", errMsg, strings.Join(rvalsStrs, ", "))
		}

		return status.Error(codes.PermissionDenied, errMsg)
	}
	return nil
}

// EnforceRuntimePolicy enforces a policy defined at run-time which augments the built-in and
// user-defined policy. This allows any explicit denies of the built-in, and user-defined policies
// to override the run-time policy. Runs normal enforcement if run-time policy is empty.
func (e *Enforcer) EnforceRuntimePolicy(project string, policy string, rvals ...any) bool {
	enf := e.CreateEnforcerWithRuntimePolicy(project, policy)
	return e.EnforceWithCustomEnforcer(enf, rvals...)
}

// CreateEnforcerWithRuntimePolicy creates an enforcer with a policy defined at run-time which augments the built-in and
// user-defined policy. This allows any explicit denies of the built-in, and user-defined policies
// to override the run-time policy. Runs normal enforcement if run-time policy is empty.
func (e *Enforcer) CreateEnforcerWithRuntimePolicy(project string, policy string) CasbinEnforcer {
	return e.getCasbinEnforcer(project, policy)
}

// EnforceWithCustomEnforcer wraps enforce with an custom enforcer
func (e *Enforcer) EnforceWithCustomEnforcer(enf CasbinEnforcer, rvals ...any) bool {
	return enforce(enf, e.defaultRole, e.claimsEnforcerFunc, rvals...)
}

// enforce is a helper to additionally check a default role and invoke a custom claims enforcement function
func enforce(enf CasbinEnforcer, defaultRole string, claimsEnforcerFunc ClaimsEnforcerFunc, rvals ...any) bool {
	// check the default role
	if defaultRole != "" && len(rvals) >= 2 {
		if ok, err := enf.Enforce(append([]any{defaultRole}, rvals[1:]...)...); ok && err == nil {
			return true
		}
	}
	if len(rvals) == 0 {
		return false
	}
	// check if subject is jwt.Claims vs. a normal subject string and run custom claims
	// enforcement func (if set)
	sub := rvals[0]
	switch s := sub.(type) {
	case string:
		// noop
	case jwt.Claims:
		if claimsEnforcerFunc != nil && claimsEnforcerFunc(s, rvals...) {
			return true
		}
		rvals = append([]any{""}, rvals[1:]...)
	default:
		rvals = append([]any{""}, rvals[1:]...)
	}
	ok, err := enf.Enforce(rvals...)
	return ok && err == nil
}

// SetBuiltinPolicy sets a built-in policy, which augments any user defined policies
func (e *Enforcer) SetBuiltinPolicy(policy string) error {
	e.invalidateCache(func() {
		e.adapter.builtinPolicy = policy
	})
	return e.LoadPolicy()
}

// SetUserPolicy sets a user policy, augmenting the built-in policy
func (e *Enforcer) SetUserPolicy(policy string) error {
	e.invalidateCache(func() {
		e.adapter.userDefinedPolicy = policy
	})
	return e.LoadPolicy()
}

// newInformers returns an informer which watches updates on the rbac configmap
func (e *Enforcer) newInformer() cache.SharedIndexInformer {
	tweakConfigMap := func(options *metav1.ListOptions) {
		cmFieldSelector := fields.ParseSelectorOrDie("metadata.name=" + e.configmap)
		options.FieldSelector = cmFieldSelector.String()
	}
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	return informersv1.NewFilteredConfigMapInformer(e.clientset, e.namespace, defaultRBACSyncPeriod, indexers, tweakConfigMap)
}

// RunPolicyLoader runs the policy loader which watches policy updates from the configmap and reloads them
func (e *Enforcer) RunPolicyLoader(ctx context.Context, onUpdated func(cm *corev1.ConfigMap) error) error {
	cm, err := e.clientset.CoreV1().ConfigMaps(e.namespace).Get(ctx, e.configmap, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		err = e.syncUpdate(cm, onUpdated)
		if err != nil {
			return err
		}
	}
	e.runInformer(ctx, onUpdated)
	return nil
}

func (e *Enforcer) runInformer(ctx context.Context, onUpdated func(cm *corev1.ConfigMap) error) {
	cmInformer := e.newInformer()
	_, err := cmInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				if cm, ok := obj.(*corev1.ConfigMap); ok {
					err := e.syncUpdate(cm, onUpdated)
					if err != nil {
						log.Error(err)
					} else {
						log.Infof("RBAC ConfigMap '%s' added", e.configmap)
					}
				}
			},
			UpdateFunc: func(old, new any) {
				oldCM := old.(*corev1.ConfigMap)
				newCM := new.(*corev1.ConfigMap)
				if oldCM.ResourceVersion == newCM.ResourceVersion {
					return
				}
				err := e.syncUpdate(newCM, onUpdated)
				if err != nil {
					log.Error(err)
				} else {
					log.Infof("RBAC ConfigMap '%s' updated", e.configmap)
				}
			},
		},
	)
	if err != nil {
		log.Error(err)
	}
	log.Info("Starting rbac config informer")
	cmInformer.Run(ctx.Done())
	log.Info("rbac configmap informer cancelled")
}

// PolicyCSV will generate the final policy csv to be used
// by Argo CD RBAC. It will find entries in the given data
// that matches the policy key name convention:
//
//	policy[.overlay].csv
func PolicyCSV(data map[string]string) string {
	var strBuilder strings.Builder
	// add the main policy first
	if p, ok := data[ConfigMapPolicyCSVKey]; ok {
		strBuilder.WriteString(p)
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// append additional policies at the end of the csv
	for _, key := range keys {
		value := data[key]
		if strings.HasPrefix(key, "policy.") &&
			strings.HasSuffix(key, ".csv") &&
			key != ConfigMapPolicyCSVKey {
			strBuilder.WriteString("\n")
			strBuilder.WriteString(value)
		}
	}
	return strBuilder.String()
}

// syncUpdate updates the enforcer
func (e *Enforcer) syncUpdate(cm *corev1.ConfigMap, onUpdated func(cm *corev1.ConfigMap) error) error {
	e.SetDefaultRole(cm.Data[ConfigMapPolicyDefaultKey])
	e.SetMatchMode(cm.Data[ConfigMapMatchModeKey])
	policyCSV := PolicyCSV(cm.Data)
	if err := onUpdated(cm); err != nil {
		return err
	}
	return e.SetUserPolicy(policyCSV)
}

// ValidatePolicy verifies a policy string is acceptable to casbin
func ValidatePolicy(policy string) error {
	casbinEnforcer, err := newEnforcerSafe(globMatchFunc, newBuiltInModel(), newAdapter("", "", policy))
	if err != nil {
		return fmt.Errorf("policy syntax error: %s", policy)
	}

	// Check for referential integrity
	if err := CheckUserDefinedRoleReferentialIntegrity(casbinEnforcer); err != nil {
		log.Warning(err.Error())
	}
	return nil
}

// newBuiltInModel is a helper to return a brand new casbin model from the built-in model string.
// This is needed because it is not safe to re-use the same casbin Model when instantiating new
// casbin enforcers.
func newBuiltInModel() model.Model {
	m, err := model.NewModelFromString(assets.ModelConf)
	if err != nil {
		panic(err)
	}
	return m
}

// Casbin adapter which satisfies persist.Adapter interface
type argocdAdapter struct {
	builtinPolicy     string
	userDefinedPolicy string
	runtimePolicy     string
}

func newAdapter(builtinPolicy, userDefinedPolicy, runtimePolicy string) *argocdAdapter {
	return &argocdAdapter{
		builtinPolicy:     builtinPolicy,
		userDefinedPolicy: userDefinedPolicy,
		runtimePolicy:     runtimePolicy,
	}
}

func (a *argocdAdapter) LoadPolicy(model model.Model) error {
	for _, policyStr := range []string{a.builtinPolicy, a.userDefinedPolicy, a.runtimePolicy} {
		for _, line := range strings.Split(policyStr, "\n") {
			if err := loadPolicyLine(strings.TrimSpace(line), model); err != nil {
				return err
			}
		}
	}
	return nil
}

// The modified version of LoadPolicyLine function defined in "persist" package of github.com/casbin/casbin.
// Uses CVS parser to correctly handle quotes in policy line.
func loadPolicyLine(line string, model model.Model) error {
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	reader := csv.NewReader(strings.NewReader(line))
	reader.TrimLeadingSpace = true
	tokens, err := reader.Read()
	if err != nil {
		return err
	}

	tokenLen := len(tokens)

	if tokenLen < 1 ||
		tokens[0] == "" ||
		(tokens[0] == "g" && tokenLen != 3) ||
		(tokens[0] == "p" && tokenLen != 6) {
		return fmt.Errorf("invalid RBAC policy: %s", line)
	}

	key := tokens[0]
	sec := key[:1]
	if _, ok := model[sec]; !ok {
		return fmt.Errorf("invalid RBAC policy: %s", line)
	}
	if _, ok := model[sec][key]; !ok {
		return fmt.Errorf("invalid RBAC policy: %s", line)
	}
	model[sec][key].Policy = append(model[sec][key].Policy, tokens[1:])
	return nil
}

func (a *argocdAdapter) SavePolicy(_ model.Model) error {
	return errors.New("not implemented")
}

func (a *argocdAdapter) AddPolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

func (a *argocdAdapter) RemovePolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

func (a *argocdAdapter) RemoveFilteredPolicy(_ string, _ string, _ int, _ ...string) error {
	return errors.New("not implemented")
}

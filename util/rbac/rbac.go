package rbac

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/util/assets"
	"github.com/argoproj/argo-cd/util/glob"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"

	"github.com/casbin/casbin"
	"github.com/casbin/casbin/model"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	ConfigMapPolicyCSVKey     = "policy.csv"
	ConfigMapPolicyDefaultKey = "policy.default"
	ConfigMapScopesKey        = "scopes"

	defaultRBACSyncPeriod = 10 * time.Minute
)

// Enforcer is a wrapper around an Casbin enforcer that:
// * is backed by a kubernetes config map
// * has a predefined RBAC model
// * supports a built-in policy
// * supports a user-defined policy
// * supports a custom JWT claims enforce function
type Enforcer struct {
	*casbin.Enforcer
	adapter            *argocdAdapter
	clientset          kubernetes.Interface
	namespace          string
	configmap          string
	claimsEnforcerFunc ClaimsEnforcerFunc
	model              model.Model
	defaultRole        string
}

// ClaimsEnforcerFunc is func template to enforce a JWT claims. The subject is replaced
type ClaimsEnforcerFunc func(claims jwt.Claims, rvals ...interface{}) bool

func newEnforcerSafe(params ...interface{}) (e *casbin.Enforcer, err error) {
	enfs, err := casbin.NewEnforcerSafe(params...)
	if err != nil {
		return nil, err
	}
	enfs.AddFunction("globMatch", func(args ...interface{}) (interface{}, error) {
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
	})
	return enfs, nil
}

func NewEnforcer(clientset kubernetes.Interface, namespace, configmap string, claimsEnforcer ClaimsEnforcerFunc) *Enforcer {
	adapter := newAdapter("", "", "")
	builtInModel := newBuiltInModel()
	enf, err := newEnforcerSafe(builtInModel, adapter)
	if err != nil {
		panic(err)
	}
	enf.EnableLog(false)
	return &Enforcer{
		Enforcer:           enf,
		adapter:            adapter,
		clientset:          clientset,
		namespace:          namespace,
		configmap:          configmap,
		model:              builtInModel,
		claimsEnforcerFunc: claimsEnforcer,
	}
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
func (e *Enforcer) Enforce(rvals ...interface{}) bool {
	return enforce(e.Enforcer, e.defaultRole, e.claimsEnforcerFunc, rvals...)
}

// EnforceErr is a convenience helper to wrap a failed enforcement with a detailed error about the request
func (e *Enforcer) EnforceErr(rvals ...interface{}) error {
	if !e.Enforce(rvals...) {
		errMsg := "permission denied"
		if len(rvals) > 0 {
			rvalsStrs := make([]string, len(rvals)-1)
			for i, rval := range rvals[1:] {
				rvalsStrs[i] = fmt.Sprintf("%s", rval)
			}
			switch s := rvals[0].(type) {
			case jwt.Claims:
				claims, err := jwtutil.MapClaims(s)
				if err != nil {
					break
				}
				sub := jwtutil.GetField(claims, "sub")
				if sub != "" {
					rvalsStrs = append(rvalsStrs, fmt.Sprintf("sub: %s", sub))
				}
				iatField, ok := claims["iat"]
				if !ok {
					break
				}
				iat, ok := iatField.(float64)
				if !ok {
					break
				}
				rvalsStrs = append(rvalsStrs, fmt.Sprintf("iat: %s", time.Unix(int64(iat), 0).Format(time.RFC3339)))
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
func (e *Enforcer) EnforceRuntimePolicy(policy string, rvals ...interface{}) bool {
	var enf *casbin.Enforcer
	var err error
	if policy == "" {
		enf = e.Enforcer
	} else {
		enf, err = newEnforcerSafe(newBuiltInModel(), newAdapter(e.adapter.builtinPolicy, e.adapter.userDefinedPolicy, policy))
		if err != nil {
			log.Warnf("invalid runtime policy: %s", policy)
			enf = e.Enforcer
		}
	}
	return enforce(enf, e.defaultRole, e.claimsEnforcerFunc, rvals...)
}

// enforce is a helper to additionally check a default role and invoke a custom claims enforcement function
func enforce(enf *casbin.Enforcer, defaultRole string, claimsEnforcerFunc ClaimsEnforcerFunc, rvals ...interface{}) bool {
	// check the default role
	if defaultRole != "" && len(rvals) >= 2 {
		if enf.Enforce(append([]interface{}{defaultRole}, rvals[1:]...)...) {
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
		rvals = append([]interface{}{""}, rvals[1:]...)
	default:
		rvals = append([]interface{}{""}, rvals[1:]...)
	}
	return enf.Enforce(rvals...)
}

// SetBuiltinPolicy sets a built-in policy, which augments any user defined policies
func (e *Enforcer) SetBuiltinPolicy(policy string) error {
	e.adapter.builtinPolicy = policy
	return e.LoadPolicy()
}

// SetUserPolicy sets a user policy, augmenting the built-in policy
func (e *Enforcer) SetUserPolicy(policy string) error {
	e.adapter.userDefinedPolicy = policy
	return e.LoadPolicy()
}

// newInformers returns an informer which watches updates on the rbac configmap
func (e *Enforcer) newInformer() cache.SharedIndexInformer {
	tweakConfigMap := func(options *metav1.ListOptions) {
		cmFieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", e.configmap))
		options.FieldSelector = cmFieldSelector.String()
	}
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	return v1.NewFilteredConfigMapInformer(e.clientset, e.namespace, defaultRBACSyncPeriod, indexers, tweakConfigMap)
}

// RunPolicyLoader runs the policy loader which watches policy updates from the configmap and reloads them
func (e *Enforcer) RunPolicyLoader(ctx context.Context, onUpdated func(cm *apiv1.ConfigMap) error) error {
	cm, err := e.clientset.CoreV1().ConfigMaps(e.namespace).Get(ctx, e.configmap, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
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

func (e *Enforcer) runInformer(ctx context.Context, onUpdated func(cm *apiv1.ConfigMap) error) {
	cmInformer := e.newInformer()
	cmInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if cm, ok := obj.(*apiv1.ConfigMap); ok {
					err := e.syncUpdate(cm, onUpdated)
					if err != nil {
						log.Error(err)
					} else {
						log.Infof("RBAC ConfigMap '%s' added", e.configmap)
					}
				}
			},
			UpdateFunc: func(old, new interface{}) {
				oldCM := old.(*apiv1.ConfigMap)
				newCM := new.(*apiv1.ConfigMap)
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
	log.Info("Starting rbac config informer")
	cmInformer.Run(ctx.Done())
	log.Info("rbac configmap informer cancelled")
}

// syncUpdate updates the enforcer
func (e *Enforcer) syncUpdate(cm *apiv1.ConfigMap, onUpdated func(cm *apiv1.ConfigMap) error) error {
	e.SetDefaultRole(cm.Data[ConfigMapPolicyDefaultKey])
	policyCSV, ok := cm.Data[ConfigMapPolicyCSVKey]
	if !ok {
		policyCSV = ""
	}
	if err := onUpdated(cm); err != nil {
		return err
	}
	return e.SetUserPolicy(policyCSV)
}

// ValidatePolicy verifies a policy string is acceptable to casbin
func ValidatePolicy(policy string) error {
	_, err := newEnforcerSafe(newBuiltInModel(), newAdapter("", "", policy))
	if err != nil {
		return fmt.Errorf("policy syntax error: %s", policy)
	}
	return nil
}

// newBuiltInModel is a helper to return a brand new casbin model from the built-in model string.
// This is needed because it is not safe to re-use the same casbin Model when instantiating new
// casbin enforcers.
func newBuiltInModel() model.Model {
	return casbin.NewModel(assets.ModelConf)
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

	key := tokens[0]
	sec := key[:1]
	model[sec][key].Policy = append(model[sec][key].Policy, tokens[1:])
	return nil
}

func (a *argocdAdapter) SavePolicy(model model.Model) error {
	return errors.New("not implemented")
}

func (a *argocdAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	return errors.New("not implemented")
}

func (a *argocdAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	return errors.New("not implemented")
}

func (a *argocdAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	return errors.New("not implemented")
}

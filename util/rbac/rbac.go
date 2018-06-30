package rbac

import (
	"context"
	"fmt"
	"time"

	"github.com/casbin/casbin"
	"github.com/casbin/casbin/model"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gobuffalo/packr"
	scas "github.com/qiangmzsx/string-adapter"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	jwtutil "github.com/argoproj/argo-cd/util/jwt"
)

const (
	ConfigMapPolicyCSVKey     = "policy.csv"
	ConfigMapPolicyDefaultKey = "policy.default"

	builtinModelFile      = "model.conf"
	defaultRBACSyncPeriod = 10 * time.Minute
)

// ClaimsEnforcerFunc is func template
type ClaimsEnforcerFunc func(rvals ...interface{}) bool

type Enforcer struct {
	*casbin.Enforcer
	adapter            *scas.Adapter
	clientset          kubernetes.Interface
	namespace          string
	configmap          string
	claimsEnforcerFunc ClaimsEnforcerFunc

	model             model.Model
	defaultRole       string
	builtinPolicy     string
	userDefinedPolicy string
}

func NewEnforcer(clientset kubernetes.Interface, namespace, configmap string, claimsEnforcer ClaimsEnforcerFunc) *Enforcer {
	box := packr.NewBox(".")
	modelConf := box.String(builtinModelFile)
	model := casbin.NewModel(modelConf)
	adapter := scas.NewAdapter("")
	enf := casbin.NewEnforcer(model, adapter)
	enf.EnableLog(false)
	return &Enforcer{
		Enforcer:           enf,
		adapter:            adapter,
		clientset:          clientset,
		namespace:          namespace,
		configmap:          configmap,
		model:              model,
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

// Enforce is a wrapper around casbin.Enforce to additionally enforce a default role
func (e *Enforcer) Enforce(rvals ...interface{}) bool {
	if e.Enforcer.Enforce(rvals...) {
		return true
	}
	if e.defaultRole == "" {
		return false
	}
	rvals = append([]interface{}{e.defaultRole}, rvals[1:]...)
	return e.Enforcer.Enforce(rvals...)
}

// EnforceClaims checks if the first value is a jwt.Claims and runs enforce against its groups and sub
func (e *Enforcer) EnforceClaims(rvals ...interface{}) bool {
	// Use default claims enforcer if it is nil
	if e.claimsEnforcerFunc == nil {
		return e.defaultEnforceClaims(rvals...)
	}

	return e.claimsEnforcerFunc(rvals...)
}

func (e *Enforcer) defaultEnforceClaims(rvals ...interface{}) bool {
	claims, ok := rvals[0].(jwt.Claims)
	if !ok {
		if rvals[0] == nil {
			vals := append([]interface{}{""}, rvals[1:]...)
			return e.Enforce(vals...)
		}
		return e.Enforce(rvals...)
	}
	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		vals := append([]interface{}{""}, rvals[1:]...)
		return e.Enforce(vals...)
	}
	groups := jwtutil.GetGroups(mapClaims)
	for _, group := range groups {
		vals := append([]interface{}{group}, rvals[1:]...)
		if e.Enforcer.Enforce(vals...) {
			return true
		}
	}
	user := jwtutil.GetField(mapClaims, "sub")
	vals := append([]interface{}{user}, rvals[1:]...)
	return e.Enforce(vals...)
}

// SetBuiltinPolicy sets a built-in policy, which augments any user defined policies
func (e *Enforcer) SetBuiltinPolicy(policy string) error {
	e.builtinPolicy = policy
	e.adapter.Line = fmt.Sprintf("%s\n%s", e.builtinPolicy, e.userDefinedPolicy)
	return e.LoadPolicy()
}

// SetUserPolicy sets a user policy, augmenting the built-in policy
func (e *Enforcer) SetUserPolicy(policy string) error {
	e.userDefinedPolicy = policy
	e.adapter.Line = fmt.Sprintf("%s\n%s", e.builtinPolicy, e.userDefinedPolicy)
	return e.LoadPolicy()
}

// newInformers returns an informer which watches updates on the rbac configmap
func (e *Enforcer) newInformer() cache.SharedIndexInformer {
	tweakConfigMap := func(options *metav1.ListOptions) {
		cmFieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", e.configmap))
		options.FieldSelector = cmFieldSelector.String()
	}
	return v1.NewFilteredConfigMapInformer(e.clientset, e.namespace, defaultRBACSyncPeriod, cache.Indexers{}, tweakConfigMap)
}

// RunPolicyLoader runs the policy loader which watches policy updates from the configmap and reloads them
func (e *Enforcer) RunPolicyLoader(ctx context.Context) error {
	cm, err := e.clientset.CoreV1().ConfigMaps(e.namespace).Get(e.configmap, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
	} else {
		err = e.syncUpdate(cm)
		if err != nil {
			return err
		}
	}
	e.runInformer(ctx)
	return nil
}

func (e *Enforcer) runInformer(ctx context.Context) {
	cmInformer := e.newInformer()
	cmInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if cm, ok := obj.(*apiv1.ConfigMap); ok {
					err := e.syncUpdate(cm)
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
				err := e.syncUpdate(newCM)
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
func (e *Enforcer) syncUpdate(cm *apiv1.ConfigMap) error {
	e.SetDefaultRole(cm.Data[ConfigMapPolicyDefaultKey])
	policyCSV, ok := cm.Data[ConfigMapPolicyCSVKey]
	if !ok {
		policyCSV = ""
	}
	return e.SetUserPolicy(policyCSV)
}

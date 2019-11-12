package engine

import (
	"time"

	"github.com/argoproj/argo-cd/engine/controller"

	"github.com/argoproj/argo-cd/engine/pkg"
	appv1 "github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/engine/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/engine/util/kube"
	"github.com/argoproj/argo-cd/engine/util/lua"
)

func NewEngine(
	namespace string, // TODO: remove
	settingsMgr pkg.ReconciliationSettings,
	db pkg.CredentialsStore,
	auditLogger pkg.AuditLogger,
	applicationClientset appclientset.Interface,
	repoClientset pkg.ManifestGenerator,
	argoCache pkg.AppStateCache,
	appResyncPeriod time.Duration,
	selfHealTimeout time.Duration,
	metricsPort int,
	kubectlParallelismLimit int64,
	healthCheck func() error, // TODO: remove
	luaVMFactory func(map[string]appv1.ResourceOverride) *lua.VM, // TODO: probably remove
	callbacks pkg.Callbacks,
) (pkg.Engine, error) {
	return controller.NewApplicationController(
		namespace,
		settingsMgr,
		db,
		auditLogger,
		applicationClientset,
		repoClientset,
		argoCache,
		&kube.KubectlCmd{},
		appResyncPeriod,
		selfHealTimeout,
		metricsPort,
		kubectlParallelismLimit,
		healthCheck,
		luaVMFactory,
		callbacks)
}

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/util/diff"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppComparator defines methods which allow to compare application spec and actual application state.
type AppComparator interface {
	CompareAppState(appRepoPath string, app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error)
}

// KsonnetAppComparator allows to compare application using KSonnet CLI
type KsonnetAppComparator struct {
	clusterService cluster.ClusterServiceServer
}

// CompareAppState compares application spec and real app state using KSonnet
func (ks *KsonnetAppComparator) CompareAppState(appRepoPath string, app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error) {
	log.Infof("Comparing app %s state", app.ObjectMeta.Name)
	appPath := path.Join(appRepoPath, app.Spec.Source.Path)
	ksApp, err := ksutil.NewKsonnetApp(appPath)
	if err != nil {
		return nil, err
	}
	appSpec := ksApp.AppSpec()
	env, ok := appSpec.GetEnvironmentSpec(app.Spec.Source.Environment)
	if !ok {
		return nil, fmt.Errorf("environment '%s' does not exist in ksonnet app '%s'", app.Spec.Source.Environment, appSpec.Name)
	}

	// Generate the manifests for the environment
	objs, err := ksApp.Show(app.Spec.Source.Environment)
	if err != nil {
		return nil, err
	}

	// Get the REST config for the cluster corresponding to the environment
	clst, err := ks.clusterService.Get(context.Background(), &cluster.ClusterQuery{Server: env.Destination.Server})
	if err != nil {
		return nil, err
	}

	// Retrieve the live versions of the objects
	liveObjs, err := kubeutil.GetLiveResources(clst.RESTConfig(), objs, env.Destination.Namespace)
	if err != nil {
		return nil, err
	}

	// Do the actual comparison
	diffResults, err := diff.DiffArray(objs, liveObjs)
	if err != nil {
		return nil, err
	}
	deltaDiffs := make([]string, len(diffResults.Diffs))
	for i, diffRes := range diffResults.Diffs {
		deltaDiffs[i] = diffRes.DeltaDiff
	}
	targetState := make([]string, len(objs))
	for i, obj := range objs {
		str, err := json.Marshal(obj.Object)
		if err != nil {
			return nil, err
		}
		targetState[i] = string(str)
	}
	compResult := v1alpha1.ComparisonResult{
		TargetState: targetState,
		ComparedTo:  app.Spec.Source,
		ComparedAt:  metav1.Time{Time: time.Now().UTC()},
		Server:      clst.Server,
		Namespace:   env.Destination.Namespace,
		DeltaDiffs:  deltaDiffs,
	}
	if diffResults.Modified {
		if *diffResults.AdditionsOnly {
			compResult.Status = v1alpha1.ComparisonStatusEqual
		} else {
			compResult.Status = v1alpha1.ComparisonStatusDifferent
		}
	} else {
		compResult.Status = v1alpha1.ComparisonStatusEqual
	}
	return &compResult, nil
}

// NewKsonnetAppComparator creates new instance of Ksonnet app comparator
func NewKsonnetAppComparator(clusterService cluster.ClusterServiceServer) AppComparator {
	return &KsonnetAppComparator{
		clusterService: clusterService,
	}
}

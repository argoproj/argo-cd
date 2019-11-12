package settings

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-openapi/errors"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/rest"

	"k8s.io/client-go/discovery/fake"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/pkg/client/clientset/versioned"
	appsfake "github.com/argoproj/argo-cd/engine/pkg/client/clientset/versioned/fake"
	alpha1 "github.com/argoproj/argo-cd/engine/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"k8s.io/client-go/discovery"
)

type staticAppClientSet struct {
	apps     map[string]*v1alpha1.Application
	projects alpha1.AppProjectInterface
	watch    *watch.RaceFreeFakeWatcher
	lock     *sync.Mutex
}

func (s staticAppClientSet) Create(app *v1alpha1.Application) (*v1alpha1.Application, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.apps[app.Name] = app
	s.watch.Add(app)
	return app.DeepCopy(), nil
}

func (s staticAppClientSet) Update(app *v1alpha1.Application) (*v1alpha1.Application, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.apps[app.Name] = app
	s.watch.Modify(app)
	return app.DeepCopy(), nil
}

func (s staticAppClientSet) Delete(name string, options *v1.DeleteOptions) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if app, ok := s.apps[name]; ok {
		delete(s.apps, name)
		s.watch.Delete(app)
	}
	return nil
}

func (s staticAppClientSet) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return fmt.Errorf("not implemented")
}

func (s staticAppClientSet) Get(name string, options v1.GetOptions) (*v1alpha1.Application, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if app, ok := s.apps[name]; ok {
		return app, nil
	}
	return nil, errors.NotFound(fmt.Sprintf("applicaition %s not found", name))
}

func (s staticAppClientSet) List(opts v1.ListOptions) (*v1alpha1.ApplicationList, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	res := v1alpha1.ApplicationList{}
	for _, app := range s.apps {
		res.Items = append(res.Items, *app)
	}
	return &res, nil
}

func (s staticAppClientSet) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return s.watch, nil
}

func (s staticAppClientSet) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Application, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	app, ok := s.apps[name]
	if !ok {
		return app, errors.NotFound(fmt.Sprintf("applicaition %s not found", name))
	}

	origBytes, err := json.Marshal(app)
	if err != nil {
		return nil, err
	}

	newAppData, err := strategicpatch.StrategicMergePatch(origBytes, data, app)
	if err != nil {
		return nil, err
	}
	updatedApp := &v1alpha1.Application{}
	err = json.Unmarshal(newAppData, updatedApp)
	if err != nil {
		return nil, err
	}
	s.apps[app.Name] = updatedApp
	s.watch.Modify(updatedApp)
	return updatedApp, nil
}

func (s staticAppClientSet) RESTClient() rest.Interface {
	return nil
}

func (s staticAppClientSet) AppProjects(namespace string) alpha1.AppProjectInterface {
	return s.projects
}

func (s staticAppClientSet) Applications(namespace string) alpha1.ApplicationInterface {
	return s
}

func (s staticAppClientSet) Discovery() discovery.DiscoveryInterface {
	return &fake.FakeDiscovery{}
}

func (s staticAppClientSet) ArgoprojV1alpha1() alpha1.ArgoprojV1alpha1Interface {
	return s
}

func NewStaticAppClientSet(project v1alpha1.AppProject, applications ...v1alpha1.Application) versioned.Interface {
	apps := make(map[string]*v1alpha1.Application)
	for i := range applications {
		app := applications[i]
		apps[app.Name] = &app
	}
	return &staticAppClientSet{
		apps:     apps,
		projects: appsfake.NewSimpleClientset(&project).ArgoprojV1alpha1().AppProjects(project.Namespace),
		lock:     &sync.Mutex{},
		watch:    watch.NewRaceFreeFake(),
	}
}

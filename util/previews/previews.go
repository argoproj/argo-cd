package previews

import (
	"strings"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	alpha1 "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
)

const previewNamespace = "preview"

type PreviewService struct {
	appClientset  appclientset.Interface
	statusService StatusService
}

func NewPreviewService(appClientSet appclientset.Interface, statusService StatusService) PreviewService {
	return PreviewService{
		appClientset:  appClientSet,
		statusService: statusService,
	}
}

func (p PreviewService) Exists(previewAppName, revision string) (bool, error) {
	found, err := p.getApplications().Get(previewAppName, v1.GetOptions{})

	if err != nil && strings.HasSuffix(err.Error(), "not found") {
		return false, nil
	}

	if found == nil {
		return false, err
	}

	return true, nil
}

func (p PreviewService) Create(app v1alpha1.Application, preview v1alpha1.Preview, sha string) error {

	previewApp := previewApp(app, preview)

	exists, err := p.Exists(previewApp.Name, preview.Revision)
	if err != nil {
		return err
	}

	if exists {
		log.Infof("not creating an existing appName=%s", previewApp.Name)
	} else {
		log.Infof("creating appName=%v", previewApp.Name)

		_, err = p.getApplications().Create(previewApp)
		if err != nil {
			return err
		}
	}

	return p.statusService.SetStatus(*previewApp, sha, "pending")
}

func (p PreviewService) Delete(app v1alpha1.Application, preview v1alpha1.Preview) error {
	previewApp := previewApp(app, preview)

	exists, err := p.Exists(previewApp.Name, preview.Revision)
	if err != nil {
		return err
	}

	if exists {
		log.Infof("not deleting non-existent appName=%s", previewApp.Name)
	} else {
		applications := p.getApplications()

		log.Infof("deleting appName=%v", previewApp.Name)

		return applications.Delete(previewApp.Name, &v1.DeleteOptions{})
	}

	return nil
}

func (p PreviewService) getApplications() alpha1.ApplicationInterface {
	return p.appClientset.ArgoprojV1alpha1().Applications("argocd")
}

func previewApp(app v1alpha1.Application, preview v1alpha1.Preview) *v1alpha1.Application {
	previewApp := &v1alpha1.Application{}

	previewApp.Name = app.Name + "-preview-" + preview.Revision

	previewApp.Spec = *app.Spec.DeepCopy()

	previewApp.Spec.Preview = preview

	previewApp.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{Prune: true}}

	previewApp.Spec.Source.TargetRevision = preview.Revision

	previewApp.Spec.Destination.Namespace = previewNamespace
	previewApp.Spec.Destination.Server = app.Spec.Destination.Server

	return previewApp
}

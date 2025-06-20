package generator

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/argoproj/argo-cd/v3/util/settings"

	"github.com/argoproj/argo-cd/v3/util/db"

	"github.com/argoproj/argo-cd/v3/hack/gen-resources/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
)

type ApplicationGenerator struct {
	argoClientSet *appclientset.Clientset
	clientSet     *kubernetes.Clientset
}

func NewApplicationGenerator(argoClientSet *appclientset.Clientset, clientSet *kubernetes.Clientset) Generator {
	return &ApplicationGenerator{argoClientSet, clientSet}
}

func (generator *ApplicationGenerator) buildRandomSource(repositories []*v1alpha1.Repository) (*v1alpha1.ApplicationSource, error) {
	seed := rand.New(rand.NewSource(time.Now().Unix()))
	repoNumber := seed.Int() % len(repositories)
	return &v1alpha1.ApplicationSource{
		RepoURL:        repositories[repoNumber].Repo,
		Path:           "helm-guestbook",
		TargetRevision: "master",
	}, nil
}

func (generator *ApplicationGenerator) buildSource(opts *util.GenerateOpts, repositories []*v1alpha1.Repository) (*v1alpha1.ApplicationSource, error) {
	if opts.ApplicationOpts.SourceOpts.Strategy == "Random" {
		return generator.buildRandomSource(repositories)
	}
	return generator.buildRandomSource(repositories)
}

func (generator *ApplicationGenerator) buildRandomDestination(opts *util.GenerateOpts, clusters []v1alpha1.Cluster) (*v1alpha1.ApplicationDestination, error) {
	seed := rand.New(rand.NewSource(time.Now().Unix()))
	clusterNumber := seed.Int() % len(clusters)
	return &v1alpha1.ApplicationDestination{
		Namespace: opts.Namespace,
		Name:      clusters[clusterNumber].Name,
	}, nil
}

func (generator *ApplicationGenerator) buildDestination(opts *util.GenerateOpts, clusters []v1alpha1.Cluster) (*v1alpha1.ApplicationDestination, error) {
	if opts.ApplicationOpts.DestinationOpts.Strategy == "Random" {
		return generator.buildRandomDestination(opts, clusters)
	}
	return generator.buildRandomDestination(opts, clusters)
}

func (generator *ApplicationGenerator) Generate(opts *util.GenerateOpts) error {
	settingsMgr := settings.NewSettingsManager(context.TODO(), generator.clientSet, opts.Namespace)
	repositories, err := db.NewDB(opts.Namespace, settingsMgr, generator.clientSet).ListRepositories(context.TODO())
	if err != nil {
		return err
	}
	clusters, err := db.NewDB(opts.Namespace, settingsMgr, generator.clientSet).ListClusters(context.TODO())
	if err != nil {
		return err
	}
	applications := generator.argoClientSet.ArgoprojV1alpha1().Applications(opts.Namespace)
	for i := 0; i < opts.ApplicationOpts.Samples; i++ {
		log.Printf("Generate application #%v", i)
		source, err := generator.buildSource(opts, repositories)
		if err != nil {
			return err
		}
		log.Printf("Pick source %q", source)
		destination, err := generator.buildDestination(opts, clusters.Items)
		if err != nil {
			return err
		}
		log.Printf("Pick destination %q", destination)
		log.Printf("Create application")
		_, err = applications.Create(context.TODO(), &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "application-",
				Namespace:    opts.Namespace,
				Labels:       labels,
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Destination: *destination,
				Source:      source,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (generator *ApplicationGenerator) Clean(opts *util.GenerateOpts) error {
	log.Printf("Clean applications")
	applications := generator.argoClientSet.ArgoprojV1alpha1().Applications(opts.Namespace)
	return applications.DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-generator",
	})
}

package generator

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/argoproj/argo-cd/v2/util/settings"

	"github.com/argoproj/argo-cd/v2/util/db"

	"github.com/argoproj/argo-cd/v2/hack/gen-resources/util"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
)

type ApplicationGenerator struct {
	argoClientSet *appclientset.Clientset
	clientSet     *kubernetes.Clientset
	db            db.ArgoDB
}

func NewApplicationGenerator(argoClientSet *appclientset.Clientset, clientSet *kubernetes.Clientset, db db.ArgoDB) Generator {
	return &ApplicationGenerator{argoClientSet, clientSet, db}
}

func (pg *ApplicationGenerator) buildRandomSource(repositories []*v1alpha1.Repository) (*v1alpha1.ApplicationSource, error) {
	rand.Seed(time.Now().Unix())
	repoNumber := rand.Int() % len(repositories)
	return &v1alpha1.ApplicationSource{
		RepoURL:        repositories[repoNumber].Repo,
		Path:           "helm-guestbook",
		TargetRevision: "master",
	}, nil
}

func (ag *ApplicationGenerator) buildSource(opts *util.GenerateOpts, repositories []*v1alpha1.Repository) (*v1alpha1.ApplicationSource, error) {
	switch opts.ApplicationOpts.SourceOpts.Strategy {
	case "Random":
		return ag.buildRandomSource(repositories)
	}
	return ag.buildRandomSource(repositories)
}

func (pg *ApplicationGenerator) buildRandomDestination(opts *util.GenerateOpts, clusters []v1alpha1.Cluster) (*v1alpha1.ApplicationDestination, error) {
	rand.Seed(time.Now().Unix())
	clusterNumber := rand.Int() % len(clusters)
	return &v1alpha1.ApplicationDestination{
		Namespace: opts.Namespace,
		Name:      clusters[clusterNumber].Name,
	}, nil
}

func (ag *ApplicationGenerator) buildDestination(opts *util.GenerateOpts, clusters []v1alpha1.Cluster) (*v1alpha1.ApplicationDestination, error) {
	switch opts.ApplicationOpts.DestinationOpts.Strategy {
	case "Random":
		return ag.buildRandomDestination(opts, clusters)
	}
	return ag.buildRandomDestination(opts, clusters)
}

func (pg *ApplicationGenerator) Generate(opts *util.GenerateOpts) error {
	settingsMgr := settings.NewSettingsManager(context.TODO(), pg.clientSet, opts.Namespace)
	repositories, err := db.NewDB(opts.Namespace, settingsMgr, pg.clientSet).ListRepositories(context.TODO())
	if err != nil {
		return err
	}
	clusters, err := db.NewDB(opts.Namespace, settingsMgr, pg.clientSet).ListClusters(context.TODO())
	if err != nil {
		return err
	}
	applications := pg.argoClientSet.ArgoprojV1alpha1().Applications(opts.Namespace)
	for i := 0; i < opts.ApplicationOpts.Samples; i++ {
		log.Printf("Generate application #%v", i)
		source, err := pg.buildSource(opts, repositories)
		if err != nil {
			return err
		}
		log.Printf("Pick source \"%s\"", source)
		destination, err := pg.buildDestination(opts, clusters.Items)
		if err != nil {
			return err
		}
		log.Printf("Pick destination \"%s\"", destination)
		log.Printf("Create application")
		_, err = applications.Create(context.TODO(), &v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "application-",
				Namespace:    opts.Namespace,
				Labels:       labels,
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Destination: *destination,
				Source:      *source,
			},
		}, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ag *ApplicationGenerator) Clean(opts *util.GenerateOpts) error {
	log.Printf("Clean applications")
	applications := ag.argoClientSet.ArgoprojV1alpha1().Applications(opts.Namespace)
	return applications.DeleteCollection(context.TODO(), v1.DeleteOptions{}, v1.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-generator",
	})
}

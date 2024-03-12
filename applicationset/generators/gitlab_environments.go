package generators

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	environment "github.com/argoproj/argo-cd/v2/applicationset/services/gitlab_environments"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var _ Generator = (*GitlabEnvironmentGenerator)(nil)

const (
	DefaultEnvironmentRequeueAfterSeconds = 30 * time.Minute
)

type GitlabEnvironmentGenerator struct {
	client                 client.Client
	getServiceProviderFunc func(context.Context, *argoprojiov1alpha1.GitlabEnvironmentGenerator, *argoprojiov1alpha1.ApplicationSet) (environment.EnvironmentService, error)
	auth                   SCMAuthProviders
	scmRootCAPath          string
}

func NewGitlabEnvironmentGenerator(client client.Client, auth SCMAuthProviders, scmRootCAPath string) Generator {
	g := &GitlabEnvironmentGenerator{
		client:        client,
		auth:          auth,
		scmRootCAPath: scmRootCAPath,
	}
	g.getServiceProviderFunc = g.getServiceProvider
	return g
}

func (g *GitlabEnvironmentGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	// Return a requeue default of 30 minutes, if no default is specified.

	if appSetGenerator.GitlabEnvironment.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.GitlabEnvironment.RequeueAfterSeconds) * time.Second
	}

	return DefaultEnvironmentRequeueAfterSeconds
}

func (g *GitlabEnvironmentGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.GitlabEnvironment.Template
}

func (g *GitlabEnvironmentGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if appSetGenerator.GitlabEnvironment == nil {
		return nil, EmptyAppSetGeneratorError
	}

	ctx := context.Background()
	svc, _ := g.getServiceProviderFunc(ctx, appSetGenerator.GitlabEnvironment, applicationSetInfo)

	environments, err := svc.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing environments: %v", err)
	}
	params := make([]map[string]interface{}, 0, len(environments))

	for _, env := range environments {
		paramMap := map[string]interface{}{
			"id":               env.ID,
			"name":             env.Name,
			"external_url":     env.ExternalURL,
			"environment_slug": env.Slug,
			"state":            env.State,
			"tier":             env.Tier,
		}

		params = append(params, paramMap)
	}
	return params, nil
}

// getSecretRef gets the value of the key for the specified Secret resource.
func (g *GitlabEnvironmentGenerator) getSecretRef(ctx context.Context, ref *argoprojiov1alpha1.SecretRef, namespace string) (string, error) {
	if ref == nil {
		return "", nil
	}

	secret := &corev1.Secret{}
	err := g.client.Get(
		ctx,
		client.ObjectKey{
			Name:      ref.SecretName,
			Namespace: namespace,
		},
		secret)
	if err != nil {
		return "", fmt.Errorf("error fetching secret %s/%s: %v", namespace, ref.SecretName, err)
	}
	tokenBytes, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q in secret %s/%s not found", ref.Key, namespace, ref.SecretName)
	}
	return string(tokenBytes), nil
}

// selectServiceProvider selects the provider to get environments from the configuration
func (g *GitlabEnvironmentGenerator) getServiceProvider(ctx context.Context, generatorConfig *argoprojiov1alpha1.GitlabEnvironmentGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) (environment.EnvironmentService, error) {
	token, err := g.getSecretRef(ctx, generatorConfig.TokenRef, applicationSetInfo.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error fetching Secret token: %v", err)
	}
	return environment.NewGitLabService(ctx, token, generatorConfig.API, generatorConfig.Project, generatorConfig.EnvironmentState, g.scmRootCAPath, generatorConfig.Insecure)
}

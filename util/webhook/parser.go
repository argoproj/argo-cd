package webhook

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

// WebhookProvider identifies the SCM provider that sent a webhook payload.
type WebhookProvider string

const (
	WebhookProviderAzureDevOps     WebhookProvider = "azuredevops"
	WebhookProviderBitbucket       WebhookProvider = "bitbucket"
	WebhookProviderBitbucketServer WebhookProvider = "bitbucketserver"
	WebhookProviderGitHub          WebhookProvider = "github"
	WebhookProviderGitLab          WebhookProvider = "gitlab"
	WebhookProviderGogs            WebhookProvider = "gogs"
	WebhookProviderGHCR            WebhookProvider = "ghcr"
)

// WebhookConsumer selects the events supported by each webhook consumer.
type WebhookConsumer string

const (
	WebhookConsumerApplication    WebhookConsumer = "application"
	WebhookConsumerApplicationSet WebhookConsumer = "applicationset"
)

type providerFactory struct {
	name      WebhookProvider
	consumers []WebhookConsumer
	new       func() (ProviderParser, error)
}

func defaultProviderFactories(settings *settings.ArgoCDSettings) []providerFactory {
	return []providerFactory{
		{
			name:      WebhookProviderAzureDevOps,
			consumers: []WebhookConsumer{WebhookConsumerApplication, WebhookConsumerApplicationSet},
			new: func() (ProviderParser, error) {
				parser, err := azuredevops.New(azuredevops.Options.BasicAuth(settings.GetWebhookAzureDevOpsUsername(), settings.GetWebhookAzureDevOpsPassword()))
				return &azureDevOpsParser{webhook: parser}, err
			},
		},
		{
			name:      WebhookProviderGogs,
			consumers: []WebhookConsumer{WebhookConsumerApplication},
			new: func() (ProviderParser, error) {
				parser, err := gogs.New(gogs.Options.Secret(settings.GetWebhookGogsSecret()))
				return &gogsParser{webhook: parser}, err
			},
		},
		{
			name:      WebhookProviderGitHub,
			consumers: []WebhookConsumer{WebhookConsumerApplication, WebhookConsumerApplicationSet},
			new: func() (ProviderParser, error) {
				parser, err := github.New(github.Options.Secret(settings.GetWebhookGitHubSecret()))
				return &githubParser{webhook: parser}, err
			},
		},
		{
			name:      WebhookProviderGitLab,
			consumers: []WebhookConsumer{WebhookConsumerApplication, WebhookConsumerApplicationSet},
			new: func() (ProviderParser, error) {
				parser, err := gitlab.New(gitlab.Options.Secret(settings.GetWebhookGitLabSecret()))
				return &gitlabParser{webhook: parser}, err
			},
		},
		{
			name:      WebhookProviderBitbucket,
			consumers: []WebhookConsumer{WebhookConsumerApplication},
			new: func() (ProviderParser, error) {
				parser, err := bitbucket.New(bitbucket.Options.UUID(settings.GetWebhookBitbucketUUID()))
				return &bitbucketParser{webhook: parser}, err
			},
		},
		{
			name:      WebhookProviderBitbucketServer,
			consumers: []WebhookConsumer{WebhookConsumerApplication},
			new: func() (ProviderParser, error) {
				parser, err := bitbucketserver.New(bitbucketserver.Options.Secret(settings.GetWebhookBitbucketServerSecret()))
				return &bitbucketServerParser{webhook: parser}, err
			},
		},
		{
			name:      WebhookProviderGHCR,
			consumers: []WebhookConsumer{WebhookConsumerApplication},
			new: func() (ProviderParser, error) {
				return newGHCRParser(settings.GetWebhookGitHubSecret()), nil
			},
		},
	}
}

// NewProviderParsers constructs the providers supported by consumer. Healthy
// providers are returned even when another provider fails to initialize.
func NewProviderParsers(settings *settings.ArgoCDSettings, consumer WebhookConsumer) ([]ProviderParser, error) {
	return newProviderParsers(consumer, defaultProviderFactories(settings))
}

func newProviderParsers(consumer WebhookConsumer, factories []providerFactory) ([]ProviderParser, error) {
	if consumer != WebhookConsumerApplication && consumer != WebhookConsumerApplicationSet {
		return nil, fmt.Errorf("unsupported webhook consumer: %s", consumer)
	}

	parsers := make([]ProviderParser, 0, len(factories))
	var initErrors []error
	for _, factory := range factories {
		if !slices.Contains(factory.consumers, consumer) {
			continue
		}
		parser, err := factory.new()
		if err != nil {
			initErrors = append(initErrors, fmt.Errorf("unable to initialize %s webhook parser: %w", factory.name, err))
			continue
		}
		parsers = append(parsers, parser)
	}
	return parsers, errors.Join(initErrors...)
}

// Dispatch selects the first provider that recognizes r and delegates parsing.
func Dispatch(parsers []ProviderParser, r *http.Request, consumer WebhookConsumer) (any, WebhookProvider, error) {
	for _, parser := range parsers {
		if parser.CanHandle(r) {
			payload, err := parser.Parse(r, consumer)
			return payload, parser.Name(), err
		}
	}
	return nil, "", nil
}

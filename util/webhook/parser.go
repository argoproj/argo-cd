package webhook

import (
	"fmt"
	"net/http"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"
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

// WebhookSettings supplies the secrets used to validate incoming webhook payloads.
type WebhookSettings interface {
	GetWebhookGitHubSecret() string
	GetWebhookGitLabSecret() string
	GetWebhookBitbucketUUID() string
	GetWebhookBitbucketServerSecret() string
	GetWebhookGogsSecret() string
	GetWebhookAzureDevOpsUsername() string
	GetWebhookAzureDevOpsPassword() string
}

// PayloadParser owns provider webhook parsers and dispatches requests according
// to their provider headers.
type PayloadParser struct {
	github          *github.Webhook
	gitlab          *gitlab.Webhook
	bitbucket       *bitbucket.Webhook
	bitbucketserver *bitbucketserver.Webhook
	azuredevops     *azuredevops.Webhook
	gogs            *gogs.Webhook
	ghcr            *ghcrParser
}

// NewPayloadParser constructs all provider webhook parsers from Argo CD settings.
func NewPayloadParser(settings WebhookSettings) (*PayloadParser, error) {
	githubWebhook, err := github.New(github.Options.Secret(settings.GetWebhookGitHubSecret()))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize GitHub webhook parser: %w", err)
	}
	gitlabWebhook, err := gitlab.New(gitlab.Options.Secret(settings.GetWebhookGitLabSecret()))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize GitLab webhook parser: %w", err)
	}
	bitbucketWebhook, err := bitbucket.New(bitbucket.Options.UUID(settings.GetWebhookBitbucketUUID()))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Bitbucket webhook parser: %w", err)
	}
	bitbucketServerWebhook, err := bitbucketserver.New(bitbucketserver.Options.Secret(settings.GetWebhookBitbucketServerSecret()))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Bitbucket Server webhook parser: %w", err)
	}
	gogsWebhook, err := gogs.New(gogs.Options.Secret(settings.GetWebhookGogsSecret()))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Gogs webhook parser: %w", err)
	}
	azureDevOpsWebhook, err := azuredevops.New(azuredevops.Options.BasicAuth(settings.GetWebhookAzureDevOpsUsername(), settings.GetWebhookAzureDevOpsPassword()))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Azure DevOps webhook parser: %w", err)
	}

	return &PayloadParser{
		github:          githubWebhook,
		gitlab:          gitlabWebhook,
		bitbucket:       bitbucketWebhook,
		bitbucketserver: bitbucketServerWebhook,
		azuredevops:     azureDevOpsWebhook,
		gogs:            gogsWebhook,
		ghcr:            newGHCRParser(settings.GetWebhookGitHubSecret()),
	}, nil
}

// Parse detects the provider from request headers and parses the events supported
// by the given consumer. It returns an empty provider for unknown events.
func (p *PayloadParser) Parse(r *http.Request, consumer WebhookConsumer) (any, WebhookProvider, error) {
	switch {
	case consumer == WebhookConsumerApplication && p.ghcr.CanHandle(r):
		payload, err := p.ghcr.Parse(r)
		return payload, WebhookProviderGHCR, err
	case r.Header.Get("X-Vss-Activityid") != "":
		if consumer == WebhookConsumerApplicationSet {
			payload, err := p.azuredevops.Parse(r, azuredevops.GitPushEventType, azuredevops.GitPullRequestCreatedEventType, azuredevops.GitPullRequestUpdatedEventType, azuredevops.GitPullRequestMergedEventType)
			return payload, WebhookProviderAzureDevOps, err
		}
		payload, err := p.azuredevops.Parse(r, azuredevops.GitPushEventType)
		return payload, WebhookProviderAzureDevOps, err
	// Gogs carries incompatible GitHub headers, so it must be detected first.
	case r.Header.Get("X-Gogs-Event") != "" && consumer == WebhookConsumerApplication:
		payload, err := p.gogs.Parse(r, gogs.PushEvent)
		return payload, WebhookProviderGogs, err
	case r.Header.Get("X-GitHub-Event") != "":
		if consumer == WebhookConsumerApplicationSet {
			payload, err := p.github.Parse(r, github.PushEvent, github.PullRequestEvent, github.PingEvent)
			return payload, WebhookProviderGitHub, err
		}
		if consumer != WebhookConsumerApplication {
			return nil, "", nil
		}
		payload, err := p.github.Parse(r, github.PushEvent, github.PingEvent)
		return payload, WebhookProviderGitHub, err
	case r.Header.Get("X-Gitlab-Event") != "":
		if consumer == WebhookConsumerApplicationSet {
			payload, err := p.gitlab.Parse(r, gitlab.PushEvents, gitlab.TagEvents, gitlab.MergeRequestEvents, gitlab.SystemHookEvents)
			return payload, WebhookProviderGitLab, err
		}
		if consumer != WebhookConsumerApplication {
			return nil, "", nil
		}
		payload, err := p.gitlab.Parse(r, gitlab.PushEvents, gitlab.TagEvents, gitlab.SystemHookEvents)
		return payload, WebhookProviderGitLab, err
	case r.Header.Get("X-Hook-UUID") != "" && consumer == WebhookConsumerApplication:
		payload, err := p.bitbucket.Parse(r, bitbucket.RepoPushEvent)
		return payload, WebhookProviderBitbucket, err
	case r.Header.Get("X-Event-Key") != "" && consumer == WebhookConsumerApplication:
		payload, err := p.bitbucketserver.Parse(r, bitbucketserver.RepositoryReferenceChangedEvent, bitbucketserver.DiagnosticsPingEvent)
		return payload, WebhookProviderBitbucketServer, err
	default:
		return nil, "", nil
	}
}

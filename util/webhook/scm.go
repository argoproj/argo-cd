package webhook

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
)

type azureDevOpsParser struct {
	webhook *azuredevops.Webhook
}

func (p *azureDevOpsParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Vss-Activityid") != ""
}

func (p *azureDevOpsParser) Name() WebhookProvider {
	return WebhookProviderAzureDevOps
}

func (p *azureDevOpsParser) Parse(r *http.Request, consumer WebhookConsumer) (any, error) {
	var payload any
	var err error
	switch consumer {
	case WebhookConsumerApplication:
		payload, err = p.webhook.Parse(r, azuredevops.GitPushEventType)
	case WebhookConsumerApplicationSet:
		payload, err = p.webhook.Parse(r, azuredevops.GitPushEventType, azuredevops.GitPullRequestCreatedEventType, azuredevops.GitPullRequestUpdatedEventType, azuredevops.GitPullRequestMergedEventType)
	default:
		return nil, fmt.Errorf("unsupported webhook consumer: %s", consumer)
	}
	if errors.Is(err, azuredevops.ErrBasicAuthVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("Azure DevOps webhook basic auth verification failed")
	}
	return payload, err
}

// gogsParser must be evaluated before githubParser: Gogs requests carry both
// Gogs and (incompatible) GitHub headers.
type gogsParser struct {
	webhook *gogs.Webhook
}

func (p *gogsParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Gogs-Event") != ""
}

func (p *gogsParser) Name() WebhookProvider {
	return WebhookProviderGogs
}

func (p *gogsParser) Parse(r *http.Request, consumer WebhookConsumer) (any, error) {
	if consumer != WebhookConsumerApplication {
		return nil, fmt.Errorf("unsupported webhook consumer: %s", consumer)
	}
	payload, err := p.webhook.Parse(r, gogs.PushEvent)
	if errors.Is(err, gogs.ErrHMACVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("Gogs webhook HMAC verification failed")
	}
	return payload, err
}

type githubParser struct {
	webhook *github.Webhook
}

func (p *githubParser) CanHandle(r *http.Request) bool {
	event := r.Header.Get("X-GitHub-Event")
	if r.Header.Get("X-Gogs-Event") != "" {
		return false
	}
	// "package" is delivered via the same X-GitHub-Event header but is owned by
	// ghcrParser. Excluding it here keeps the parser order independent.
	return event != "" && event != "package"
}

func (p *githubParser) Name() WebhookProvider {
	return WebhookProviderGitHub
}

func (p *githubParser) Parse(r *http.Request, consumer WebhookConsumer) (any, error) {
	var payload any
	var err error
	switch consumer {
	case WebhookConsumerApplication:
		payload, err = p.webhook.Parse(r, github.PushEvent, github.PingEvent)
	case WebhookConsumerApplicationSet:
		payload, err = p.webhook.Parse(r, github.PushEvent, github.PullRequestEvent, github.PingEvent)
	default:
		return nil, fmt.Errorf("unsupported webhook consumer: %s", consumer)
	}
	if errors.Is(err, github.ErrHMACVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitHub webhook HMAC verification failed")
	}
	return payload, err
}

type gitlabParser struct {
	webhook *gitlab.Webhook
}

func (p *gitlabParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Gitlab-Event") != ""
}

func (p *gitlabParser) Name() WebhookProvider {
	return WebhookProviderGitLab
}

func (p *gitlabParser) Parse(r *http.Request, consumer WebhookConsumer) (any, error) {
	var payload any
	var err error
	switch consumer {
	case WebhookConsumerApplication:
		payload, err = p.webhook.Parse(r, gitlab.PushEvents, gitlab.TagEvents, gitlab.SystemHookEvents)
	case WebhookConsumerApplicationSet:
		payload, err = p.webhook.Parse(r, gitlab.PushEvents, gitlab.TagEvents, gitlab.MergeRequestEvents, gitlab.SystemHookEvents)
	default:
		return nil, fmt.Errorf("unsupported webhook consumer: %s", consumer)
	}
	if errors.Is(err, gitlab.ErrGitLabTokenVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab webhook token verification failed")
	}
	return payload, err
}

type bitbucketParser struct {
	webhook *bitbucket.Webhook
}

func (p *bitbucketParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Hook-UUID") != ""
}

func (p *bitbucketParser) Name() WebhookProvider {
	return WebhookProviderBitbucket
}

func (p *bitbucketParser) Parse(r *http.Request, consumer WebhookConsumer) (any, error) {
	if consumer != WebhookConsumerApplication {
		return nil, fmt.Errorf("unsupported webhook consumer: %s", consumer)
	}
	payload, err := p.webhook.Parse(r, bitbucket.RepoPushEvent)
	if errors.Is(err, bitbucket.ErrUUIDVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook UUID verification failed")
	}
	return payload, err
}

type bitbucketServerParser struct {
	webhook *bitbucketserver.Webhook
}

func (p *bitbucketServerParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Event-Key") != ""
}

func (p *bitbucketServerParser) Name() WebhookProvider {
	return WebhookProviderBitbucketServer
}

func (p *bitbucketServerParser) Parse(r *http.Request, consumer WebhookConsumer) (any, error) {
	if consumer != WebhookConsumerApplication {
		return nil, fmt.Errorf("unsupported webhook consumer: %s", consumer)
	}
	payload, err := p.webhook.Parse(r, bitbucketserver.RepositoryReferenceChangedEvent, bitbucketserver.DiagnosticsPingEvent)
	if errors.Is(err, bitbucketserver.ErrHMACVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook HMAC verification failed")
	}
	return payload, err
}

package webhook

import (
	"errors"
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

// Extractor dispatches a webhook request to the matching provider.
//
// CanHandle inspects request headers to decide whether this parser owns the
// request. Parse extracts the provider-specific payload. On verification
// failures it either emits a security-audit log line directly or returns a
// known sentinel error (e.g. ErrHMACVerificationFailed) that the caller logs
// centrally. A (nil, nil) return signals a request that was claimed but
// intentionally skipped (e.g. an unsupported sub-event).
type Extractor interface {
	CanHandle(r *http.Request) bool
	Parse(r *http.Request) (any, error)
}

type azureDevOpsParser struct {
	webhook *azuredevops.Webhook
}

func (p *azureDevOpsParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Vss-Activityid") != ""
}

func (p *azureDevOpsParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, azuredevops.GitPushEventType)
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

func (p *gogsParser) Parse(r *http.Request) (any, error) {
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
	// "package" is delivered via the same X-GitHub-Event header but is owned by
	// ghcrParser. Excluding it here keeps the parser order independent.
	return event != "" && event != "package"
}

func (p *githubParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, github.PushEvent, github.PingEvent)
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

func (p *gitlabParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, gitlab.PushEvents, gitlab.TagEvents, gitlab.SystemHookEvents)
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

func (p *bitbucketParser) Parse(r *http.Request) (any, error) {
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

func (p *bitbucketServerParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, bitbucketserver.RepositoryReferenceChangedEvent, bitbucketserver.DiagnosticsPingEvent)
	if errors.Is(err, bitbucketserver.ErrHMACVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook HMAC verification failed")
	}
	return payload, err
}

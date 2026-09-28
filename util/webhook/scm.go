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

	"github.com/argoproj/argo-cd/v3/util/settings"

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

// ParserOptions selects which providers a webhook handler accepts and which
// event types each SCM provider parses. An SCM provider with no events is not
// registered, so its requests are rejected as unknown webhook events.
type ParserOptions struct {
	AzureDevOpsEvents     []azuredevops.Event
	GogsEvents            []gogs.Event
	GitHubEvents          []github.Event
	GitLabEvents          []gitlab.Event
	BitbucketEvents       []bitbucket.Event
	BitbucketServerEvents []bitbucketserver.Event
	Harbor                bool
	GHCR                  bool
	DockerHub             bool
}

// NewParsers builds the parsers enabled in opts, configured with the webhook
// secrets in set. Order matters: requests are dispatched to the first parser
// whose CanHandle matches.
func NewParsers(set *settings.ArgoCDSettings, opts ParserOptions) []Extractor {
	var parsers []Extractor
	if len(opts.AzureDevOpsEvents) > 0 {
		if hook, err := azuredevops.New(azuredevops.Options.BasicAuth(set.GetWebhookAzureDevOpsUsername(), set.GetWebhookAzureDevOpsPassword())); err != nil {
			log.Warnf("Unable to init the Azure Devops webhook: %v", err)
		} else {
			parsers = append(parsers, &azureDevOpsParser{webhook: hook, events: opts.AzureDevOpsEvents})
		}
	}
	// Gogs needs to be checked before GitHub since it carries both Gogs and incompatible GitHub headers
	if len(opts.GogsEvents) > 0 {
		if hook, err := gogs.New(gogs.Options.Secret(set.GetWebhookGogsSecret())); err != nil {
			log.Warnf("Unable to init the Gogs webhook: %v", err)
		} else {
			parsers = append(parsers, &gogsParser{webhook: hook, events: opts.GogsEvents})
		}
	}
	// Check for GitHub events now
	if len(opts.GitHubEvents) > 0 {
		if hook, err := github.New(github.Options.Secret(set.GetWebhookGitHubSecret())); err != nil {
			log.Warnf("Unable to init the GitHub webhook: %v", err)
		} else {
			parsers = append(parsers, &githubParser{webhook: hook, events: opts.GitHubEvents})
		}
	}
	if len(opts.GitLabEvents) > 0 {
		if hook, err := gitlab.New(gitlab.Options.Secret(set.GetWebhookGitLabSecret())); err != nil {
			log.Warnf("Unable to init the GitLab webhook: %v", err)
		} else {
			parsers = append(parsers, &gitlabParser{webhook: hook, events: opts.GitLabEvents})
		}
	}
	if len(opts.BitbucketEvents) > 0 {
		if hook, err := bitbucket.New(bitbucket.Options.UUID(set.GetWebhookBitbucketUUID())); err != nil {
			log.Warnf("Unable to init the Bitbucket webhook: %v", err)
		} else {
			parsers = append(parsers, &bitbucketParser{webhook: hook, events: opts.BitbucketEvents})
		}
	}
	if len(opts.BitbucketServerEvents) > 0 {
		if hook, err := bitbucketserver.New(bitbucketserver.Options.Secret(set.GetWebhookGitHubSecret())); err != nil {
			log.Warnf("Unable to init the Bitbucket Server webhook: %v", err)
		} else {
			parsers = append(parsers, &bitbucketServerParser{webhook: hook, events: opts.BitbucketServerEvents})
		}
	}

	// check for OCI registries now
	if opts.Harbor {
		parsers = append(parsers, newHarborParser(set.GetWebhookHarborSecret()))
	}
	if opts.GHCR {
		parsers = append(parsers, NewGHCRParser(set.GetWebhookGitHubSecret()))
	}
	if opts.DockerHub {
		parsers = append(parsers, newDockerHubParser(set.GetWebhookDockerHubSecret()))
	}

	return parsers
}

type azureDevOpsParser struct {
	webhook *azuredevops.Webhook
	events  []azuredevops.Event
}

func (p *azureDevOpsParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Vss-Activityid") != ""
}

func (p *azureDevOpsParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, p.events...)
	if errors.Is(err, azuredevops.ErrBasicAuthVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("Azure DevOps webhook basic auth verification failed")
	}
	return payload, err
}

// gogsParser must be evaluated before githubParser: Gogs requests carry both
// Gogs and (incompatible) GitHub headers.
type gogsParser struct {
	webhook *gogs.Webhook
	events  []gogs.Event
}

func (p *gogsParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Gogs-Event") != ""
}

func (p *gogsParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, p.events...)
	if errors.Is(err, gogs.ErrHMACVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("Gogs webhook HMAC verification failed")
	}
	return payload, err
}

type githubParser struct {
	webhook *github.Webhook
	events  []github.Event
}

func (p *githubParser) CanHandle(r *http.Request) bool {
	event := r.Header.Get("X-GitHub-Event")
	// "package" is delivered via the same X-GitHub-Event header but is owned by
	// ghcrParser. Excluding it here keeps the parser order independent.
	return event != "" && event != "package"
}

func (p *githubParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, p.events...)
	if errors.Is(err, github.ErrHMACVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitHub webhook HMAC verification failed")
	}
	return payload, err
}

type gitlabParser struct {
	webhook *gitlab.Webhook
	events  []gitlab.Event
}

func (p *gitlabParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Gitlab-Event") != ""
}

func (p *gitlabParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, p.events...)
	if errors.Is(err, gitlab.ErrGitLabTokenVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab webhook token verification failed")
	}
	return payload, err
}

type bitbucketParser struct {
	webhook *bitbucket.Webhook
	events  []bitbucket.Event
}

func (p *bitbucketParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Hook-UUID") != ""
}

func (p *bitbucketParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, p.events...)
	if errors.Is(err, bitbucket.ErrUUIDVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook UUID verification failed")
	}
	return payload, err
}

type bitbucketServerParser struct {
	webhook *bitbucketserver.Webhook
	events  []bitbucketserver.Event
}

func (p *bitbucketServerParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Event-Key") != ""
}

func (p *bitbucketServerParser) Parse(r *http.Request) (any, error) {
	payload, err := p.webhook.Parse(r, p.events...)
	if errors.Is(err, bitbucketserver.ErrHMACVerificationFailed) {
		log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook HMAC verification failed")
	}
	return payload, err
}

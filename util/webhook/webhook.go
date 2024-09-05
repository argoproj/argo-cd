package webhook

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	azureDevOps "github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketServer "github.com/go-playground/webhooks/v6/bitbucket-server"
	gitHub "github.com/go-playground/webhooks/v6/github"
	gitLab "github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type WebhookPayloadHandler interface {
	HandlePayload(payload interface{})
}

type Webhook struct {
	sync.WaitGroup
	parallelism       int
	maxPayloadSize    int64
	argoCdSettingsMgr *settings.SettingsManager
	payloadHandler    WebhookPayloadHandler
	payloadQueue      chan interface{}
	gitHub            *gitHub.Webhook
	gitLab            *gitLab.Webhook
	bitbucket         *bitbucket.Webhook
	bitbucketServer   *bitbucketServer.Webhook
	azureDevOps       *azureDevOps.Webhook
	gogs              *gogs.Webhook
}

// https://www.rfc-editor.org/rfc/rfc3986#section-3.2.1
// https://github.com/shadow-maint/shadow/blob/master/libmisc/chkname.c#L36
const usernameRegex = `[a-zA-Z0-9_\.][a-zA-Z0-9_\.-]{0,30}[a-zA-Z0-9_\.\$-]?`

const payloadQueueSize = 50000

func NewWebhook(
	parallelism int,
	maxPayloadSize int64,
	argoCdSettingsMgr *settings.SettingsManager,
	payloadHandler WebhookPayloadHandler,
) (*Webhook, error) {
	argoCdSettings, err := argoCdSettingsMgr.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("Failed to get argocd settings: %w", err)
	}

	gitHubWebhook, err := gitHub.New(gitHub.Options.Secret(argoCdSettings.WebhookGitHubSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init the GitHub webhook: %w", err)
	}

	gitLabWebhook, err := gitLab.New(gitLab.Options.Secret(argoCdSettings.WebhookGitLabSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init the GitLab webhook: %w", err)
	}

	bitbucketWebhook, err := bitbucket.New(bitbucket.Options.UUID(argoCdSettings.WebhookBitbucketUUID))
	if err != nil {
		return nil, fmt.Errorf("Unable to init the Bitbucket webhook: %w", err)
	}

	bitbucketServerWebhook, err := bitbucketServer.New(bitbucketServer.Options.Secret(argoCdSettings.WebhookBitbucketServerSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init the Bitbucket Server webhook: %w", err)
	}

	gogsWebhook, err := gogs.New(gogs.Options.Secret(argoCdSettings.WebhookGogsSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init the Gogs webhook: %w", err)
	}

	azureDevOpsWebhook, err := azureDevOps.New(azureDevOps.Options.BasicAuth(argoCdSettings.WebhookAzureDevOpsUsername, argoCdSettings.WebhookAzureDevOpsPassword))
	if err != nil {
		return nil, fmt.Errorf("Unable to init the Azure DevOps webhook: %w", err)
	}

	webhook := Webhook{
		parallelism:       parallelism,
		maxPayloadSize:    maxPayloadSize,
		argoCdSettingsMgr: argoCdSettingsMgr,
		payloadHandler:    payloadHandler,
		payloadQueue:      make(chan interface{}, payloadQueueSize),
		gitHub:            gitHubWebhook,
		gitLab:            gitLabWebhook,
		bitbucket:         bitbucketWebhook,
		bitbucketServer:   bitbucketServerWebhook,
		azureDevOps:       azureDevOpsWebhook,
		gogs:              gogsWebhook,
	}

	webhook.startWorkerPool()

	return &webhook, nil
}

func (webhook *Webhook) startWorkerPool() {
	for i := 0; i < webhook.parallelism; i++ {
		webhook.Add(1)

		go func() {
			defer webhook.Done()

			for {
				payload, ok := <-webhook.payloadQueue

				if !ok {
					return
				}

				webhook.payloadHandler.HandlePayload(payload)
			}
		}()
	}
}

func getUrlRegex(originalUrl string, regexpFormat string) (*regexp.Regexp, error) {
	urlObj, err := url.Parse(originalUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repoURL '%s'", originalUrl)
	}

	regexEscapedHostname := regexp.QuoteMeta(urlObj.Hostname())
	regexEscapedPath := regexp.QuoteMeta(urlObj.EscapedPath()[1:]) + `(\.git)?`

	regexpStr := fmt.Sprintf(regexpFormat, usernameRegex, regexEscapedHostname, regexEscapedPath)

	repoRegexp, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp for repoURL '%s'", originalUrl)
	}

	return repoRegexp, nil
}

func GetWebUrlRegex(originalUrl string) (*regexp.Regexp, error) {
	return getUrlRegex(originalUrl, `(?i)^((https?|ssh)://)?(%[1]s@)?((alt)?ssh\.)?%[2]s(:[0-9]+)?[:/]%[3]s$`)
}

func GetApiUrlRegex(originalUrl string) (*regexp.Regexp, error) {
	return getUrlRegex(originalUrl, `(?i)^(https?://)?(%[1]s@)?%[2]s(:[0-9]+)?/?$`)
}

func (webhook *Webhook) HandleRequest(writer http.ResponseWriter, request *http.Request) {
	var payload interface{}
	var err error

	request.Body = http.MaxBytesReader(writer, request.Body, webhook.maxPayloadSize)

	switch {
	case request.Header.Get("X-Vss-Activityid") != "":
		payload, err = webhook.azureDevOps.Parse(request, azureDevOps.GitPushEventType, azureDevOps.GitPullRequestCreatedEventType, azureDevOps.GitPullRequestUpdatedEventType, azureDevOps.GitPullRequestMergedEventType)

		if errors.Is(err, azureDevOps.ErrBasicAuthVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("Azure DevOps webhook basic auth verification failed")
		}

	// Gogs needs to be checked before GitHub since it carries both Gogs and (incompatible) GitHub headers.
	case request.Header.Get("X-Gogs-Event") != "":
		payload, err = webhook.gogs.Parse(request, gogs.PushEvent)

		if errors.Is(err, gogs.ErrHMACVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("Gogs webhook HMAC verification failed")
		}

	case request.Header.Get("X-GitHub-Event") != "":
		payload, err = webhook.gitHub.Parse(request, gitHub.PushEvent, gitHub.PullRequestEvent, gitHub.PingEvent)

		if errors.Is(err, gitHub.ErrHMACVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitHub webhook HMAC verification failed")
		}

	case request.Header.Get("X-Gitlab-Event") != "":
		payload, err = webhook.gitLab.Parse(request, gitLab.PushEvents, gitLab.TagEvents, gitLab.MergeRequestEvents, gitLab.SystemHookEvents)

		if errors.Is(err, gitLab.ErrGitLabTokenVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab webhook token verification failed")
		}

	case request.Header.Get("X-Hook-UUID") != "":
		payload, err = webhook.bitbucket.Parse(request, bitbucket.RepoPushEvent)

		if errors.Is(err, bitbucket.ErrUUIDVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook UUID verification failed")
		}

	case request.Header.Get("X-Event-Key") != "":
		payload, err = webhook.bitbucketServer.Parse(request, bitbucketServer.RepositoryReferenceChangedEvent, bitbucketServer.DiagnosticsPingEvent)

		if errors.Is(err, bitbucketServer.ErrHMACVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook HMAC verification failed")
		}

	default:
		log.Debug("Ignoring unknown webhook event")
		http.Error(writer, "Unknown webhook event", http.StatusBadRequest)

		return
	}

	if err != nil {
		// If the error is due to a large payload, return a more user-friendly error message.
		if err.Error() == "error parsing payload" {
			msg := fmt.Sprintf("Webhook processing failed: The payload is either too large or corrupted. Please check the payload size (must be under %v MB) and ensure it is valid JSON", webhook.maxPayloadSize/1024/1024)

			log.WithField(common.SecurityField, common.SecurityHigh).Warn(msg)
			http.Error(writer, msg, http.StatusBadRequest)

			return
		}

		log.Infof("Webhook processing failed: %s", err)

		status := http.StatusBadRequest

		if request.Method != http.MethodPost {
			status = http.StatusMethodNotAllowed
		}

		http.Error(writer, fmt.Sprintf("Webhook processing failed: %s", html.EscapeString(err.Error())), status)

		return
	}

	select {
	case webhook.payloadQueue <- payload:
	default:
		log.Info("Queue is full, discarding webhook payload")
		http.Error(writer, "Queue is full, discarding webhook payload", http.StatusServiceUnavailable)
	}
}

func (webhook *Webhook) CloseAndWait() {
	close(webhook.payloadQueue)
	webhook.Wait()
}

func ParseRevision(ref string) string {
	refParts := strings.SplitN(ref, "/", 3)

	return refParts[len(refParts)-1]
}

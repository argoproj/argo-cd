package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/guard"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

var (
	ErrSignedWebhookVerificationFailed = errors.New("GitLab signed webhook verification failed")
	ErrMissingSignatureHeader          = errors.New("missing webhook-signature header")
	ErrMissingWebhookIDHeader          = errors.New("missing webhook-id header")
	ErrMissingWebhookTimestampHeader   = errors.New("missing webhook-timestamp header")
	ErrInvalidWebhookTimestamp         = errors.New("invalid webhook-timestamp header")
	ErrStaleWebhookTimestamp           = errors.New("stale webhook timestamp")
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
			log.Warnf("Unable to init the Azure DevOps webhook: %v", err)
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
	if len(opts.GitHubEvents) > 0 {
		if hook, err := github.New(github.Options.Secret(set.GetWebhookGitHubSecret())); err != nil {
			log.Warnf("Unable to init the GitHub webhook: %v", err)
		} else {
			parsers = append(parsers, &githubParser{webhook: hook, events: opts.GitHubEvents})
		}
	}
	if len(opts.GitLabEvents) > 0 {
		if hook, err := gitlab.New(); err != nil {
			log.Warnf("Unable to init the GitLab webhook: %v", err)
		} else {
			var secret, signingSecret string
			var tolerance time.Duration
			if set != nil {
				secret = set.GetWebhookGitLabSecret()
				signingSecret = set.GetWebhookGitLabSigningSecret()
				tolerance = set.GetWebhookGitLabTimestampTolerance()
			}
			parsers = append(parsers, &gitlabParser{
				webhook:       hook,
				events:        opts.GitLabEvents,
				secret:        secret,
				signingSecret: signingSecret,
				tolerance:     tolerance,
			})
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
		if hook, err := bitbucketserver.New(bitbucketserver.Options.Secret(set.GetWebhookBitbucketServerSecret())); err != nil {
			log.Warnf("Unable to init the Bitbucket Server webhook: %v", err)
		} else {
			parsers = append(parsers, &bitbucketServerParser{webhook: hook, events: opts.BitbucketServerEvents})
		}
	}

	// OCI Registries parsers
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

// Dispatch hands the request to the first parser that can handle it. The
// handled return is true when a parser claimed the request, regardless of
// whether parsing produced a payload or an error; callers use it to distinguish
// "unknown webhook event" (false) from "claimed but skipped" (true, nil, nil).
func Dispatch(parsers []Extractor, r *http.Request) (any, bool, error) {
	for _, p := range parsers {
		if p.CanHandle(r) {
			payload, err := p.Parse(r)
			return payload, true, err
		}
	}
	log.Debug("Ignoring unknown webhook event")
	return nil, false, nil
}

// HandleRequest parses a webhook request with the first parser that can handle
// it and queues the payload for the handler's workers, writing the HTTP
// response. Request bodies larger than maxPayloadSizeB are rejected.
func HandleRequest(w http.ResponseWriter, r *http.Request, parsers []Extractor, maxPayloadSizeB int64, queue chan<- any) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSizeB)
	payload, handled, err := Dispatch(parsers, r)
	if !handled {
		http.Error(w, "Unknown webhook event", http.StatusBadRequest)
		return
	}
	if err != nil {
		if errors.Is(err, ErrHMACVerificationFailed) || errors.Is(err, ErrSecretVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Info("Registry webhook authentication failed")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// If the error is due to a large payload, return a more user-friendly error message
		if isParsingPayloadError(err) {
			log.WithField(common.SecurityField, common.SecurityHigh).Warnf("Webhook processing failed: payload too large or corrupted (limit %v MB): %v", maxPayloadSizeB/1024/1024, err)
			http.Error(w, fmt.Sprintf("Webhook processing failed: payload must be valid JSON under %v MB", maxPayloadSizeB/1024/1024), http.StatusBadRequest)
			return
		}

		status := http.StatusBadRequest
		if r.Method != http.MethodPost {
			status = http.StatusMethodNotAllowed
		}
		log.Infof("Webhook processing failed: %v", err)
		http.Error(w, "Webhook processing failed", status)
		return
	}

	// Parser claimed the request but produced no payload (e.g. GHCR event that
	// was intentionally skipped). Acknowledge with 200 and skip the queue.
	if payload == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	select {
	case queue <- payload:
	default:
		log.Info("Queue is full, discarding webhook payload")
		http.Error(w, "Queue is full, discarding webhook payload", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// StartWorkers starts count goroutines, tracked by wg, that pass each payload
// read from queue to handle until the queue is closed. A panic in handle is
// logged with panicMsg and the worker moves on to the next payload.
func StartWorkers(wg *sync.WaitGroup, count int, queue <-chan any, handle func(any), component, panicMsg string) {
	compLog := log.WithField("component", component)
	for range count {
		wg.Go(func() {
			for payload := range queue {
				guard.RecoverAndLog(func() { handle(payload) }, compLog, panicMsg)
			}
		})
	}
}

// isParsingPayloadError returns a bool if the error is parsing payload error
func isParsingPayloadError(err error) bool {
	return errors.Is(err, github.ErrParsingPayload) ||
		errors.Is(err, gitlab.ErrParsingPayload) ||
		errors.Is(err, gogs.ErrParsingPayload) ||
		errors.Is(err, bitbucket.ErrParsingPayload) ||
		errors.Is(err, bitbucketserver.ErrParsingPayload) ||
		errors.Is(err, azuredevops.ErrParsingPayload)
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
	webhook       *gitlab.Webhook
	events        []gitlab.Event
	secret        string
	signingSecret string
	tolerance     time.Duration
}

func (p *gitlabParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-Gitlab-Event") != ""
}

func (p *gitlabParser) Parse(r *http.Request) (any, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	if p.signingSecret != "" {
		sigHeader := getHeader(r.Header, "webhook-signature", "Webhook-Signature")
		if sigHeader != "" {
			if err := p.verifySignature(r.Header, body); err != nil {
				log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab signed webhook verification failed: %v", err)
				return nil, err
			}
		} else {
			// If signing secret is enabled and signature header is missing, check if legacy secret can authenticate.
			// Only allow fallback to legacy secret if it is configured and does not start with whsec_.
			hasLegacySecret := p.secret != "" && !strings.HasPrefix(p.secret, "whsec_")
			if hasLegacySecret {
				token := r.Header.Get("X-Gitlab-Token")
				if subtle.ConstantTimeCompare([]byte(token), []byte(p.secret)) != 1 {
					log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab webhook token verification failed")
					return nil, gitlab.ErrGitLabTokenVerificationFailed
				}
			} else {
				log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab signed webhook verification failed: %v", ErrMissingSignatureHeader)
				return nil, ErrMissingSignatureHeader
			}
		}
	} else if p.secret != "" {
		token := r.Header.Get("X-Gitlab-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(p.secret)) != 1 {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab webhook token verification failed")
			return nil, gitlab.ErrGitLabTokenVerificationFailed
		}
	}

	return p.webhook.Parse(r, p.events...)
}

func (p *gitlabParser) verifySignature(h http.Header, body []byte) error {
	msgID := getHeader(h, "webhook-id", "Webhook-Id")
	if msgID == "" {
		return ErrMissingWebhookIDHeader
	}
	msgTimestamp := getHeader(h, "webhook-timestamp", "Webhook-Timestamp")
	if msgTimestamp == "" {
		return ErrMissingWebhookTimestampHeader
	}
	sigHeader := getHeader(h, "webhook-signature", "Webhook-Signature")
	if sigHeader == "" {
		return ErrMissingSignatureHeader
	}

	ts, err := strconv.ParseInt(msgTimestamp, 10, 64)
	if err != nil {
		return ErrInvalidWebhookTimestamp
	}
	webhookTime := time.Unix(ts, 0)
	tolerance := p.tolerance
	if tolerance <= 0 {
		tolerance = settings.DefaultWebhookGitLabTimestampTolerance
	}
	diff := time.Since(webhookTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		return ErrStaleWebhookTimestamp
	}

	key, err := parseSigningSecret(p.signingSecret)
	if err != nil {
		return fmt.Errorf("failed to parse signing secret: %w", err)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msgID))
	mac.Write([]byte("."))
	mac.Write([]byte(msgTimestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)

	signatures := strings.Fields(sigHeader)
	for _, sig := range signatures {
		parts := strings.SplitN(sig, ",", 2)
		if len(parts) == 2 && parts[0] == "v1" {
			sigBytes, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				continue
			}
			if hmac.Equal(sigBytes, expectedMAC) {
				return nil
			}
		}
	}

	return ErrSignedWebhookVerificationFailed
}

func parseSigningSecret(secret string) ([]byte, error) {
	trimmed := strings.TrimPrefix(secret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		if urlKey, urlErr := base64.URLEncoding.DecodeString(trimmed); urlErr == nil {
			return urlKey, nil
		}
		return []byte(trimmed), nil
	}
	return key, nil
}

func getHeader(h http.Header, keys ...string) string {
	for _, k := range keys {
		if v := h.Get(k); v != "" {
			return v
		}
	}
	for _, k := range keys {
		for headerKey, vals := range h {
			if strings.EqualFold(headerKey, k) && len(vals) > 0 {
				return vals[0]
			}
		}
	}
	return ""
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

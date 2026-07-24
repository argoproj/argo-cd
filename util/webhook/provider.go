package webhook

import "net/http"

// WebhookProvider identifies the provider that sent a webhook payload.
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

// ProviderParser detects and parses webhook requests for one provider.
//
// CanHandle inspects request headers to decide whether this parser owns the
// request. Parse extracts the provider-specific payload. On verification
// failures it emits a provider-specific security-audit log line directly. A
// (nil, nil) return signals a request that was claimed but intentionally
// skipped (e.g. an unsupported sub-event).
type ProviderParser interface {
	CanHandle(r *http.Request) bool
	Parse(r *http.Request, consumer WebhookConsumer) (any, error)
	Name() WebhookProvider
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

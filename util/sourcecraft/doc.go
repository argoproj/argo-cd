// Package sourcecraft provides a Go client library for the SourceCraft API.
//
// SourceCraft is a comprehensive platform for software development lifecycle management,
// providing code repository management, CI/CD pipelines, error tracking, and more.
//
// # API Client
//
// The Client type provides methods to interact with the SourceCraft REST API.
// Create a client using NewClient with your API endpoint and authentication token:
//
//	client, err := sourcecraft.NewClient(
//	    "https://api.sourcecraft.tech",
//	    sourcecraft.SetToken("your-personal-access-token"),
//	)
//
// # Webhook Support
//
// The package includes comprehensive webhook parsing and verification capabilities.
// Create a webhook parser to handle incoming webhook events:
//
//	hook, err := sourcecraft.New(
//	    sourcecraft.Options.Secret("your-webhook-secret"),
//	)
//
// Parse incoming webhook requests and handle events:
//
//	payload, err := hook.Parse(r, sourcecraft.PushEvent, sourcecraft.PullRequestAggregate)
//	if err != nil {
//	    // handle error
//	}
//
//	switch p := payload.(type) {
//	case sourcecraft.PushEventPayload:
//	    // handle push event
//	case sourcecraft.PullRequestEventPayload:
//	    // handle pull request event
//	}
//
// The webhook parser automatically verifies HMAC signatures using SHA-256
// when a secret is configured, ensuring webhook authenticity.
//
// # Thread Safety
//
// All Client methods are safe for concurrent use. The Client type uses
// internal synchronization to protect shared state.
//
// # Resources
//
//   - SourceCraft Portal: https://sourcecraft.dev
//   - API Documentation: https://api.sourcecraft.tech/docs/index.html
//   - Getting Started: https://sourcecraft.dev/portal/docs/en/sourcecraft/operations/api-start
package sourcecraft

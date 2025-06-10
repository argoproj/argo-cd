package merger

import (
	"net/http"

	log "github.com/sirupsen/logrus"

	appsetwebhook "github.com/argoproj/argo-cd/v3/applicationset/webhook"
	argocdwebhook "github.com/argoproj/argo-cd/v3/util/webhook"
)

type WebhookMerger struct {
	acdWebhookHandler    *argocdwebhook.ArgoCDWebhookHandler
	appSetWebhookHandler *appsetwebhook.WebhookHandler
}

func NewWebhookMerger(
	acdWebhookHandler *argocdwebhook.ArgoCDWebhookHandler, appSetWebhookHandler *appsetwebhook.WebhookHandler,
) *WebhookMerger {
	return &WebhookMerger{
		acdWebhookHandler:    acdWebhookHandler,
		appSetWebhookHandler: appSetWebhookHandler,
	}
}

func (m *WebhookMerger) Handler(w http.ResponseWriter, r *http.Request) {
	for name, h := range map[string]func(w http.ResponseWriter, r *http.Request) error{
		"argo cd":         m.acdWebhookHandler.HandleRequest,
		"application set": m.appSetWebhookHandler.HandleRequest,
	} {
		go func() {
			req, err := copyRequest(r)
			if err == nil {
				if err = h(w, req); err != nil {
					log.Printf("error handling %s webhook: %+v. maybe not suitable?", name, err)
				}
			} else {
				log.Printf("error copying request: %+v", err)
			}
		}()
	}
}

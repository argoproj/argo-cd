package webhookmerger

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

type WebhookHandler interface {
	HandleRequest(w http.ResponseWriter, r *http.Request) error
}

type WebhookMerger struct {
	acdWebhookHandler    WebhookHandler
	appSetWebhookHandler WebhookHandler
}

func NewWebhookMerger(
	acdWebhookHandler, appSetWebhookHandler WebhookHandler,
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
		req, err := copyRequest(r)
		if err == nil {
			if err = h(w, req); err != nil {
				log.Printf("error handling %s webhook: %+v. maybe not suitable?", name, err)
			}
		} else {
			log.Printf("error copying request: %+v", err)
		}
	}
}

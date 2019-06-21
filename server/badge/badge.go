package badge

import (
	"net/http"

	svg "github.com/ajstarks/svgo"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NewHandler ...
func NewHandler(appClientset versioned.Interface, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace}
}

//Handler ...
type Handler struct {
	namespace    string
	appClientset versioned.Interface
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["name"]

	if !ok || len(keys[0]) < 1 {
		w.Write([]byte("URL Parameter 'name=(enter application name here)' is missing"))
		return
	}

	key := keys[0]

	// Sample url: http://localhost:8080/api/badge?name=123
	app, err := h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).Get(key, v1.GetOptions{})
	if err != nil {
		w.Write([]byte("Sorry, the application whose health and sync status you were looking for does not exist, or was mistyped."))
		return
	}

	_, err = w.Write([]byte(""))
	if err != nil {
		w.WriteHeader(503)
	} else {
		w.WriteHeader(200)
	}
	s := svg.New(w)
	s.Start(2000, 2000)

	health := app.Status.Health.Status
	status := app.Status.Sync.Status
	syncStart := len(health)*6 + 1

	switch {
	case health == "Healthy":
		s.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(127,255,131);stroke:black;stroke-width:1")
		s.Text(3, 18, "Healthy", "font-size:12;fill:black")
	case health == "Progressing":
		s.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(255,251,92);stroke:black;stroke-width:1")
		s.Text(3, 18, "Progressing", "font:Helvetica;font-size:12;fill:black")
	case health == "Suspended":
		s.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(255,145,0);stroke:black;stroke-width:1")
		s.Text(2, 18, "Suspended", "font-size:12;fill:black")
	case health == "Degraded":
		s.Roundrect(0, 0, len(health)*7-3, 25, 2, 2, "fill:rgb(109,202,205);stroke:black;stroke-width:1")
		s.Text(2, 18, "Degraded", "font-size:12;fill:black")
		syncStart = len(health)*7 - 3
	case health == "Missing":
		s.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(255,36,36);stroke:black;stroke-width:1")
		s.Text(2, 18, "Missing", "font-size:12;fll:black")
	default:
		s.Roundrect(0, 0, len("Unknown")*7+2, 25, 2, 2, "fill:rgb(178,102,255);stroke:black;stroke-width:1")
		s.Text(2, 18, "Unknown", "font-size:12;fill:black")
		syncStart = len("Unknown")*7 + 2
	}
	switch {
	case status == "Synced":
		s.Roundrect(syncStart, 0, len("Synced")*7, 25, 2, 2, "fill:rgb(0,204,0);stroke:black;stroke-width:1")
		s.Text(syncStart+3, 18, "Synced", "font-size:12;fill:black")
	case status == "OutOfSync":
		s.Roundrect(syncStart, 0, len("Out of Sync")*6, 25, 2, 2, "fill:rgb(255,57,57);stroke:black;stroke-width:1")
		s.Text(syncStart+3, 18, "Out Of Sync", "font-size:12;fill:black")
	default:
		s.Roundrect(syncStart, 0, len("Unknown")*8, 25, 2, 2, "fill:rgb(209,155,177);stroke:black;stroke-width:1")
		s.Text(syncStart+4, 18, "Unknown", "font-size:12;fill:black")
	}
	s.End()
}

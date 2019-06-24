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

//Sample url: http://localhost:8080/api/badge?name=123
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	keys, ok := r.URL.Query()["job"]

	if !ok || len(keys[0]) < 1 {
		svgOne := svg.New(w)
		svgOne.Start(2000, 2000)
		svgOne.Roundrect(0, 0, 120, 25, 2, 3, "fill:rgb(200,321,233);stroke:black;stroke-width:0.7")
		svgOne.Text(4, 18, "Param 'job=' missing", "font-family:times;font-size:12")
		svgOne.End()
		return
	}

	key := keys[0]
	app, err := h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).Get(key, v1.GetOptions{})
	if err != nil {
		svgTwo := svg.New(w)
		svgTwo.Start(2000, 2000)
		svgTwo.Roundrect(0, 0, 114, 25, 2, 3, "fill:rgb(255,43,0);opacity:0.7;stroke:black;stroke-width:0.7")
		svgTwo.Text(3, 18, "Application not found", "font-family:times;font-size:12")
		svgTwo.End()
		return
	}

	_, err = w.Write([]byte(""))
	if err != nil {
		w.WriteHeader(503)
	} else {
		w.WriteHeader(200)
	}

	health := app.Status.Health.Status
	status := app.Status.Sync.Status
	syncTextStart := len(health) * 6

	svgThree := svg.New(w)
	svgThree.Start(2000, 2000)

	switch {
	case health == "Healthy":
		svgThree.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(127,255,131);stroke:black;stroke-width:0.7")
		svgThree.Text(3, 18, "Healthy", "font-family:times;font-size:12;fill:black")
	case health == "Progressing":
		svgThree.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(255,251,92);stroke:black;stroke-width:0.7")
		svgThree.Text(3, 18, "Progressing", "font-family:times;font-size:12;fill:black")
	case health == "Suspended":
		svgThree.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(255,145,0);stroke:black;stroke-width:0.7")
		svgThree.Text(2, 18, "Suspended", "font-family:times;font-size:12;fill:black")
	case health == "Degraded":
		svgThree.Roundrect(0, 0, len(health)*7-3, 25, 2, 2, "fill:rgb(109,202,205);stroke:black;stroke-width:0.7")
		svgThree.Text(2, 18, "Degraded", "font-family:times;font-size:12;fill:black")
		syncTextStart = len(health)*7 - 3
	case health == "Missing":
		svgThree.Roundrect(0, 0, len(health)*6, 25, 2, 2, "fill:rgb(255,36,36);stroke:black;stroke-width:0.7")
		svgThree.Text(2, 18, "Missing", "font-family:times;font-size:12;fll:black")
	default:
		svgThree.Roundrect(0, 0, len("Unknown")*7+2, 25, 2, 2, "fill:rgb(178,102,255);stroke:black;stroke-width:0.7")
		svgThree.Text(2, 18, "Unknown", "font-family:times;font-size:12;fill:black")
		syncTextStart = len("Unknown")*7 + 2
	}
	switch {
	case status == "Synced":
		svgThree.Roundrect(syncTextStart, 0, len("Synced")*7, 25, 2, 2, "fill:rgb(0,204,0);stroke:black;stroke-width:0.7")
		svgThree.Text(syncTextStart+3, 18, "Synced", "font-family:times;font-size:12;fill:black")
	case status == "OutOfSync":
		svgThree.Roundrect(syncTextStart, 0, len("Out of Sync")*6, 25, 2, 2, "fill:rgb(255,57,57);stroke:black;stroke-width:0.7")
		svgThree.Text(syncTextStart+3, 18, "Out Of Sync", "font-family:times;font-size:12;fill:black")
	default:
		svgThree.Roundrect(syncTextStart, 0, len("Unknown")*8, 25, 2, 2, "fill:rgb(209,155,177);stroke:black;stroke-width:0.7")
		svgThree.Text(syncTextStart+4, 18, "Unknown", "font-familye:times;font-size:12;fill:black")
	}

	svgThree.End()
}

//

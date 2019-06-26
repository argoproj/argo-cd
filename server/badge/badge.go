package badge

import (
	"net/http"
	"strings"

	svg "github.com/ajstarks/svgo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
)

//NewHandler creates handler serving to do api/badge endpoint
func NewHandler(appClientset versioned.Interface, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace}
}

//Handler used to get application in order to access health/sync
type Handler struct {
	namespace    string
	appClientset versioned.Interface
}

//ServeHTTP returns badge with health and sync status for application
//(or an error badge if wrong query or application name is given)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//Sample url: http://localhost:8080/api/badge?name=123
	keys, ok := r.URL.Query()["name"]

	//error when you add another query after name = and do like http://localhost:8080/api/badge?name=123/hadfkjajdhj

	pageWidth := 2000
	pageHeight := 2000
	xStart := 0
	yStart := 0
	badgeHeight := 25
	//badgeCurve is the rx/ry value for the round rectangle for each badge
	badgeCurve := 2
	textFormat := "font-size:11;fill:black"

	if !ok || len(keys[0]) < 1 {
		svgOne := svg.New(w)
		svgOne.Start(pageWidth, pageHeight)
		svgOne.Roundrect(xStart, yStart, 120, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(200,321,233);stroke:black;stroke-width:0.7")
		svgOne.Text(4, 18, "Param 'name=' missing", textFormat)
		svgOne.End()
		return
	}

	key := keys[0]
	//if another query is added after the appplication name and is separated by a /
	q := strings.Split(key, "/")
	key = q[0]
	app, err := h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).Get(key, v1.GetOptions{})
	if err != nil {
		svgTwo := svg.New(w)
		svgTwo.Start(pageWidth, pageHeight)
		svgTwo.Roundrect(xStart, yStart, 114, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(255,43,0);stroke:black;stroke-width:0.7")
		svgTwo.Text(3, 18, "Application not found", textFormat)
		svgTwo.Text(4, 30, key)
		svgTwo.End()
		return
	}

	health := app.Status.Health.Status
	status := "Unknown"
	//status := app.Status.Sync.Status
	healthBadgeLength := len(health) * 7
	//healthBadgeLength is where the sync badge starts along with being the length of the health badge
	syncBadgeLength := len(status) * 7
	syncTextStart := healthBadgeLength + 3
	svgThree := svg.New(w)
	svgThree.Start(pageWidth, pageHeight)
	xHealthText := 3
	yText := 18
	switch health {
	case "Healthy":
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(127,255,131);stroke:black;stroke-width:0.7")
		svgThree.Text(xHealthText, yText, health, textFormat)
	case "Progressing":
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(255,251,92);stroke:black;stroke-width:0.7")
		svgThree.Text(xHealthText, yText, health, textFormat)
	case "Suspended":
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(255,145,0);stroke:black;stroke-width:0.7")
		svgThree.Text(xHealthText, yText, health, textFormat)
	case "Degraded":
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(109,202,205);stroke:black;stroke-width:0.7")
		svgThree.Text(xHealthText, yText, health, textFormat)
	case "Missing":
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(255,36,36);stroke:black;stroke-width:0.7")
		svgThree.Text(xHealthText, yText, health, textFormat)
	default:
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(178,102,255);stroke:black;stroke-width:0.7")
		svgThree.Text(xHealthText, yText, health, textFormat)
	}
	switch status {
	case "Synced":
		svgThree.Roundrect(healthBadgeLength, yStart, syncBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(0,204,0);stroke:black;stroke-width:0.7")
		svgThree.Text(syncTextStart, yText, status, textFormat)
	case "OutOfSync":
		svgThree.Roundrect(healthBadgeLength, yStart, syncBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(255,57,57);stroke:black;stroke-width:0.7")
		svgThree.Text(syncTextStart, yText, status, textFormat)
	default:
		svgThree.Roundrect(healthBadgeLength, yStart, syncBadgeLength, badgeHeight, badgeCurve, badgeCurve, "fill:rgb(209,155,177);stroke:black;stroke-width:0.7")
		svgThree.Text(syncTextStart, yText, status, textFormat)
	}

	svgThree.End()
}

//func (h string, s string) rectangle() {

//}/

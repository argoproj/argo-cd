package badge

import (
	"net/http"
	"strings"

	svg "github.com/ajstarks/svgo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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

const (
	syncTextOffStart = 3
	lightBlue        = "fill:rgb(200,321,233);stroke:black"
	red              = "fill:rgb(255,43,0);stroke:black"
	lightGreen       = "fill:rgb(127,255,131);stroke:black"
	yellow           = "fill:rgb(255,251,92);stroke:black"
	orange           = "fill:rgb(255,145,0);stroke:black"
	teal             = "fill:rgb(109,202,205);stroke:black"
	purple           = "fill:rgb(178,102,255);stroke:black"
	darkgreen        = "fill:rgb(0,204,0);stroke:black"

	pageWidth   = 2000
	pageHeight  = 2000
	xStart      = 0
	yStart      = 0
	badgeHeight = 25
	//badgeCurve is the rx/ry value for the round rectangle for each badge
	badgeCurve = 2
	textFormat = "font-size:11;fill:black"
	yText      = 17
	//xHealthText is x pos where text for health badge and edge case badges start
	xHealthText         = 3
	nameMissingLength   = 120
	badgeBuffer         = 7
	notFoundBadgeBuffer = 6
)

//ServeHTTP returns badge with health and sync status for application
//(or an error badge if wrong query or application name is given)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//Sample url: http://localhost:8080/api/badge?name=123
	keys, ok := r.URL.Query()["name"]

	if !ok || len(keys[0]) < 1 {
		svgOne := svg.New(w)
		svgOne.Start(pageWidth, pageHeight)
		svgOne.Roundrect(xStart, yStart, nameMissingLength, badgeHeight, badgeCurve, badgeCurve, lightBlue)
		svgOne.Text(xHealthText, yText, "Param 'name=' missing", textFormat)
		svgOne.End()
		return
	}

	key := keys[0]
	//if another query is added after the appplication name and is separated by a / this will make sure it only looks at
	//what is between the name= and / and will open the applicaion by that name
	q := strings.Split(key, "/")
	key = q[0]
	app, err := h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).Get(key, v1.GetOptions{})
	if err != nil {
		notFoundBadgeLength := len("Application"+key+" not found") * notFoundBadgeBuffer
		svgTwo := svg.New(w)
		svgTwo.Start(pageWidth, pageHeight)
		svgTwo.Roundrect(xStart, yStart, notFoundBadgeLength, badgeHeight, badgeCurve, badgeCurve, red)
		svgTwo.Text(xHealthText, yText, "Application '"+key+"' not found", textFormat)
		svgTwo.End()
		return
	}

	health := app.Status.Health.Status
	status := app.Status.Sync.Status

	healthBadgeLength := len(health) * badgeBuffer
	//healthBadgeLength is where the sync badge starts along with being the length of the health badge
	syncBadgeLength := len(status) * badgeBuffer

	syncTextStart := healthBadgeLength + syncTextOffStart

	svgThree := svg.New(w)
	svgThree.Start(pageWidth, pageHeight)

	switch health {
	case appv1.HealthStatusHealthy:
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, lightGreen)
		svgThree.Text(xHealthText, yText, health, textFormat)
	case appv1.HealthStatusProgressing:
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, yellow)
		svgThree.Text(xHealthText, yText, health, textFormat)
	case appv1.HealthStatusSuspended:
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, orange)
		svgThree.Text(xHealthText, yText, health, textFormat)
	case appv1.HealthStatusDegraded:
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, teal)
		svgThree.Text(xHealthText, yText, health, textFormat)
	case appv1.HealthStatusMissing:
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, red)
		svgThree.Text(xHealthText, yText, health, textFormat)
	default:
		svgThree.Roundrect(xStart, yStart, healthBadgeLength, badgeHeight, badgeCurve, badgeCurve, purple)
		svgThree.Text(xHealthText, yText, health, textFormat)
	}
	switch status {
	case appv1.SyncStatusCodeSynced:
		svgThree.Roundrect(healthBadgeLength, yStart, syncBadgeLength, badgeHeight, badgeCurve, badgeCurve, darkgreen)
		svgThree.Text(syncTextStart, yText, string(status), textFormat)
	case appv1.SyncStatusCodeOutOfSync:
		svgThree.Roundrect(healthBadgeLength, yStart, syncBadgeLength, badgeHeight, badgeCurve, badgeCurve, red)
		svgThree.Text(syncTextStart, yText, string(status), textFormat)
	default:
		svgThree.Roundrect(healthBadgeLength, yStart, syncBadgeLength, badgeHeight, badgeCurve, badgeCurve, purple)
		svgThree.Text(syncTextStart, yText, string(status), textFormat)
	}

	svgThree.End()
}

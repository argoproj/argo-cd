package badge

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"

	"net/http"
	"text/template"

	"github.com/andanhm/go-prettytime"
	healthutil "github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

//NewHandler creates handler serving to do api/badge endpoint
func NewHandler(appClientset versioned.Interface, settingsMrg *settings.SettingsManager, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace, settingsMgr: settingsMrg}
}

//Handler used to get application in order to access health/sync
type Handler struct {
	namespace    string
	appClientset versioned.Interface
	settingsMgr  *settings.SettingsManager
}

const (
	svgDefaultWidth                     = 131
	svgLeftRectDefaultWidth             = 76
	svgRightRectDefaultWidth            = 57
	svgRightTextDefaultX                = 1035
	svgRightRectSyncDateDefaultWidth    = 100
	svgRightTextSyncDateDefaultX        = 1250
	svgRectSpace                        = 2
	svgRevisionTextDefaultLastSyncTimeX = 2080
	svgRevisionTextDefaultX             = 1660
	svgRevisionRectDefaultWidth         = 62
)

type badgeArgs struct {
	Width          int
	LeftBgColor    string
	RightBGColor   string
	LeftText       string
	RightText      string
	RightRectWidth int
	RightTextX     int

	RevisionRectDisplay string
	RevisionText        string
	RevisionRectX       int
	RevisionRectTextX   int
}

//ServeHTTP returns badge with health and sync status for application
//(or an error badge if wrong query or application name is given)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	health := healthutil.HealthStatusUnknown
	status := appv1.SyncStatusCodeUnknown
	syncDate := ""
	revision := ""
	lastSyncTimeEnabled := false
	revisionEnabled := false
	enabled := false
	notFound := false
	if sets, err := h.settingsMgr.GetSettings(); err == nil {
		enabled = sets.StatusBadgeEnabled
	}

	//Sample url: http://localhost:8080/api/badge?name=123
	if name, ok := r.URL.Query()["name"]; ok && enabled {
		if app, err := h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).Get(context.Background(), name[0], v1.GetOptions{}); err == nil {
			health = app.Status.Health.Status
			status = app.Status.Sync.Status
			if app.Status.OperationState != nil && app.Status.OperationState.SyncResult != nil {
				syncDate = prettytime.Format(app.Status.OperationState.FinishedAt.Time)
				revision = app.Status.OperationState.SyncResult.Revision
			}
		} else if errors.IsNotFound(err) {
			notFound = true
		}
	}
	//Sample url: http://localhost:8080/api/badge?project=default
	if projects, ok := r.URL.Query()["project"]; ok && enabled {
		if apps, err := h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).List(context.Background(), v1.ListOptions{}); err == nil {
			applicationSet := argo.FilterByProjects(apps.Items, projects)
			for _, a := range applicationSet {
				if a.Status.Sync.Status != appv1.SyncStatusCodeSynced {
					status = appv1.SyncStatusCodeOutOfSync
				}
				if a.Status.Health.Status != healthutil.HealthStatusHealthy {
					health = healthutil.HealthStatusDegraded
				}
			}
			if health != healthutil.HealthStatusDegraded && len(applicationSet) > 0 {
				health = healthutil.HealthStatusHealthy
			}
			if status != appv1.SyncStatusCodeOutOfSync && len(applicationSet) > 0 {
				status = appv1.SyncStatusCodeSynced
			}
		}
	}
	//Sample url: http://localhost:8080/api/badge?name=123&revision=true
	if _, ok := r.URL.Query()["revision"]; ok && enabled {
		revisionEnabled = true
	}

	//Sample url: http://localhost:8080/api/badge?name=123&lastSyncTime=true
	if _, ok := r.URL.Query()["lastSyncTime"]; ok && enabled {
		lastSyncTime, _ := strconv.ParseBool(r.URL.Query()["lastSyncTime"][0])
		lastSyncTimeEnabled = lastSyncTime
	}

	leftColorString := ""
	if leftColor, ok := HealthStatusColors[health]; ok {
		leftColorString = toRGBString(leftColor)
	} else {
		leftColorString = toRGBString(Grey)
	}

	rightColorString := ""
	if rightColor, ok := SyncStatusColors[status]; ok {
		rightColorString = toRGBString(rightColor)
	} else {
		rightColorString = toRGBString(Grey)
	}

	leftText := string(health)
	rightText := string(status)

	if notFound {
		leftText = "Not Found"
		rightText = ""
	}

	badgeArgs := badgeArgs{
		Width:               svgDefaultWidth,
		RevisionRectDisplay: "none",
		RightRectWidth:      svgRightRectDefaultWidth,
		RightTextX:          svgRightTextDefaultX,
	}

	if !notFound && lastSyncTimeEnabled && syncDate != "" {
		rightText = fmt.Sprintf("%s %s", rightText, syncDate)
		badgeArgs.RightRectWidth = svgRightRectSyncDateDefaultWidth
		badgeArgs.RightTextX = svgRightTextSyncDateDefaultX
	}

	badgeArgs.LeftBgColor = leftColorString
	badgeArgs.RightBGColor = rightColorString
	badgeArgs.LeftText = leftText
	badgeArgs.RightText = rightText

	badgeArgs.Width = svgLeftRectDefaultWidth + badgeArgs.RightRectWidth

	if !notFound && revisionEnabled && revision != "" {
		shortRevision := revision
		if len(shortRevision) > 7 {
			shortRevision = shortRevision[:7]
		}
		badgeArgs.RevisionText = fmt.Sprintf("(%s)", shortRevision)
		badgeArgs.RevisionRectDisplay = "inline"
		badgeArgs.RevisionRectX = badgeArgs.Width + svgRectSpace
		if lastSyncTimeEnabled {
			badgeArgs.RevisionRectTextX = svgRevisionTextDefaultLastSyncTimeX
		} else {
			badgeArgs.RevisionRectTextX = svgRevisionTextDefaultX
		}
		badgeArgs.Width = badgeArgs.RevisionRectX + svgRevisionRectDefaultWidth
	}

	w.Header().Set("Content-Type", "image/svg+xml")

	//Ask cache's to not cache the contents in order prevent the badge from becoming stale
	w.Header().Set("Cache-Control", "private, no-store")

	//Allow badges to be fetched via XHR from frontend applications without running into CORS issues
	w.Header().Set("Access-Control-Allow-Origin", "*")

	badge := assets.BadgeSVG
	config := template.Must(template.New("badge").Parse(badge))
	var buff bytes.Buffer
	err := config.ExecuteTemplate(&buff, "badge", badgeArgs)
	if err != nil {
		log.Errorf("error executing template for badge creation %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error executing template for badge creation"))
	} else {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buff.Bytes())
	}
}

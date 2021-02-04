package badge

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	healthutil "github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/assets"
	"github.com/argoproj/argo-cd/util/settings"
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

var (
	svgWidthPattern          = regexp.MustCompile(`^<svg width="([^"]*)"`)
	displayNonePattern       = regexp.MustCompile(`display="none"`)
	leftRectColorPattern     = regexp.MustCompile(`id="leftRect" fill="([^"]*)"`)
	rightRectColorPattern    = regexp.MustCompile(`id="rightRect" fill="([^"]*)"`)
	revisionRectColorPattern = regexp.MustCompile(`id="revisionRect" fill="([^"]*)"`)
	leftTextPattern          = regexp.MustCompile(`id="leftText" [^>]*>([^<]*)`)
	rightTextPattern         = regexp.MustCompile(`id="rightText" [^>]*>([^<]*)`)
	revisionTextPattern      = regexp.MustCompile(`id="revisionText" [^>]*>([^<]*)`)
)

const (
	svgWidthWithRevision = 192
)

func replaceFirstGroupSubMatch(re *regexp.Regexp, str string, repl string) string {
	result := ""
	lastIndex := 0

	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		groups := []string{}
		for i := 0; i < len(v); i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}

		result += str[lastIndex:v[0]] + groups[0] + repl
		lastIndex = v[1]
	}

	return result + str[lastIndex:]
}

//ServeHTTP returns badge with health and sync status for application
//(or an error badge if wrong query or application name is given)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	health := healthutil.HealthStatusUnknown
	status := appv1.SyncStatusCodeUnknown
	revision := ""
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

	badge := assets.BadgeSVG
	badge = leftRectColorPattern.ReplaceAllString(badge, fmt.Sprintf(`id="leftRect" fill="%s" $2`, leftColorString))
	badge = rightRectColorPattern.ReplaceAllString(badge, fmt.Sprintf(`id="rightRect" fill="%s" $2`, rightColorString))
	badge = replaceFirstGroupSubMatch(leftTextPattern, badge, leftText)
	badge = replaceFirstGroupSubMatch(rightTextPattern, badge, rightText)

	if !notFound && revisionEnabled && revision != "" {
		// Increase width of SVG and enable display of revision components
		badge = svgWidthPattern.ReplaceAllString(badge, fmt.Sprintf(`<svg width="%d" $2`, svgWidthWithRevision))
		badge = displayNonePattern.ReplaceAllString(badge, `display="inline"`)
		badge = revisionRectColorPattern.ReplaceAllString(badge, fmt.Sprintf(`id="revisionRect" fill="%s" $2`, rightColorString))
		shortRevision := revision
		if len(shortRevision) > 7 {
			shortRevision = shortRevision[:7]
		}
		badge = replaceFirstGroupSubMatch(revisionTextPattern, badge, fmt.Sprintf("(%s)", shortRevision))
	}

	w.Header().Set("Content-Type", "image/svg+xml")

	//Ask cache's to not cache the contents in order prevent the badge from becoming stale
	w.Header().Set("Cache-Control", "private, no-store")

	//Allow badges to be fetched via XHR from frontend applications without running into CORS issues
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(badge))
}

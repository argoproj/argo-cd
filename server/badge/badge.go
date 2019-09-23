package badge

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/util/settings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/assets"
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
	unknown     = "rgb(178,102,255)"
	success     = "#18be52"
	warning     = "#f4c030"
	failed      = "#E96D76"
	progressing = "#0DADEA"
	suspended   = "#CCD6DD"
)

var (
	leftPathColorPattern  = regexp.MustCompile(`id="leftPath" fill="([^"]*)"`)
	rightPathColorPattern = regexp.MustCompile(`id="rightPath" fill="([^"]*)"`)
	leftText1Pattern      = regexp.MustCompile(`id="leftText1" [^>]*>([^<]*)`)
	leftText2Pattern      = regexp.MustCompile(`id="leftText2" [^>]*>([^<]*)`)
	rightText1Pattern     = regexp.MustCompile(`id="rightText1" [^>]*>([^<]*)`)
	rightText2Pattern     = regexp.MustCompile(`id="rightText2" [^>]*>([^<]*)`)
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
	health := appv1.HealthStatusUnknown
	status := appv1.SyncStatusCodeUnknown
	enabled := false
	if sets, err := h.settingsMgr.GetSettings(); err == nil {
		enabled = sets.StatusBadgeEnabled
	}

	//Sample url: http://localhost:8080/api/badge?name=123
	if keys, ok := r.URL.Query()["name"]; ok && enabled {
		key := keys[0]
		//if another query is added after the application name and is separated by a / this will make sure it only looks at
		//what is between the name= and / and will open the applicaion by that name
		q := strings.Split(key, "/")
		key = q[0]
		if app, err := h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).Get(key, v1.GetOptions{}); err == nil {
			health = app.Status.Health.Status
			status = app.Status.Sync.Status
		}
	}

	leftColor := ""
	rightColor := ""
	leftText := health
	rightText := string(status)

	switch health {
	case appv1.HealthStatusHealthy:
		leftColor = success
	case appv1.HealthStatusProgressing:
		leftColor = progressing
	case appv1.HealthStatusSuspended:
		leftColor = suspended
	case appv1.HealthStatusDegraded:
		leftColor = failed
	case appv1.HealthStatusMissing:
		leftColor = unknown
	default:
		leftColor = unknown
	}

	switch status {
	case appv1.SyncStatusCodeSynced:
		rightColor = success
	case appv1.SyncStatusCodeOutOfSync:
		rightColor = warning
	default:
		rightColor = unknown
	}
	badge := assets.BadgeSVG
	badge = leftPathColorPattern.ReplaceAllString(badge, fmt.Sprintf(`id="leftPath" fill="%s" $2`, leftColor))
	badge = rightPathColorPattern.ReplaceAllString(badge, fmt.Sprintf(`id="rightPath" fill="%s" $2`, rightColor))
	badge = replaceFirstGroupSubMatch(leftText1Pattern, badge, leftText)
	badge = replaceFirstGroupSubMatch(leftText2Pattern, badge, leftText)
	badge = replaceFirstGroupSubMatch(rightText1Pattern, badge, rightText)
	badge = replaceFirstGroupSubMatch(rightText2Pattern, badge, rightText)
	w.Header().Set("Content-Type", "image/svg+xml")

	//Ask cache's to not cache the contents in order prevent the badge from becoming stale
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(badge))
}

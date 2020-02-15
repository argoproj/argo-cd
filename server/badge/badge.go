package badge

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
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
	leftRectColorPattern  = regexp.MustCompile(`id="leftRect" fill="([^"]*)"`)
	rightRectColorPattern = regexp.MustCompile(`id="rightRect" fill="([^"]*)"`)
	leftTextPattern       = regexp.MustCompile(`id="leftText" [^>]*>([^<]*)`)
	rightTextPattern      = regexp.MustCompile(`id="rightText" [^>]*>([^<]*)`)
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
	notFound := false
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
		} else if errors.IsNotFound(err) {
			notFound = true
		}
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
	w.Header().Set("Content-Type", "image/svg+xml")

	//Ask cache's to not cache the contents in order prevent the badge from becoming stale
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(badge))
}

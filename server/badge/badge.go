package badge

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	healthutil "github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/api/errors"
	validation "k8s.io/apimachinery/pkg/api/validation"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/security"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

// NewHandler creates handler serving to do api/badge endpoint
func NewHandler(appClientset versioned.Interface, settingsMrg *settings.SettingsManager, namespace string, enabledNamespaces []string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace, settingsMgr: settingsMrg, enabledNamespaces: enabledNamespaces}
}

// Handler used to get application in order to access health/sync
type Handler struct {
	namespace         string
	appClientset      versioned.Interface
	settingsMgr       *settings.SettingsManager
	enabledNamespaces []string
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
	titleTextPattern         = regexp.MustCompile(`id="titleText" [^>]*>([^<]*)`)
	titleRectWidthPattern    = regexp.MustCompile(`(id="titleRect" .* width=)("0")`)
	rightRectWidthPattern    = regexp.MustCompile(`(id="rightRect" .* width=)("\d*")`)
	revisionRectWidthPattern = regexp.MustCompile(`(id="revisionRect" .* width=)("\d*")`)
	leftRectYCoodPattern     = regexp.MustCompile(`(id="leftRect" .* y=)("\d*")`)
	rightRectYCoodPattern    = regexp.MustCompile(`(id="rightRect" .* y=)("\d*")`)
	revisionRectYCoodPattern = regexp.MustCompile(`(id="revisionRect" .* y=)("\d*")`)
	leftTextYCoodPattern     = regexp.MustCompile(`(id="leftText" .* y=)("\d*")`)
	rightTextYCoodPattern    = regexp.MustCompile(`(id="rightText" .* y=)("\d*")`)
	revisionTextYCoodPattern = regexp.MustCompile(`(id="revisionText" .* y=)("\d*")`)
	revisionTextXCoodPattern = regexp.MustCompile(`(id="revisionText" x=)("\d*")`)
	svgHeightPattern         = regexp.MustCompile(`^(<svg .* height=)("\d*")`)
	logoYCoodPattern         = regexp.MustCompile(`(<image .* y=)("\d*")`)
)

const (
	svgWidthWithRevision      = 192
	svgWidthWithFullRevision  = 400
	svgWidthWithoutRevision   = 131
	svgHeightWithAppName      = 40
	badgeRowHeight            = 20
	statusRowYCoodWithAppName = 330
	logoYCoodWithAppName      = 22
	leftRectWidth             = 77
	widthPerChar              = 6
	textPositionWidthPerChar  = 62
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

// ServeHTTP returns badge with health and sync status for application
// (or an error badge if wrong query or application name is given)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	health := healthutil.HealthStatusUnknown
	status := appv1.SyncStatusCodeUnknown
	revision := ""
	displayedRevision := ""
	applicationName := ""
	revisionEnabled := false
	enabled := false
	displayAppName := false
	notFound := false
	adjustWidth := false
	svgWidth := svgWidthWithoutRevision
	if sets, err := h.settingsMgr.GetSettings(); err == nil {
		enabled = sets.StatusBadgeEnabled
	}

	reqNs := ""
	if ns, ok := r.URL.Query()["namespace"]; ok && enabled {
		if argo.IsValidNamespaceName(ns[0]) {
			if security.IsNamespaceEnabled(ns[0], h.namespace, h.enabledNamespaces) {
				reqNs = ns[0]
			} else {
				notFound = true
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		reqNs = h.namespace
	}

	// Sample url: http://localhost:8080/api/badge?name=123
	if name, ok := r.URL.Query()["name"]; ok && enabled && !notFound {
		if argo.IsValidAppName(name[0]) {
			if app, err := h.appClientset.ArgoprojV1alpha1().Applications(reqNs).Get(context.Background(), name[0], v1.GetOptions{}); err == nil {
				health = app.Status.Health.Status
				status = app.Status.Sync.Status
				applicationName = name[0]
				if app.Status.OperationState != nil && app.Status.OperationState.SyncResult != nil {
					revision = app.Status.OperationState.SyncResult.Revision
				}
			} else if errors.IsNotFound(err) {
				notFound = true
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	// Sample url: http://localhost:8080/api/badge?project=default
	if projects, ok := r.URL.Query()["project"]; ok && enabled && !notFound {
		for _, p := range projects {
			if errs := validation.NameIsDNSLabel(strings.ToLower(p), false); len(p) > 0 && len(errs) != 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if apps, err := h.appClientset.ArgoprojV1alpha1().Applications(reqNs).List(context.Background(), v1.ListOptions{}); err == nil {
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
	// Sample url: http://localhost:8080/api/badge?name=123&revision=true
	if revisionParam, ok := r.URL.Query()["revision"]; ok && enabled && strings.EqualFold(revisionParam[0], "true") {
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
		// Enable display of revision components
		badge = displayNonePattern.ReplaceAllString(badge, `display="inline"`)
		badge = revisionRectColorPattern.ReplaceAllString(badge, fmt.Sprintf(`id="revisionRect" fill="%s" $2`, rightColorString))

		adjustWidth = true
		displayedRevision = revision
		if keepFullRevisionParam, ok := r.URL.Query()["keepFullRevision"]; !(ok && strings.EqualFold(keepFullRevisionParam[0], "true")) && len(revision) > 7 {
			displayedRevision = revision[:7]
			svgWidth = svgWidthWithRevision
		} else {
			svgWidth = svgWidthWithFullRevision
		}

		badge = replaceFirstGroupSubMatch(revisionTextPattern, badge, fmt.Sprintf("(%s)", displayedRevision))
	}

	if widthParam, ok := r.URL.Query()["width"]; ok && enabled {
		width, err := strconv.Atoi(widthParam[0])
		if err == nil {
			svgWidth = width
			adjustWidth = true
		}
	}

	// Increase width of SVG
	if adjustWidth {
		badge = svgWidthPattern.ReplaceAllString(badge, fmt.Sprintf(`<svg width="%d" $2`, svgWidth))
		if revisionEnabled {
			xpos := (svgWidthWithoutRevision)*10 + (len(displayedRevision)+1)*textPositionWidthPerChar/2
			badge = revisionRectWidthPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, svgWidth-svgWidthWithoutRevision))
			badge = revisionTextXCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, xpos))
		} else {
			badge = rightRectWidthPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, svgWidth-leftRectWidth))
		}
	}

	if showAppNameParam, ok := r.URL.Query()["showAppName"]; ok && enabled && strings.EqualFold(showAppNameParam[0], "true") {
		displayAppName = true
	}

	if displayAppName && applicationName != "" {
		titleRectWidth := len(applicationName) * widthPerChar
		var longerWidth int = max(titleRectWidth, svgWidth)
		rightRectWidth := longerWidth - leftRectWidth
		badge = titleRectWidthPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, longerWidth))
		badge = rightRectWidthPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, rightRectWidth))
		badge = replaceFirstGroupSubMatch(titleTextPattern, badge, applicationName)
		badge = leftRectYCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, badgeRowHeight))
		badge = rightRectYCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, badgeRowHeight))
		badge = revisionRectYCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, badgeRowHeight))
		badge = leftTextYCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, statusRowYCoodWithAppName))
		badge = rightTextYCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, statusRowYCoodWithAppName))
		badge = revisionTextYCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, statusRowYCoodWithAppName))
		badge = svgHeightPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, svgHeightWithAppName))
		badge = logoYCoodPattern.ReplaceAllString(badge, fmt.Sprintf(`$1"%d"`, logoYCoodWithAppName))
		badge = svgWidthPattern.ReplaceAllString(badge, fmt.Sprintf(`<svg width="%d" $2`, longerWidth))
	}

	w.Header().Set("Content-Type", "image/svg+xml")

	// Ask cache's to not cache the contents in order prevent the badge from becoming stale
	w.Header().Set("Cache-Control", "private, no-store")

	// Allow badges to be fetched via XHR from frontend applications without running into CORS issues
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(badge))
}

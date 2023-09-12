package commands

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/util/duration"
)

const (
	firstElemPrefix = `├─`
	lastElemPrefix  = `└─`
	indent          = "  "
	pipe            = `│ `
)

func extractHealthStatusAndReason(node v1alpha1.ResourceNode) (healthStatus health.HealthStatusCode, reason string) {
	if node.Health != nil {
		healthStatus = node.Health.Status
		reason = node.Health.Message
	}
	return
}

func detailedTreeViewAppResourcesNotOrphaned(prefix string, uidToNodeMap map[string]v1alpha1.ResourceNode, parentChildMap map[string][]string, parent v1alpha1.ResourceNode, w *tabwriter.Writer) {

	if len(parent.ParentRefs) == 0 {
		healthStatus, reason := extractHealthStatusAndReason(parent)
		var age = "<unknown>"
		if parent.CreatedAt != nil {
			age = duration.HumanDuration(time.Since(parent.CreatedAt.Time))
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", parent.Group, parent.Kind, parent.Namespace, parent.Name, "No", age, healthStatus, reason)
	}
	chs := parentChildMap[parent.UID]
	for i, child := range chs {
		var p string
		switch i {
		case len(chs) - 1:
			p = prefix + lastElemPrefix
		default:
			p = prefix + firstElemPrefix
		}
		detailedTreeViewAppResourcesNotOrphaned(p, uidToNodeMap, parentChildMap, uidToNodeMap[child], w)
	}
}

func detailedTreeViewAppResourcesOrphaned(prefix string, uidToNodeMap map[string]v1alpha1.ResourceNode, parentChildMap map[string][]string, parent v1alpha1.ResourceNode, w *tabwriter.Writer) {
	healthStatus, reason := extractHealthStatusAndReason(parent)
	var age = "<unknown>"
	if parent.CreatedAt != nil {
		age = duration.HumanDuration(time.Since(parent.CreatedAt.Time))
	}
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", parent.Group, parent.Kind, parent.Namespace, parent.Name, "Yes", age, healthStatus, reason)

	chs := parentChildMap[parent.UID]
	for i, child := range chs {
		var p string
		switch i {
		case len(chs) - 1:
			p = prefix + lastElemPrefix
		default:
			p = prefix + firstElemPrefix
		}
		detailedTreeViewAppResourcesOrphaned(p, uidToNodeMap, parentChildMap, uidToNodeMap[child], w)
	}
}

func printPrefix(p string) string {
	if strings.HasSuffix(p, firstElemPrefix) {
		p = strings.Replace(p, firstElemPrefix, pipe, strings.Count(p, firstElemPrefix)-1)
	} else {
		p = strings.ReplaceAll(p, firstElemPrefix, pipe)
	}

	if strings.HasSuffix(p, lastElemPrefix) {
		p = strings.Replace(p, lastElemPrefix, strings.Repeat(" ", len([]rune(lastElemPrefix))), strings.Count(p, lastElemPrefix)-1)
	} else {
		p = strings.ReplaceAll(p, lastElemPrefix, strings.Repeat(" ", len([]rune(lastElemPrefix))))
	}
	return p
}

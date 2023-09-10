package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"k8s.io/apimachinery/pkg/util/duration"
)

const (
	firstElemPrefix = `├─`
	lastElemPrefix  = `└─`
	indent          = "  "
	pipe            = `│ `
)

var (
	gray  = color.New(color.FgHiBlack)
	red   = color.New(color.FgRed)
	green = color.New(color.FgGreen)
)

func extractHealthStatusAndReason(node v1alpha1.ResourceNode) (healthStatus health.HealthStatusCode, reason string) {
	if node.Health != nil {
		healthStatus = node.Health.Status
		reason = node.Health.Message
	}
	return
}

func treeViewAppGetDetailed(prefix string, tbl *uitable.Table, objs map[string]v1alpha1.ResourceNode, obj map[string][]string, parent v1alpha1.ResourceNode, mapNodeNameToResourceState map[string]*resourceState) {
	healthStatus, reason := extractHealthStatusAndReason(parent)

	var readyColor *color.Color
	switch healthStatus {
	case "Healthy":
		readyColor = green
	case "Degraded":
		readyColor = red
	default:
		readyColor = gray
	}

	var age = "<unknown>"
	if parent.CreatedAt != nil {
		age = duration.HumanDuration(time.Since(parent.CreatedAt.Time))
	}

	if mapNodeNameToResourceState[parent.Kind+"/"+parent.Name] != nil {
		value := mapNodeNameToResourceState[parent.Kind+"/"+parent.Name]

		tbl.AddRow(value.Group, value.Namespace, value.Kind, fmt.Sprintf("%s%s/%s",
			gray.Sprint(printPrefix(prefix)),
			parent.Kind,
			color.New(color.Bold).Sprint(parent.Name)),
			value.Status,
			value.Health,
			value.Hook,
			value.Message,
			readyColor.Sprint(healthStatus),
			readyColor.Sprint(reason),
			age)
	} else {
		tbl.AddRow(parent.Group, parent.Namespace, parent.Kind, fmt.Sprintf("%s%s/%s",
			gray.Sprint(printPrefix(prefix)),
			parent.Kind,
			color.New(color.Bold).Sprint(parent.Name)),
			"",
			"",
			"",
			"",
			readyColor.Sprint(healthStatus),
			readyColor.Sprint(reason),
			age)

	}
	chs := obj[parent.UID]
	for i, child := range chs {
		var p string
		switch i {
		case len(chs) - 1:
			p = prefix + lastElemPrefix
		default:
			p = prefix + firstElemPrefix
		}
		treeViewAppGetDetailed(p, tbl, objs, obj, objs[child], mapNodeNameToResourceState)
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

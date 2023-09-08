package commands

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
)

const (
	firstElemPrefix = `├─`
	lastElemPrefix  = `└─`
	indent          = "  "
	pipe            = `│ `
)

var (
	gray = color.New(color.FgHiBlack)
)

func treeViewAppResourcesNotOrphaned(prefix string, tbl *uitable.Table, objs map[string]v1alpha1.ResourceNode, obj map[string][]string, parent v1alpha1.ResourceNode) {
	if len(parent.ParentRefs) == 0 {
		tbl.AddRow(parent.Group, parent.Kind, parent.Namespace, fmt.Sprintf("%s%s/%s",
			gray.Sprint(printPrefix(prefix)),
			parent.Kind,
			color.New(color.Bold).Sprint(parent.Name)),
			"No")
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
		treeViewAppResourcesNotOrphaned(p, tbl, objs, obj, objs[child])
	}
}

func treeViewAppResourcesOrphaned(prefix string, tbl *uitable.Table, objs map[string]v1alpha1.ResourceNode, obj map[string][]string, parent v1alpha1.ResourceNode) {

	tbl.AddRow(parent.Group, parent.Kind, parent.Namespace, fmt.Sprintf("%s%s/%s",
		gray.Sprint(printPrefix(prefix)),
		parent.Kind,
		color.New(color.Bold).Sprint(parent.Name)),
		"No")

	chs := obj[parent.UID]
	for i, child := range chs {
		var p string
		switch i {
		case len(chs) - 1:
			p = prefix + lastElemPrefix
		default:
			p = prefix + firstElemPrefix
		}
		treeViewAppResourcesNotOrphaned(p, tbl, objs, obj, objs[child])
	}
}

func printPrefix(p string) string {
	// this part is hacky af
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

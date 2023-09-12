package commands

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const (
	firstElemPrefix = `├─`
	lastElemPrefix  = `└─`
	indent          = "  "
	pipe            = `│ `
)

func treeViewAppResourcesNotOrphaned(prefix string, uidToNodeMap map[string]v1alpha1.ResourceNode, parentChildMap map[string][]string, parent v1alpha1.ResourceNode, w *tabwriter.Writer) {
	if len(parent.ParentRefs) == 0 {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", parent.Group, parent.Kind, parent.Namespace, parent.Name, "No")
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
		treeViewAppResourcesNotOrphaned(p, uidToNodeMap, parentChildMap, uidToNodeMap[child], w)
	}
}

func treeViewAppResourcesOrphaned(prefix string, uidToNodeMap map[string]v1alpha1.ResourceNode, parentChildMap map[string][]string, parent v1alpha1.ResourceNode, w *tabwriter.Writer) {
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", parent.Group, parent.Kind, parent.Namespace, parent.Name, "Yes")
	chs := parentChildMap[parent.UID]
	for i, child := range chs {
		var p string
		switch i {
		case len(chs) - 1:
			p = prefix + lastElemPrefix
		default:
			p = prefix + firstElemPrefix
		}
		treeViewAppResourcesOrphaned(p, uidToNodeMap, parentChildMap, uidToNodeMap[child], w)
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

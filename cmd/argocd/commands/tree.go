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

func treeViewAppGet(prefix string, uidToNodeMap map[string]v1alpha1.ResourceNode, parentToChildMap map[string][]string, parent v1alpha1.ResourceNode, mapNodeNameToResourceState map[string]*resourceState, w *tabwriter.Writer) {
	if mapNodeNameToResourceState[parent.Kind+"/"+parent.Name] != nil {
		value := mapNodeNameToResourceState[parent.Kind+"/"+parent.Name]
		_, _ = fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\n", printPrefix(prefix), parent.Kind+"/"+value.Name, value.Status, value.Health, value.Message)
	} else {
		_, _ = fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\n", printPrefix(prefix), parent.Kind+"/"+parent.Name, " - ", " - ", " - ")
	}
	chs := parentToChildMap[parent.UID]
	for i, childUid := range chs {
		var p string
		switch i {
		case len(chs) - 1:
			p = prefix + lastElemPrefix
		default:
			p = prefix + firstElemPrefix
		}
		treeViewAppGet(p, uidToNodeMap, parentToChildMap, uidToNodeMap[childUid], mapNodeNameToResourceState, w)
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

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/notifications-engine/pkg/docs"
)

func main() {
	generateNotificationsDocs()
}

func generateNotificationsDocs() {
	_ = os.RemoveAll("./docs/operator-manual/notifications/services")
	_ = os.MkdirAll("./docs/operator-manual/notifications/services", 0o755)
	files, err := docs.CopyServicesDocs("./docs/operator-manual/notifications/services")
	if err != nil {
		log.Fatal(err)
	}
	if files != nil {
		if e := updateMkDocsNav("Operator Manual", "Notifications", "Notification Services", files); e != nil {
			log.Fatal(e)
		}
	}
}

func updateMkDocsNav(parent string, child string, subchild string, files []string) error {
	trimPrefixes(files, "docs/")
	sort.Strings(files)
	data, err := os.ReadFile("mkdocs.yml")
	if err != nil {
		return fmt.Errorf("error reading mkdocs.yml: %w", err)
	}
	var un unstructured.Unstructured
	if e := yaml.Unmarshal(data, &un.Object); e != nil {
		return e
	}
	nav := un.Object["nav"].([]interface{})
	rootitem, _ := findNavItem(nav, parent)
	if rootitem == nil {
		return fmt.Errorf("can't find '%s' root item in mkdoc.yml", parent)
	}
	rootnavitemmap := rootitem.(map[interface{}]interface{})
	childnav, _ := findNavItem(rootnavitemmap[parent].([]interface{}), child)
	if childnav == nil {
		return fmt.Errorf("can't find '%s' chile item under '%s' parent item in mkdoc.yml", child, parent)
	}

	childnavmap := childnav.(map[interface{}]interface{})
	childnavitems := childnavmap[child].([]interface{})

	childnavitems = removeNavItem(childnavitems, subchild)
	commands := make(map[string]interface{})
	commands[subchild] = files
	childnavmap[child] = append(childnavitems, commands)
	newmkdocs, err := yaml.Marshal(un.Object)
	if err != nil {
		return fmt.Errorf("error in marshaling final configmap: %w", err)
	}

	// The marshaller drops custom tags, so re-add this one. Turns out this is much less invasive than trying to handle
	// it at the YAML parser level.
	newmkdocs = bytes.Replace(newmkdocs, []byte("site_url: READTHEDOCS_CANONICAL_URL"), []byte("site_url: !ENV READTHEDOCS_CANONICAL_URL"), 1)

	return os.WriteFile("mkdocs.yml", newmkdocs, 0o644)
}

func trimPrefixes(files []string, prefix string) {
	for i, f := range files {
		files[i] = strings.TrimPrefix(f, prefix)
	}
}

func findNavItem(nav []interface{}, key string) (interface{}, int) {
	for i, item := range nav {
		o, ismap := item.(map[interface{}]interface{})
		if ismap {
			if _, ok := o[key]; ok {
				return o, i
			}
		}
	}
	return nil, -1
}

func removeNavItem(nav []interface{}, key string) []interface{} {
	_, i := findNavItem(nav, key)
	if i != -1 {
		nav = append(nav[:i], nav[i+1:]...)
	}
	return nav
}

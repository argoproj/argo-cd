package gen_docs

import (
	"fmt"
	"io/ioutil"
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
	os.RemoveAll("./docs/generated/notification-services")
	os.MkdirAll("./docs/generated/notification-services/", 0755)
	files, err := docs.CopyServicesDocs("./docs/generated/notification-services/")
	if err != nil {
		log.Fatal(err)
	}
	if files != nil {
		if e := updateMkDocsNav("Notifications", "Services", files); e != nil {
			log.Fatal(e)
		}
	}
}

func updateMkDocsNav(parent string, child string, files []string) error {
	trimPrefixes(files, "docs/")
	sort.Strings(files)
	data, err := ioutil.ReadFile("mkdocs.yml")
	if err != nil {
		return err
	}
	var un unstructured.Unstructured
	if e := yaml.Unmarshal(data, &un.Object); e != nil {
		return e
	}
	nav := un.Object["nav"].([]interface{})
	navitem, _ := findNavItem(nav, parent)
	if navitem == nil {
		return fmt.Errorf("Can't find '%s' nav item in mkdoc.yml", parent)
	}
	navitemmap := navitem.(map[interface{}]interface{})
	subnav := navitemmap[parent].([]interface{})
	subnav = removeNavItem(subnav, child)
	commands := make(map[string]interface{})
	commands[child] = files
	navitemmap[parent] = append(subnav, commands)

	newmkdocs, err := yaml.Marshal(un.Object)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("mkdocs.yml", newmkdocs, 0644)
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

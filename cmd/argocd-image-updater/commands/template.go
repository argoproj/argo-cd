package command

import (
	"fmt"
	"io/ioutil"
	"text/template"
	"time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/image-updater/argocd"
	"github.com/argoproj/argo-cd/v2/image-updater/image"
	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/image-updater/tag"
	"github.com/spf13/cobra"
)

func newTemplateCommand() *cobra.Command {
	var (
		commitMessageTemplatePath string
		tplStr                    string
	)
	var runCmd = &cobra.Command{
		Use:   "template [<PATH>]",
		Short: "Test & render a commit message template",
		Long: `
The template command lets you validate your commit message template. It will
parse the template at given PATH and execute it with a defined set of changes
so that you can see how it looks like when being templated by Image Updater.

If PATH is not given, will show you the default message that is used.
`,
		Run: func(cmd *cobra.Command, args []string) {
			var tpl *template.Template
			var err error
			if len(args) != 1 {
				tplStr = common.DefaultGitCommitMessage
			} else {
				commitMessageTemplatePath = args[0]
				tplData, err := ioutil.ReadFile(commitMessageTemplatePath)
				if err != nil {
					log.Fatalf("%v", err)
				}
				tplStr = string(tplData)
			}
			if tpl, err = template.New("commitMessage").Parse(tplStr); err != nil {
				log.Fatalf("could not parse commit message template: %v", err)
			}
			chL := []argocd.ChangeEntry{
				{
					Image:  image.NewFromIdentifier("gcr.io/example/example:1.0.0"),
					OldTag: tag.NewImageTag("1.0.0", time.Now(), ""),
					NewTag: tag.NewImageTag("1.0.1", time.Now(), ""),
				},
				{
					Image:  image.NewFromIdentifier("gcr.io/example/updater@sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2"),
					OldTag: tag.NewImageTag("", time.Now(), "sha256:01d09d19c2139a46aebfb577780d123d7396e97201bc7ead210a2ebff8239dee"),
					NewTag: tag.NewImageTag("", time.Now(), "sha256:7aa7a5359173d05b63cfd682e3c38487f3cb4f7f1d60659fe59fab1505977d4c"),
				},
			}
			fmt.Printf("%s\n", argocd.TemplateCommitMessage(tpl, "example-app", chL))
		},
	}
	return runCmd
}

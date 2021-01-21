package util

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/pkg/apis/application"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/gpg"
)

type ProjectOpts struct {
	Description              string
	destinations             []string
	Sources                  []string
	SignatureKeys            []string
	orphanedResourcesEnabled bool
	orphanedResourcesWarn    bool
}

func AddProjFlags(command *cobra.Command, opts *ProjectOpts) {
	command.Flags().StringVarP(&opts.Description, "description", "", "", "Project description")
	command.Flags().StringArrayVarP(&opts.destinations, "dest", "d", []string{},
		"Permitted destination server and namespace (e.g. https://192.168.99.100:8443,default)")
	command.Flags().StringArrayVarP(&opts.Sources, "src", "s", []string{}, "Permitted source repository URL")
	command.Flags().StringSliceVar(&opts.SignatureKeys, "signature-keys", []string{}, "GnuPG public key IDs for commit signature verification")
	command.Flags().BoolVar(&opts.orphanedResourcesEnabled, "orphaned-resources", false, "Enables orphaned resources monitoring")
	command.Flags().BoolVar(&opts.orphanedResourcesWarn, "orphaned-resources-warn", false, "Specifies if applications should be a warning condition when orphaned resources detected")
}

func (opts *ProjectOpts) GetDestinations() []v1alpha1.ApplicationDestination {
	destinations := make([]v1alpha1.ApplicationDestination, 0)
	for _, destStr := range opts.destinations {
		parts := strings.Split(destStr, ",")
		if len(parts) != 2 {
			log.Fatalf("Expected destination of the form: server,namespace. Received: %s", destStr)
		} else {
			destinations = append(destinations, v1alpha1.ApplicationDestination{
				Server:    parts[0],
				Namespace: parts[1],
			})
		}
	}
	return destinations
}

// TODO: Get configured keys and emit warning when a key is specified that is not configured
func (opts *ProjectOpts) GetSignatureKeys() []v1alpha1.SignatureKey {
	signatureKeys := make([]v1alpha1.SignatureKey, 0)
	for _, keyStr := range opts.SignatureKeys {
		if !gpg.IsShortKeyID(keyStr) && !gpg.IsLongKeyID(keyStr) {
			log.Fatalf("'%s' is not a valid GnuPG key ID", keyStr)
		}
		signatureKeys = append(signatureKeys, v1alpha1.SignatureKey{KeyID: gpg.KeyID(keyStr)})
	}
	return signatureKeys
}

func GetOrphanedResourcesSettings(c *cobra.Command, opts ProjectOpts) *v1alpha1.OrphanedResourcesMonitorSettings {
	warnChanged := c.Flag("orphaned-resources-warn").Changed
	if opts.orphanedResourcesEnabled || warnChanged {
		settings := v1alpha1.OrphanedResourcesMonitorSettings{}
		if warnChanged {
			settings.Warn = pointer.BoolPtr(opts.orphanedResourcesWarn)
		}
		return &settings
	}
	return nil
}

func readProjFromStdin(proj *v1alpha1.AppProject) error {
	reader := bufio.NewReader(os.Stdin)
	err := config.UnmarshalReader(reader, &proj)
	if err != nil {
		return fmt.Errorf("unable to read manifest from stdin: %v", err)
	}
	return nil
}

func readProjFromURI(fileURL string, proj *v1alpha1.AppProject) error {
	parsedURL, err := url.ParseRequestURI(fileURL)
	if err != nil || !(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
		err = config.UnmarshalLocalFile(fileURL, &proj)
	} else {
		err = config.UnmarshalRemoteFile(fileURL, &proj)
	}
	return err
}

func ConstructAppProj(fileURL string, args []string, opts ProjectOpts, c *cobra.Command) (*v1alpha1.AppProject, error) {
	var proj v1alpha1.AppProject
	if fileURL == "-" {
		// read stdin
		err := readProjFromStdin(&proj)
		if err != nil {
			return nil, err
		}
	} else if fileURL != "" {
		// read uri
		err := readProjFromURI(fileURL, &proj)
		if err != nil {
			return nil, err
		}

		if len(args) == 1 && args[0] != proj.Name {
			return nil, fmt.Errorf("project name '%s' does not match project spec metadata.name '%s'", args[0], proj.Name)
		}
	} else {
		// read arguments
		if len(args) == 0 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		projName := args[0]
		proj = v1alpha1.AppProject{
			TypeMeta: v1.TypeMeta{
				Kind:       application.AppProjectKind,
				APIVersion: application.Group + "/v1aplha1",
			},
			ObjectMeta: v1.ObjectMeta{Name: projName},
			Spec: v1alpha1.AppProjectSpec{
				Description:       opts.Description,
				Destinations:      opts.GetDestinations(),
				SourceRepos:       opts.Sources,
				SignatureKeys:     opts.GetSignatureKeys(),
				OrphanedResources: GetOrphanedResourcesSettings(c, opts),
			},
		}
	}

	return &proj, nil
}

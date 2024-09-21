package util

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/config"
	"github.com/argoproj/argo-cd/v2/util/gpg"
)

type ProjectOpts struct {
	Description                string
	destinations               []string
	destinationServiceAccounts []string
	Sources                    []string
	SignatureKeys              []string
	SourceNamespaces           []string

	orphanedResourcesEnabled   bool
	orphanedResourcesWarn      bool
	allowedClusterResources    []string
	deniedClusterResources     []string
	allowedNamespacedResources []string
	deniedNamespacedResources  []string
}

func AddProjFlags(command *cobra.Command, opts *ProjectOpts) {
	command.Flags().StringVarP(&opts.Description, "description", "", "", "Project description")
	command.Flags().StringArrayVarP(&opts.destinations, "dest", "d", []string{},
		"Permitted destination server and namespace (e.g. https://192.168.99.100:8443,default)")
	command.Flags().StringArrayVarP(&opts.Sources, "src", "s", []string{}, "Permitted source repository URL")
	command.Flags().StringSliceVar(&opts.SignatureKeys, "signature-keys", []string{}, "GnuPG public key IDs for commit signature verification")
	command.Flags().BoolVar(&opts.orphanedResourcesEnabled, "orphaned-resources", false, "Enables orphaned resources monitoring")
	command.Flags().BoolVar(&opts.orphanedResourcesWarn, "orphaned-resources-warn", false, "Specifies if applications should have a warning condition when orphaned resources detected")
	command.Flags().StringArrayVar(&opts.allowedClusterResources, "allow-cluster-resource", []string{}, "List of allowed cluster level resources")
	command.Flags().StringArrayVar(&opts.deniedClusterResources, "deny-cluster-resource", []string{}, "List of denied cluster level resources")
	command.Flags().StringArrayVar(&opts.allowedNamespacedResources, "allow-namespaced-resource", []string{}, "List of allowed namespaced resources")
	command.Flags().StringArrayVar(&opts.deniedNamespacedResources, "deny-namespaced-resource", []string{}, "List of denied namespaced resources")
	command.Flags().StringSliceVar(&opts.SourceNamespaces, "source-namespaces", []string{}, "List of source namespaces for applications")
}

func getGroupKindList(values []string) []v1.GroupKind {
	var res []v1.GroupKind
	for _, val := range values {
		if parts := strings.Split(val, "/"); len(parts) == 2 {
			res = append(res, v1.GroupKind{Group: parts[0], Kind: parts[1]})
		} else if len(parts) == 1 {
			res = append(res, v1.GroupKind{Kind: parts[0]})
		}
	}
	return res
}

func (opts *ProjectOpts) GetAllowedClusterResources() []v1.GroupKind {
	return getGroupKindList(opts.allowedClusterResources)
}

func (opts *ProjectOpts) GetDeniedClusterResources() []v1.GroupKind {
	return getGroupKindList(opts.deniedClusterResources)
}

func (opts *ProjectOpts) GetAllowedNamespacedResources() []v1.GroupKind {
	return getGroupKindList(opts.allowedNamespacedResources)
}

func (opts *ProjectOpts) GetDeniedNamespacedResources() []v1.GroupKind {
	return getGroupKindList(opts.deniedNamespacedResources)
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

func (opts *ProjectOpts) GetDestinationServiceAccounts() []v1alpha1.ApplicationDestinationServiceAccount {
	destinationServiceAccounts := make([]v1alpha1.ApplicationDestinationServiceAccount, 0)
	for _, destStr := range opts.destinationServiceAccounts {
		parts := strings.Split(destStr, ",")
		if len(parts) != 2 {
			log.Fatalf("Expected destination of the form: server,namespace. Received: %s", destStr)
		} else {
			destinationServiceAccounts = append(destinationServiceAccounts, v1alpha1.ApplicationDestinationServiceAccount{
				Server:                parts[0],
				Namespace:             parts[1],
				DefaultServiceAccount: parts[2],
			})
		}
	}
	return destinationServiceAccounts
}

// GetSignatureKeys TODO: Get configured keys and emit warning when a key is specified that is not configured
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

func (opts *ProjectOpts) GetSourceNamespaces() []string {
	return opts.SourceNamespaces
}

func GetOrphanedResourcesSettings(flagSet *pflag.FlagSet, opts ProjectOpts) *v1alpha1.OrphanedResourcesMonitorSettings {
	warnChanged := flagSet.Changed("orphaned-resources-warn")
	if opts.orphanedResourcesEnabled || warnChanged {
		settings := v1alpha1.OrphanedResourcesMonitorSettings{}
		if warnChanged {
			settings.Warn = ptr.To(opts.orphanedResourcesWarn)
		}
		return &settings
	}
	return nil
}

func readProjFromStdin(proj *v1alpha1.AppProject) error {
	reader := bufio.NewReader(os.Stdin)
	err := config.UnmarshalReader(reader, &proj)
	if err != nil {
		return fmt.Errorf("unable to read manifest from stdin: %w", err)
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
	if err != nil {
		return fmt.Errorf("error reading proj from uri: %w", err)
	}
	return nil
}

func SetProjSpecOptions(flags *pflag.FlagSet, spec *v1alpha1.AppProjectSpec, projOpts *ProjectOpts) int {
	visited := 0
	flags.Visit(func(f *pflag.Flag) {
		visited++
		switch f.Name {
		case "description":
			spec.Description = projOpts.Description
		case "dest":
			spec.Destinations = projOpts.GetDestinations()
		case "src":
			spec.SourceRepos = projOpts.Sources
		case "signature-keys":
			spec.SignatureKeys = projOpts.GetSignatureKeys()
		case "allow-cluster-resource":
			spec.ClusterResourceWhitelist = projOpts.GetAllowedClusterResources()
		case "deny-cluster-resource":
			spec.ClusterResourceBlacklist = projOpts.GetDeniedClusterResources()
		case "allow-namespaced-resource":
			spec.NamespaceResourceWhitelist = projOpts.GetAllowedNamespacedResources()
		case "deny-namespaced-resource":
			spec.NamespaceResourceBlacklist = projOpts.GetDeniedNamespacedResources()
		case "source-namespaces":
			spec.SourceNamespaces = projOpts.GetSourceNamespaces()
		case "dest-service-accounts":
			spec.DestinationServiceAccounts = projOpts.GetDestinationServiceAccounts()
		}
	})
	if flags.Changed("orphaned-resources") || flags.Changed("orphaned-resources-warn") {
		spec.OrphanedResources = GetOrphanedResourcesSettings(flags, *projOpts)
		visited++
	}
	return visited
}

func ConstructAppProj(fileURL string, args []string, opts ProjectOpts, c *cobra.Command) (*v1alpha1.AppProject, error) {
	proj := v1alpha1.AppProject{
		TypeMeta: v1.TypeMeta{
			Kind:       application.AppProjectKind,
			APIVersion: application.Group + "/v1alpha1",
		},
	}
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
		proj.Name = args[0]
	}
	SetProjSpecOptions(c.Flags(), &proj.Spec, &opts)
	return &proj, nil
}

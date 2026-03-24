package commands

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/gpgkey"
	projectpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/sourceintegrity"
	"github.com/argoproj/argo-cd/v3/util/templates"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

const (
	msgNoGitPolicies = "No source integrity git policies defined for project"
	msgExamples      = `
# List git policies
argocd proj source-integrity git policies list PROJECT

# Add a new git policy
argocd proj source-integrity git policies add PROJECT ...

# Update a git policy
argocd proj source-integrity git policies update PROJECT POLICY_ID ...

# Delete a git policy
argocd proj source-integrity git policies delete PROJECT POLICY_ID
`
)

var (
	// Indirections needed for a client lookup during tests. Mocks are injected here.
	newProjectClient = func(clientOpts *argocdclient.ClientOptions, c *cobra.Command) (io.Closer, projectpkg.ProjectServiceClient) {
		return headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
	}
	newGpgKeyClient = func(clientOpts *argocdclient.ClientOptions, c *cobra.Command) (io.Closer, gpgkey.GPGKeyServiceClient) {
		return headless.NewClientOrDie(clientOpts, c).NewGPGKeyClientOrDie()
	}
)

// NewProjectSourceIntegrityCommand returns a new instance of an `argocd proj source-integrity` command
func NewProjectSourceIntegrityCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:     "source-integrity",
		Short:   "Manage criteria for source integrity",
		Example: templates.Examples(msgExamples),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewProjectSourceIntegrityGitCommand(clientOpts))
	return command
}

// NewProjectSourceIntegrityGitCommand returns a new instance of an `argocd proj source-integrity git` command
func NewProjectSourceIntegrityGitCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:     "git",
		Short:   "Manage policies for Git repositories",
		Example: templates.Examples(msgExamples),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewProjectSourceIntegrityGitPoliciesCommand(clientOpts))
	return command
}

// NewProjectSourceIntegrityGitPoliciesCommand returns a new instance of an `argocd proj source-integrity git policies` command
func NewProjectSourceIntegrityGitPoliciesCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:     "policies",
		Short:   "Manage git source integrity policies",
		Example: templates.Examples(msgExamples),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewProjectSourceIntegrityGitPoliciesListCommand(clientOpts))
	command.AddCommand(NewProjectSourceIntegrityGitPoliciesAddCommand(clientOpts))
	command.AddCommand(NewProjectSourceIntegrityGitPoliciesDeleteCommand(clientOpts))
	command.AddCommand(NewProjectSourceIntegrityGitPoliciesUpdateCommand(clientOpts))
	return command
}

// NewProjectSourceIntegrityGitPoliciesListCommand returns a new instance of an `argocd proj source-integrity git policies list` command
func NewProjectSourceIntegrityGitPoliciesListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "list PROJECT",
		Short: "List git source integrity policies",
		Long:  "List git source integrity policies.\n\nThe listed policy ID is used to identify given policy in delete and update commands.",
		Example: templates.Examples(`
			# List all git policies for project PROJECT.
			argocd proj source-integrity git policies list PROJECT
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			if proj.Spec.SourceIntegrity == nil || proj.Spec.SourceIntegrity.Git == nil || len(proj.Spec.SourceIntegrity.Git.Policies) == 0 {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), msgNoGitPolicies)
				return
			}

			listGitGpgPolicies(c.OutOrStdout(), proj)
		},
	}
	return command
}

func listGitGpgPolicies(out io.Writer, proj *v1alpha1.AppProject) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "ID\tGPG-MODE\tGPG-KEYS\tREPO-URLS\n")
	for i, policy := range proj.Spec.SourceIntegrity.Git.Policies {
		gpgMode := "<none>"
		gpgKeys := "<none>"
		if policy.GPG != nil {
			gpgMode = string(policy.GPG.Mode)
			if len(policy.GPG.Keys) > 0 {
				keys := make([]string, len(policy.GPG.Keys))
				copy(keys, policy.GPG.Keys)
				gpgKeys = strings.Join(keys, ", ")
			}
		}

		repoURLs := "<none>"
		if len(policy.Repos) > 0 {
			urls := make([]string, len(policy.Repos))
			for j, repo := range policy.Repos {
				urls[j] = repo.URL
			}
			repoURLs = strings.Join(urls, ", ")
		}

		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i, gpgMode, gpgKeys, repoURLs)
	}
	_ = w.Flush()
}

// NewProjectSourceIntegrityGitPoliciesDeleteCommand returns a new instance of an `argocd proj source-integrity git policies delete` command
func NewProjectSourceIntegrityGitPoliciesDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "delete PROJECT POLICY_ID...",
		Short: "Delete a git source integrity policy",
		Example: templates.Examples(`
			# Delete git policy at index 1 and 3 from project named PROJECT
			argocd proj source-integrity git policies delete PROJECT 1 3
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) < 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			if proj.Spec.SourceIntegrity == nil || proj.Spec.SourceIntegrity.Git == nil || len(proj.Spec.SourceIntegrity.Git.Policies) == 0 {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), msgNoGitPolicies)
				return
			}

			originalPolicyCount := len(proj.Spec.SourceIntegrity.Git.Policies)
			indicesToDelete := make(map[int]any)
			for _, policyId := range args[1:] {
				index, err := strconv.Atoi(policyId)
				errors.CheckError(err)
				if index < 0 || index >= originalPolicyCount {
					log.Fatalf("POLICY_ID %d is out of range (0-%d)", index, originalPolicyCount-1)
				}

				indicesToDelete[index] = nil
			}

			// Build a new slice with only policies whose indices are not in the set
			newPolicies := make([]*v1alpha1.SourceIntegrityGitPolicy, 0, originalPolicyCount-len(indicesToDelete))
			for i, policy := range proj.Spec.SourceIntegrity.Git.Policies {
				if _, shouldDelete := indicesToDelete[i]; !shouldDelete {
					newPolicies = append(newPolicies, policy)
				}
			}
			proj.Spec.SourceIntegrity.Git.Policies = newPolicies

			cleanupSourceIntegrityIfEmpty(proj)

			_, err = projects.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// cleanupSourceIntegrityIfEmpty removes spec.sourceIntegrity from a project if policies are emptied
func cleanupSourceIntegrityIfEmpty(proj *v1alpha1.AppProject) {
	if proj.Spec.SourceIntegrity != nil && proj.Spec.SourceIntegrity.Git != nil && len(proj.Spec.SourceIntegrity.Git.Policies) == 0 {
		proj.Spec.SourceIntegrity.Git = nil
		proj.Spec.SourceIntegrity = nil
	}
}

// NewProjectSourceIntegrityGitPoliciesAddCommand returns a new instance of an `argocd proj source-integrity git policies add` command
func NewProjectSourceIntegrityGitPoliciesAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		repoURLs []string
		gpgMode  string
		gpgKeys  []string
	)
	command := &cobra.Command{
		Use:   "add PROJECT",
		Short: "Add a git source integrity policy",
		Example: templates.Examples(`
			# Add a new git policy with repo URLs and GPG settings
			argocd proj source-integrity git policies add PROJECT \
				--repo-url 'https://github.com/foo/*' \
				--gpg-mode strict \
				--gpg-key D56C4FCA57A46444
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			projName := args[0]
			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			newPolicy := v1alpha1.SourceIntegrityGitPolicy{GPG: &v1alpha1.SourceIntegrityGitPolicyGPG{
				Mode: validateGpgMode(gpgMode),
			}}
			for _, url := range repoURLs {
				newPolicy.Repos = append(newPolicy.Repos, v1alpha1.SourceIntegrityGitPolicyRepo{URL: url})
			}
			for _, key := range gpgKeys {
				_, err := sourceintegrity.KeyID(key)
				if err != nil {
					log.Fatalf("Invalid GPG key ID '%s': %v", key, err)
				}
			}
			newPolicy.GPG.Keys = gpgKeys

			closer, gpgKeyClient := newGpgKeyClient(clientOpts, c)
			defer utilio.Close(closer)
			warnOnProblems(c.ErrOrStderr(), &newPolicy, gpgKeyClient)

			if proj.Spec.SourceIntegrity == nil {
				proj.Spec.SourceIntegrity = &v1alpha1.SourceIntegrity{}
			}
			if proj.Spec.SourceIntegrity.Git == nil {
				proj.Spec.SourceIntegrity.Git = &v1alpha1.SourceIntegrityGit{}
			}
			proj.Spec.SourceIntegrity.Git.Policies = append(proj.Spec.SourceIntegrity.Git.Policies, &newPolicy)

			_, err = projects.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)

			// Print resulting policies out of convenience
			listGitGpgPolicies(c.OutOrStdout(), proj)
		},
	}
	command.Flags().StringSliceVar(&repoURLs, "repo-url", []string{}, "Repository URL pattern (can be repeated)")
	command.Flags().StringVar(&gpgMode, "gpg-mode", "", "GPG verification mode (strict, head, or none)")
	command.Flags().StringSliceVar(&gpgKeys, "gpg-key", []string{}, "GPG key ID (can be repeated)")
	return command
}

func validateGpgMode(gpgMode string) v1alpha1.SourceIntegrityGitPolicyGPGMode {
	out := v1alpha1.SourceIntegrityGitPolicyGPGMode(gpgMode)
	if out != v1alpha1.SourceIntegrityGitPolicyGPGModeStrict &&
		out != v1alpha1.SourceIntegrityGitPolicyGPGModeHead &&
		out != v1alpha1.SourceIntegrityGitPolicyGPGModeNone {
		log.Fatal("gpg-mode must be one of: strict, head, none")
	}
	return out
}

// NewProjectSourceIntegrityGitPoliciesUpdateCommand returns a new instance of an `argocd proj source-integrity git policies update` command
func NewProjectSourceIntegrityGitPoliciesUpdateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		repoURLs       []string
		deleteRepoURLs []string
		addRepoURLs    []string
		gpgMode        string
		gpgKeys        []string
		deleteGPGKeys  []string
		addGPGKeys     []string
	)
	command := &cobra.Command{
		Use:   "update PROJECT POLICY_ID",
		Short: "Update a git source integrity policy",
		Example: templates.Examples(`
			# Update policy at index to set specific repo URLs, removing the old ones
			argocd proj source-integrity git policies update PROJECT POLICY_ID \
				--repo-url 'https://github.com/foo/*'

			# Update policy at index to add and remove repo URLs
			argocd proj source-integrity git policies update PROJECT POLICY_ID \
				--add-repo-url 'https://github.com/bar/*' \
				--delete-repo-url 'https://github.com/foo/*'

			# Update policy GPG mode and keys
			argocd proj source-integrity git policies update PROJECT POLICY_ID \
				--gpg-mode strict \
				--add-gpg-key D56C4FCA57A46444
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			projName := args[0]
			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			if proj.Spec.SourceIntegrity == nil || proj.Spec.SourceIntegrity.Git == nil || len(proj.Spec.SourceIntegrity.Git.Policies) == 0 {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), msgNoGitPolicies)
				return
			}

			index, err := strconv.Atoi(args[1])
			errors.CheckError(err)
			originalPolicyCount := len(proj.Spec.SourceIntegrity.Git.Policies)
			if index < 0 || index >= originalPolicyCount {
				log.Fatalf("POLICY_ID %d is out of range (0-%d)", index, originalPolicyCount-1)
			}

			if len(gpgKeys) > 0 && (len(deleteGPGKeys) > 0 || len(addGPGKeys) > 0) {
				log.Fatal("Option --gpg-key, cannot be combined with --add-gpg-key or --delete-gpg-key")
			}

			if len(repoURLs) > 0 && (len(deleteRepoURLs) > 0 || len(addRepoURLs) > 0) {
				log.Fatal("Option --repo-url, cannot be combined with --add-repo-url or --delete-repo-url")
			}

			if len(repoURLs) > 0 {
				var repos []v1alpha1.SourceIntegrityGitPolicyRepo
				for _, url := range repoURLs {
					repos = append(repos, v1alpha1.SourceIntegrityGitPolicyRepo{URL: url})
				}
				proj.Spec.SourceIntegrity.Git.Policies[index].Repos = repos
			}
			for _, url := range deleteRepoURLs {
				for i := 0; i < len(proj.Spec.SourceIntegrity.Git.Policies[index].Repos); i++ {
					if proj.Spec.SourceIntegrity.Git.Policies[index].Repos[i].URL == url {
						proj.Spec.SourceIntegrity.Git.Policies[index].Repos = append(proj.Spec.SourceIntegrity.Git.Policies[index].Repos[:i], proj.Spec.SourceIntegrity.Git.Policies[index].Repos[i+1:]...)
						i--
					}
				}
			}
			for _, url := range addRepoURLs {
				found := false
				for _, repo := range proj.Spec.SourceIntegrity.Git.Policies[index].Repos {
					if repo.URL == url {
						found = true
						break
					}
				}
				if !found {
					proj.Spec.SourceIntegrity.Git.Policies[index].Repos = append(proj.Spec.SourceIntegrity.Git.Policies[index].Repos, v1alpha1.SourceIntegrityGitPolicyRepo{URL: url})
				}
			}

			// There are no other types for now, so turning GPG is the only sensible thing
			if proj.Spec.SourceIntegrity.Git.Policies[index].GPG == nil {
				proj.Spec.SourceIntegrity.Git.Policies[index].GPG = &v1alpha1.SourceIntegrityGitPolicyGPG{}
			}

			// Reset mode
			if gpgMode != "" {
				proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Mode = validateGpgMode(gpgMode)
			} else if proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Mode == "" {
				// The policy is updated to a gpg, but this mandatory field is unset
				log.Fatal("gpg-mode must be set")
			}

			// Reset keys
			if len(gpgKeys) > 0 {
				for _, key := range gpgKeys {
					_, err := sourceintegrity.KeyID(key)
					if err != nil {
						log.Fatalf("Invalid GPG key ID '%s': %v", key, err)
					}
				}
				proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys = gpgKeys
			}
			for _, key := range deleteGPGKeys {
				for i := 0; i < len(proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys); i++ {
					if proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys[i] == key {
						proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys = append(proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys[:i], proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys[i+1:]...)
						i--
					}
				}
			}
			for _, key := range addGPGKeys {
				_, err := sourceintegrity.KeyID(key)
				if err != nil {
					log.Fatalf("Invalid GPG key ID '%s': %v", key, err)
				}
				found := slices.Contains(proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys, key)
				if !found {
					proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys = append(proj.Spec.SourceIntegrity.Git.Policies[index].GPG.Keys, key)
				}
			}

			closer, gpgKeyClient := newGpgKeyClient(clientOpts, c)
			defer utilio.Close(closer)
			warnOnProblems(c.ErrOrStderr(), proj.Spec.SourceIntegrity.Git.Policies[index], gpgKeyClient)

			_, err = projects.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)

			// Print resulting policies out of convenience
			listGitGpgPolicies(c.OutOrStdout(), proj)
		},
	}
	command.Flags().StringVar(&gpgMode, "gpg-mode", "", "Set GPG verification mode (strict, head, or none)")
	command.Flags().StringSliceVar(&repoURLs, "repo-url", []string{}, "Set repository URL pattern (replaces existing)")
	command.Flags().StringSliceVar(&addRepoURLs, "add-repo-url", []string{}, "Add repository URL pattern")
	command.Flags().StringSliceVar(&deleteRepoURLs, "delete-repo-url", []string{}, "Delete repository URL pattern")
	command.Flags().StringSliceVar(&gpgKeys, "gpg-key", []string{}, "Set GPG key ID (replaces existing)")
	command.Flags().StringSliceVar(&addGPGKeys, "add-gpg-key", []string{}, "Add GPG key ID")
	command.Flags().StringSliceVar(&deleteGPGKeys, "delete-gpg-key", []string{}, "Delete GPG key ID")
	return command
}

// warnOnProblems checks if a policy has empty repo URLs or GPG keys and prints warnings
func warnOnProblems(stderr io.Writer, policy *v1alpha1.SourceIntegrityGitPolicy, gpgKeyClient gpgkey.GPGKeyServiceClient) {
	if len(policy.Repos) == 0 {
		_, _ = fmt.Fprintln(stderr, "Warning: Policy has no repository URLs and will never be used")
	}
	if policy.GPG != nil && len(policy.GPG.Keys) == 0 {
		_, _ = fmt.Fprintln(stderr, "Warning: Policy has no GPG keys and will never validate any revision")
	} else {
		absent := make(map[string]any)
		for _, key := range policy.GPG.Keys {
			absent[key] = nil
		}

		keyring, err := gpgKeyClient.List(context.TODO(), &gpgkey.GnuPGPublicKeyQuery{})
		errors.CheckError(err)
		for _, keyringKey := range keyring.Items {
			delete(absent, keyringKey.KeyID)
		}

		if len(absent) != 0 {
			absentKeys := slices.Collect(maps.Keys(absent))
			slices.Sort(absentKeys)
			_, _ = fmt.Fprintf(stderr,
				"Warning: Following GPG keys are not in repo-server keyring: %s\n",
				strings.Join(absentKeys, ", "),
			)
		}
	}
}

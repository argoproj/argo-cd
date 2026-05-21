package commands

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/gpgkey"
	projectpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/cli"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/sourceintegrity"
	"github.com/argoproj/argo-cd/v3/util/templates"
)

const (
	msgNoGitPolicies = "No source integrity git policies defined for project %q"
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
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			if err != nil {
				return fmt.Errorf("Failed getting project %q: %w", projName, err)
			}

			if proj.Spec.SourceIntegrity == nil || proj.Spec.SourceIntegrity.Git == nil || len(proj.Spec.SourceIntegrity.Git.Policies) == 0 {
				return fmt.Errorf(msgNoGitPolicies, projName)
			}

			listGitGpgPolicies(c.OutOrStdout(), proj)
			return nil
		},
	}
	return command
}

func listGitGpgPolicies(out io.Writer, proj *v1alpha1.AppProject) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tGPG-MODE\tGPG-KEYS\tREPO-URLS")
	for i, policy := range proj.Spec.SourceIntegrity.Git.Policies {
		gpgMode := "<none>"
		gpgKeys := "<none>"
		if policy.GPG != nil {
			gpgMode = string(policy.GPG.Mode)
			if len(policy.GPG.Keys) > 0 {
				gpgKeys = strings.Join(policy.GPG.Keys, ", ")
			}
		}

		repoURLs := "<none>"
		if len(policy.Repos) > 0 {
			urls := make([]string, 0, len(policy.Repos))
			for _, repo := range policy.Repos {
				urls = append(urls, repo.URL)
			}
			repoURLs = strings.Join(urls, ", ")
		}

		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i, gpgMode, gpgKeys, repoURLs)
	}
	_ = w.Flush()
}

// NewProjectSourceIntegrityGitPoliciesDeleteCommand returns a new instance of an `argocd proj source-integrity git policies delete` command
func NewProjectSourceIntegrityGitPoliciesDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete PROJECT POLICY_ID...",
		Short: "Delete a git source integrity policy",
		Example: templates.Examples(`
			# Delete git policy at index 1 and 3 from project named PROJECT
			argocd proj source-integrity git policies delete PROJECT 1 3
		`),
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			if len(args) < 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			if err != nil {
				return fmt.Errorf("Failed getting project %q: %w", projName, err)
			}

			if proj.Spec.SourceIntegrity == nil || proj.Spec.SourceIntegrity.Git == nil || len(proj.Spec.SourceIntegrity.Git.Policies) == 0 {
				return fmt.Errorf(msgNoGitPolicies, projName)
			}

			originalPolicyCount := len(proj.Spec.SourceIntegrity.Git.Policies)
			idsToDelete := make(map[int]any)
			for _, policyId := range args[1:] {
				index, err := strconv.Atoi(policyId)
				if err != nil {
					return fmt.Errorf("Invalid POLICY_ID '%s'", args[1])
				}

				if index < 0 || index >= originalPolicyCount {
					return fmt.Errorf("POLICY_ID %d is out of range (0-%d)", index, originalPolicyCount-1)
				}

				idsToDelete[index] = nil
			}

			if !yes {
				idsStr := make([]string, 0, len(idsToDelete))
				for i := range maps.Keys(idsToDelete) {
					idsStr = append(idsStr, strconv.Itoa(i))
				}
				sort.Strings(idsStr)
				prompt := fmt.Sprintf("Are you sure you want to delete policie(s) %s from project %q? [y/N] ", strings.Join(idsStr, ", "), projName)
				if cli.AskToProceedS(prompt) != "y" {
					fmt.Fprintln(c.OutOrStdout(), "Aborted by user.")
					return nil
				}
			}

			// Build a new slice with only policies whose indices are not in the set
			newPolicies := make([]*v1alpha1.SourceIntegrityGitPolicy, 0, originalPolicyCount-len(idsToDelete))
			for i, policy := range proj.Spec.SourceIntegrity.Git.Policies {
				if _, shouldDelete := idsToDelete[i]; !shouldDelete {
					newPolicies = append(newPolicies, policy)
				}
			}
			proj.Spec.SourceIntegrity.Git.Policies = newPolicies

			cleanupSourceIntegrityIfEmpty(proj)

			_, err = projects.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			if err != nil {
				return fmt.Errorf("Failed updating project %q: %w", projName, err)
			}

			return nil
		},
	}
	command.Flags().BoolVarP(&yes, "yes", "y", false, "Skip explicit confirmation")
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
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			projName := args[0]
			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			if err != nil {
				return fmt.Errorf("Failed getting project %q: %w", projName, err)
			}

			mode, err := validateGpgMode(gpgMode)
			if err != nil {
				return err
			}
			newPolicy := v1alpha1.SourceIntegrityGitPolicy{GPG: &v1alpha1.SourceIntegrityGitPolicyGPG{Mode: mode}}
			for _, url := range repoURLs {
				newPolicy.Repos = append(newPolicy.Repos, v1alpha1.SourceIntegrityGitPolicyRepo{URL: url})
			}
			for _, key := range gpgKeys {
				_, err := sourceintegrity.KeyID(key)
				if err != nil {
					return fmt.Errorf("Invalid GPG key ID '%s': %w", key, err)
				}
			}
			newPolicy.GPG.Keys = gpgKeys

			closer, gpgKeyClient := newGpgKeyClient(clientOpts, c)
			defer utilio.Close(closer)
			if err := warnOnProblems(c, &newPolicy, gpgKeyClient); err != nil {
				return err
			}

			if proj.Spec.SourceIntegrity == nil {
				proj.Spec.SourceIntegrity = &v1alpha1.SourceIntegrity{}
			}
			if proj.Spec.SourceIntegrity.Git == nil {
				proj.Spec.SourceIntegrity.Git = &v1alpha1.SourceIntegrityGit{}
			}
			proj.Spec.SourceIntegrity.Git.Policies = append(proj.Spec.SourceIntegrity.Git.Policies, &newPolicy)

			_, err = projects.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			if err != nil {
				return fmt.Errorf("Failed updating project %q: %w", projName, err)
			}

			// Print resulting policies out of convenience
			listGitGpgPolicies(c.OutOrStdout(), proj)
			return nil
		},
	}
	command.Flags().StringSliceVar(&repoURLs, "repo-url", []string{}, "Repository URL pattern (can be repeated)")
	command.Flags().StringVar(&gpgMode, "gpg-mode", "", "GPG verification mode (strict, head, or none)")
	command.Flags().StringSliceVar(&gpgKeys, "gpg-key", []string{}, "GPG key ID (can be repeated)")
	return command
}

func validateGpgMode(gpgMode string) (v1alpha1.SourceIntegrityGitPolicyGPGMode, error) {
	out := v1alpha1.SourceIntegrityGitPolicyGPGMode(gpgMode)

	switch gpgMode {
	case "strict", "head", "none":
		return out, nil
	case "":
		return out, errors.New("gpg-mode must be set")
	default:
		return out, errors.New("gpg-mode must be one of: strict, head, none")
	}
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
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			cleanup, projects := newProjectClient(clientOpts, c)
			defer utilio.Close(cleanup)

			projName := args[0]
			proj, err := projects.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			if err != nil {
				return fmt.Errorf("Failed getting project %q: %w", projName, err)
			}

			if proj.Spec.SourceIntegrity == nil || proj.Spec.SourceIntegrity.Git == nil || len(proj.Spec.SourceIntegrity.Git.Policies) == 0 {
				return fmt.Errorf(msgNoGitPolicies, projName)
			}

			index, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("Invalid POLICY_ID '%s'", args[1])
			}
			originalPolicyCount := len(proj.Spec.SourceIntegrity.Git.Policies)
			if index < 0 || index >= originalPolicyCount {
				return fmt.Errorf("POLICY_ID %d is out of range (0-%d)", index, originalPolicyCount-1)
			}
			policy := proj.Spec.SourceIntegrity.Git.Policies[index]

			if len(gpgKeys) > 0 && (len(deleteGPGKeys) > 0 || len(addGPGKeys) > 0) {
				return errors.New("Option --gpg-key, cannot be combined with --add-gpg-key or --delete-gpg-key")
			}

			if len(repoURLs) > 0 && (len(deleteRepoURLs) > 0 || len(addRepoURLs) > 0) {
				return errors.New("Option --repo-url, cannot be combined with --add-repo-url or --delete-repo-url")
			}

			if len(repoURLs) > 0 {
				var repos []v1alpha1.SourceIntegrityGitPolicyRepo
				for _, url := range repoURLs {
					repos = append(repos, v1alpha1.SourceIntegrityGitPolicyRepo{URL: url})
				}
				policy.Repos = repos
			}
			for _, url := range deleteRepoURLs {
				for i := 0; i < len(policy.Repos); i++ {
					if policy.Repos[i].URL == url {
						policy.Repos = append(policy.Repos[:i], policy.Repos[i+1:]...)
						i--
					}
				}
			}
			for _, url := range addRepoURLs {
				found := false
				for _, repo := range policy.Repos {
					if repo.URL == url {
						found = true
						break
					}
				}
				if !found {
					policy.Repos = append(policy.Repos, v1alpha1.SourceIntegrityGitPolicyRepo{URL: url})
				}
			}

			// There are no other types for now, so turning GPG is the only sensible thing
			if policy.GPG == nil {
				policy.GPG = &v1alpha1.SourceIntegrityGitPolicyGPG{}
			}

			// Reset mode
			if gpgMode != "" {
				mode, err := validateGpgMode(gpgMode)
				if err != nil {
					return err
				}
				policy.GPG.Mode = mode
			} else if policy.GPG.Mode == "" {
				// The policy is updated to a gpg one, but this mandatory field is unset
				return errors.New("gpg-mode must be set")
			}

			// Reset keys
			if len(gpgKeys) > 0 {
				for _, key := range gpgKeys {
					_, err := sourceintegrity.KeyID(key)
					if err != nil {
						return fmt.Errorf("Invalid GPG key ID '%s': %w", key, err)
					}
				}
				policy.GPG.Keys = gpgKeys
			}
			for _, key := range deleteGPGKeys {
				for i := 0; i < len(policy.GPG.Keys); i++ {
					if policy.GPG.Keys[i] == key {
						policy.GPG.Keys = append(policy.GPG.Keys[:i], policy.GPG.Keys[i+1:]...)
						i--
					}
				}
			}
			for _, key := range addGPGKeys {
				_, err := sourceintegrity.KeyID(key)
				if err != nil {
					return fmt.Errorf("Invalid GPG key ID '%s': %w", key, err)
				}
				found := slices.Contains(policy.GPG.Keys, key)
				if !found {
					policy.GPG.Keys = append(policy.GPG.Keys, key)
				}
			}

			closer, gpgKeyClient := newGpgKeyClient(clientOpts, c)
			defer utilio.Close(closer)
			if err := warnOnProblems(c, policy, gpgKeyClient); err != nil {
				return err
			}

			_, err = projects.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			if err != nil {
				return fmt.Errorf("failed updating project %q: %w", projName, err)
			}

			// Print resulting policies out of convenience
			listGitGpgPolicies(c.OutOrStdout(), proj)
			return nil
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
func warnOnProblems(c *cobra.Command, policy *v1alpha1.SourceIntegrityGitPolicy, gpgKeyClient gpgkey.GPGKeyServiceClient) error {
	stderr := c.ErrOrStderr()
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

		keyring, err := gpgKeyClient.List(c.Context(), &gpgkey.GnuPGPublicKeyQuery{})
		if err != nil {
			return fmt.Errorf("failed listing GPG keys: %w", err)
		}
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

	return nil
}

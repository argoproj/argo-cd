/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/argoproj/argo-cd/v2/common"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Utility struct for a reference to a secret key.
type SecretRef struct {
	SecretName string `json:"secretName" protobuf:"bytes,1,opt,name=secretName"`
	Key        string `json:"key" protobuf:"bytes,2,opt,name=key"`
}

// ApplicationSet is a set of Application resources
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=applicationsets,shortName=appset;appsets
// +kubebuilder:subresource:status
type ApplicationSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ApplicationSetSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            ApplicationSetStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ApplicationSetSpec represents a class of application set state.
type ApplicationSetSpec struct {
	GoTemplate bool                      `json:"goTemplate,omitempty" protobuf:"bytes,1,name=goTemplate"`
	Generators []ApplicationSetGenerator `json:"generators" protobuf:"bytes,2,name=generators"`
	Template   ApplicationSetTemplate    `json:"template" protobuf:"bytes,3,name=template"`
	SyncPolicy *ApplicationSetSyncPolicy `json:"syncPolicy,omitempty" protobuf:"bytes,4,name=syncPolicy"`
}

// ApplicationSetSyncPolicy configures how generated Applications will relate to their
// ApplicationSet.
type ApplicationSetSyncPolicy struct {
	// PreserveResourcesOnDeletion will preserve resources on deletion. If PreserveResourcesOnDeletion is set to true, these Applications will not be deleted.
	PreserveResourcesOnDeletion bool `json:"preserveResourcesOnDeletion,omitempty" protobuf:"bytes,1,name=syncPolicy"`
}

// ApplicationSetTemplate represents argocd ApplicationSpec
type ApplicationSetTemplate struct {
	ApplicationSetTemplateMeta `json:"metadata" protobuf:"bytes,1,name=metadata"`
	Spec                       ApplicationSpec `json:"spec" protobuf:"bytes,2,name=spec"`
}

// ApplicationSetTemplateMeta represents the Argo CD application fields that may
// be used for Applications generated from the ApplicationSet (based on metav1.ObjectMeta)
type ApplicationSetTemplateMeta struct {
	Name        string            `json:"name,omitempty" protobuf:"bytes,1,name=name"`
	Namespace   string            `json:"namespace,omitempty" protobuf:"bytes,2,name=namespace"`
	Labels      map[string]string `json:"labels,omitempty" protobuf:"bytes,3,name=labels"`
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,4,name=annotations"`
	Finalizers  []string          `json:"finalizers,omitempty" protobuf:"bytes,5,name=finalizers"`
}

// ApplicationSetGenerator represents a generator at the top level of an ApplicationSet.
type ApplicationSetGenerator struct {
	List                    *ListGenerator        `json:"list,omitempty" protobuf:"bytes,1,name=list"`
	Clusters                *ClusterGenerator     `json:"clusters,omitempty" protobuf:"bytes,2,name=clusters"`
	Git                     *GitGenerator         `json:"git,omitempty" protobuf:"bytes,3,name=git"`
	SCMProvider             *SCMProviderGenerator `json:"scmProvider,omitempty" protobuf:"bytes,4,name=scmProvider"`
	ClusterDecisionResource *DuckTypeGenerator    `json:"clusterDecisionResource,omitempty" protobuf:"bytes,5,name=clusterDecisionResource"`
	PullRequest             *PullRequestGenerator `json:"pullRequest,omitempty" protobuf:"bytes,6,name=pullRequest"`
	Matrix                  *MatrixGenerator      `json:"matrix,omitempty" protobuf:"bytes,7,name=matrix"`
	Merge                   *MergeGenerator       `json:"merge,omitempty" protobuf:"bytes,8,name=merge"`

	// Selector allows to post-filter all generator.
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,9,name=selector"`
}

// ApplicationSetNestedGenerator represents a generator nested within a combination-type generator (MatrixGenerator or
// MergeGenerator).
type ApplicationSetNestedGenerator struct {
	List                    *ListGenerator        `json:"list,omitempty" protobuf:"bytes,1,name=list"`
	Clusters                *ClusterGenerator     `json:"clusters,omitempty" protobuf:"bytes,2,name=clusters"`
	Git                     *GitGenerator         `json:"git,omitempty" protobuf:"bytes,3,name=git"`
	SCMProvider             *SCMProviderGenerator `json:"scmProvider,omitempty" protobuf:"bytes,4,name=scmProvider"`
	ClusterDecisionResource *DuckTypeGenerator    `json:"clusterDecisionResource,omitempty" protobuf:"bytes,5,name=clusterDecisionResource"`
	PullRequest             *PullRequestGenerator `json:"pullRequest,omitempty" protobuf:"bytes,6,name=pullRequest"`

	// Matrix should have the form of NestedMatrixGenerator
	Matrix *apiextensionsv1.JSON `json:"matrix,omitempty" protobuf:"bytes,7,name=matrix"`

	// Merge should have the form of NestedMergeGenerator
	Merge *apiextensionsv1.JSON `json:"merge,omitempty" protobuf:"bytes,8,name=merge"`

	// Selector allows to post-filter all generator.
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,9,name=selector"`
}

type ApplicationSetNestedGenerators []ApplicationSetNestedGenerator

// ApplicationSetTerminalGenerator represents a generator nested within a nested generator (for example, a list within
// a merge within a matrix). A generator at this level may not be a combination-type generator (MatrixGenerator or
// MergeGenerator). ApplicationSet enforces this nesting depth limit because CRDs do not support recursive types.
// https://github.com/kubernetes-sigs/controller-tools/issues/477
type ApplicationSetTerminalGenerator struct {
	List                    *ListGenerator        `json:"list,omitempty" protobuf:"bytes,1,name=list"`
	Clusters                *ClusterGenerator     `json:"clusters,omitempty" protobuf:"bytes,2,name=clusters"`
	Git                     *GitGenerator         `json:"git,omitempty" protobuf:"bytes,3,name=git"`
	SCMProvider             *SCMProviderGenerator `json:"scmProvider,omitempty" protobuf:"bytes,4,name=scmProvider"`
	ClusterDecisionResource *DuckTypeGenerator    `json:"clusterDecisionResource,omitempty" protobuf:"bytes,5,name=clusterDecisionResource"`
	PullRequest             *PullRequestGenerator `json:"pullRequest,omitempty" protobuf:"bytes,6,name=pullRequest"`
}

type ApplicationSetTerminalGenerators []ApplicationSetTerminalGenerator

// toApplicationSetNestedGenerators converts a terminal generator (a generator which cannot be a combination-type
// generator) to a "nested" generator. The conversion is for convenience, allowing generator g to be used where a nested
// generator is expected.
func (g ApplicationSetTerminalGenerators) toApplicationSetNestedGenerators() []ApplicationSetNestedGenerator {
	nestedGenerators := make([]ApplicationSetNestedGenerator, len(g))
	for i, terminalGenerator := range g {
		nestedGenerators[i] = ApplicationSetNestedGenerator{
			List:                    terminalGenerator.List,
			Clusters:                terminalGenerator.Clusters,
			Git:                     terminalGenerator.Git,
			SCMProvider:             terminalGenerator.SCMProvider,
			ClusterDecisionResource: terminalGenerator.ClusterDecisionResource,
			PullRequest:             terminalGenerator.PullRequest,
		}
	}
	return nestedGenerators
}

// ListGenerator include items info
type ListGenerator struct {
	Elements []apiextensionsv1.JSON `json:"elements" protobuf:"bytes,1,name=elements"`
	Template ApplicationSetTemplate `json:"template,omitempty" protobuf:"bytes,2,name=template"`
}

// MatrixGenerator generates the cartesian product of two sets of parameters. The parameters are defined by two nested
// generators.
type MatrixGenerator struct {
	Generators []ApplicationSetNestedGenerator `json:"generators" protobuf:"bytes,1,name=generators"`
	Template   ApplicationSetTemplate          `json:"template,omitempty" protobuf:"bytes,2,name=template"`
}

// NestedMatrixGenerator is a MatrixGenerator nested under another combination-type generator (MatrixGenerator or
// MergeGenerator). NestedMatrixGenerator does not have an override template, because template overriding has no meaning
// within the constituent generators of combination-type generators.
//
// NOTE: Nested matrix generator is not included directly in the CRD struct, instead it is included
// as a generic 'apiextensionsv1.JSON' object, and then marshalled into a NestedMatrixGenerator
// when processed.
type NestedMatrixGenerator struct {
	Generators ApplicationSetTerminalGenerators `json:"generators" protobuf:"bytes,1,name=generators"`
}

// ToNestedMatrixGenerator converts a JSON struct (from the K8s resource) to corresponding
// NestedMatrixGenerator object.
func ToNestedMatrixGenerator(j *apiextensionsv1.JSON) (*NestedMatrixGenerator, error) {
	if j == nil {
		return nil, nil
	}

	nestedMatrixGenerator := NestedMatrixGenerator{}
	err := json.Unmarshal(j.Raw, &nestedMatrixGenerator)
	if err != nil {
		return nil, err
	}

	return &nestedMatrixGenerator, nil
}

// ToMatrixGenerator converts a NestedMatrixGenerator to a MatrixGenerator. This conversion is for convenience, allowing
// a NestedMatrixGenerator to be used where a MatrixGenerator is expected (of course, the converted generator will have
// no override template).
func (g NestedMatrixGenerator) ToMatrixGenerator() *MatrixGenerator {
	return &MatrixGenerator{
		Generators: g.Generators.toApplicationSetNestedGenerators(),
	}
}

// MergeGenerator merges the output of two or more generators. Where the values for all specified merge keys are equal
// between two sets of generated parameters, the parameter sets will be merged with the parameters from the latter
// generator taking precedence. Parameter sets with merge keys not present in the base generator's params will be
// ignored.
// For example, if the first generator produced [{a: '1', b: '2'}, {c: '1', d: '1'}] and the second generator produced
// [{'a': 'override'}], the united parameters for merge keys = ['a'] would be
// [{a: 'override', b: '1'}, {c: '1', d: '1'}].
//
// MergeGenerator supports template overriding. If a MergeGenerator is one of multiple top-level generators, its
// template will be merged with the top-level generator before the parameters are applied.
type MergeGenerator struct {
	Generators []ApplicationSetNestedGenerator `json:"generators" protobuf:"bytes,1,name=generators"`
	MergeKeys  []string                        `json:"mergeKeys" protobuf:"bytes,2,name=mergeKeys"`
	Template   ApplicationSetTemplate          `json:"template,omitempty" protobuf:"bytes,3,name=template"`
}

// NestedMergeGenerator is a MergeGenerator nested under another combination-type generator (MatrixGenerator or
// MergeGenerator). NestedMergeGenerator does not have an override template, because template overriding has no meaning
// within the constituent generators of combination-type generators.
//
// NOTE: Nested merge generator is not included directly in the CRD struct, instead it is included
// as a generic 'apiextensionsv1.JSON' object, and then marshalled into a NestedMergeGenerator
// when processed.
type NestedMergeGenerator struct {
	Generators ApplicationSetTerminalGenerators `json:"generators" protobuf:"bytes,1,name=generators"`
	MergeKeys  []string                         `json:"mergeKeys" protobuf:"bytes,2,name=mergeKeys"`
}

// ToNestedMergeGenerator converts a JSON struct (from the K8s resource) to corresponding
// NestedMergeGenerator object.
func ToNestedMergeGenerator(j *apiextensionsv1.JSON) (*NestedMergeGenerator, error) {
	if j == nil {
		return nil, nil
	}

	nestedMergeGenerator := NestedMergeGenerator{}
	err := json.Unmarshal(j.Raw, &nestedMergeGenerator)
	if err != nil {
		return nil, err
	}

	return &nestedMergeGenerator, nil
}

// ToMergeGenerator converts a NestedMergeGenerator to a MergeGenerator. This conversion is for convenience, allowing
// a NestedMergeGenerator to be used where a MergeGenerator is expected (of course, the converted generator will have
// no override template).
func (g NestedMergeGenerator) ToMergeGenerator() *MergeGenerator {
	return &MergeGenerator{
		Generators: g.Generators.toApplicationSetNestedGenerators(),
		MergeKeys:  g.MergeKeys,
	}
}

// ClusterGenerator defines a generator to match against clusters registered with ArgoCD.
type ClusterGenerator struct {
	// Selector defines a label selector to match against all clusters registered with ArgoCD.
	// Clusters today are stored as Kubernetes Secrets, thus the Secret labels will be used
	// for matching the selector.
	Selector metav1.LabelSelector   `json:"selector,omitempty" protobuf:"bytes,1,name=selector"`
	Template ApplicationSetTemplate `json:"template,omitempty" protobuf:"bytes,2,name=template"`

	// Values contains key/value pairs which are passed directly as parameters to the template
	Values map[string]string `json:"values,omitempty" protobuf:"bytes,3,name=values"`
}

// DuckType defines a generator to match against clusters registered with ArgoCD.
type DuckTypeGenerator struct {
	// ConfigMapRef is a ConfigMap with the duck type definitions needed to retrieve the data
	//              this includes apiVersion(group/version), kind, matchKey and validation settings
	// Name is the resource name of the kind, group and version, defined in the ConfigMapRef
	// RequeueAfterSeconds is how long before the duckType will be rechecked for a change
	ConfigMapRef        string               `json:"configMapRef" protobuf:"bytes,1,name=configMapRef"`
	Name                string               `json:"name,omitempty" protobuf:"bytes,2,name=name"`
	RequeueAfterSeconds *int64               `json:"requeueAfterSeconds,omitempty" protobuf:"bytes,3,name=requeueAfterSeconds"`
	LabelSelector       metav1.LabelSelector `json:"labelSelector,omitempty" protobuf:"bytes,4,name=labelSelector"`

	Template ApplicationSetTemplate `json:"template,omitempty" protobuf:"bytes,5,name=template"`
	// Values contains key/value pairs which are passed directly as parameters to the template
	Values map[string]string `json:"values,omitempty" protobuf:"bytes,6,name=values"`
}

type GitGenerator struct {
	RepoURL             string                      `json:"repoURL" protobuf:"bytes,1,name=repoURL"`
	Directories         []GitDirectoryGeneratorItem `json:"directories,omitempty" protobuf:"bytes,2,name=directories"`
	Files               []GitFileGeneratorItem      `json:"files,omitempty" protobuf:"bytes,3,name=files"`
	Revision            string                      `json:"revision" protobuf:"bytes,4,name=revision"`
	RequeueAfterSeconds *int64                      `json:"requeueAfterSeconds,omitempty" protobuf:"bytes,5,name=requeueAfterSeconds"`
	Template            ApplicationSetTemplate      `json:"template,omitempty" protobuf:"bytes,6,name=template"`
}

type GitDirectoryGeneratorItem struct {
	Path    string `json:"path" protobuf:"bytes,1,name=path"`
	Exclude bool   `json:"exclude,omitempty" protobuf:"bytes,2,name=exclude"`
}

type GitFileGeneratorItem struct {
	Path string `json:"path" protobuf:"bytes,1,name=path"`
}

// SCMProviderGenerator defines a generator that scrapes a SCMaaS API to find candidate repos.
type SCMProviderGenerator struct {
	// Which provider to use and config for it.
	Github          *SCMProviderGeneratorGithub          `json:"github,omitempty" protobuf:"bytes,1,opt,name=github"`
	Gitlab          *SCMProviderGeneratorGitlab          `json:"gitlab,omitempty" protobuf:"bytes,2,opt,name=gitlab"`
	Bitbucket       *SCMProviderGeneratorBitbucket       `json:"bitbucket,omitempty" protobuf:"bytes,3,opt,name=bitbucket"`
	BitbucketServer *SCMProviderGeneratorBitbucketServer `json:"bitbucketServer,omitempty" protobuf:"bytes,4,opt,name=bitbucketServer"`
	Gitea           *SCMProviderGeneratorGitea           `json:"gitea,omitempty" protobuf:"bytes,5,opt,name=gitea"`
	AzureDevOps     *SCMProviderGeneratorAzureDevOps     `json:"azureDevOps,omitempty" protobuf:"bytes,6,opt,name=azureDevOps"`
	// Filters for which repos should be considered.
	Filters []SCMProviderGeneratorFilter `json:"filters,omitempty" protobuf:"bytes,7,rep,name=filters"`
	// Which protocol to use for the SCM URL. Default is provider-specific but ssh if possible. Not all providers
	// necessarily support all protocols.
	CloneProtocol string `json:"cloneProtocol,omitempty" protobuf:"bytes,8,opt,name=cloneProtocol"`
	// Standard parameters.
	RequeueAfterSeconds *int64                 `json:"requeueAfterSeconds,omitempty" protobuf:"varint,9,opt,name=requeueAfterSeconds"`
	Template            ApplicationSetTemplate `json:"template,omitempty" protobuf:"bytes,10,opt,name=template"`
}

// SCMProviderGeneratorGitea defines a connection info specific to Gitea.
type SCMProviderGeneratorGitea struct {
	// Gitea organization or user to scan. Required.
	Owner string `json:"owner" protobuf:"bytes,1,opt,name=owner"`
	// The Gitea URL to talk to. For example https://gitea.mydomain.com/.
	API string `json:"api" protobuf:"bytes,2,opt,name=api"`
	// Authentication token reference.
	TokenRef *SecretRef `json:"tokenRef,omitempty" protobuf:"bytes,3,opt,name=tokenRef"`
	// Scan all branches instead of just the default branch.
	AllBranches bool `json:"allBranches,omitempty" protobuf:"varint,4,opt,name=allBranches"`
	// Allow self-signed TLS / Certificates; default: false
	Insecure bool `json:"insecure,omitempty" protobuf:"varint,5,opt,name=insecure"`
}

// SCMProviderGeneratorGithub defines connection info specific to GitHub.
type SCMProviderGeneratorGithub struct {
	// GitHub org to scan. Required.
	Organization string `json:"organization" protobuf:"bytes,1,opt,name=organization"`
	// The GitHub API URL to talk to. If blank, use https://api.github.com/.
	API string `json:"api,omitempty" protobuf:"bytes,2,opt,name=api"`
	// Authentication token reference.
	TokenRef *SecretRef `json:"tokenRef,omitempty" protobuf:"bytes,3,opt,name=tokenRef"`
	// AppSecretName is a reference to a GitHub App repo-creds secret.
	AppSecretName string `json:"appSecretName,omitempty" protobuf:"bytes,4,opt,name=appSecretName"`
	// Scan all branches instead of just the default branch.
	AllBranches bool `json:"allBranches,omitempty" protobuf:"varint,5,opt,name=allBranches"`
}

// SCMProviderGeneratorGitlab defines connection info specific to Gitlab.
type SCMProviderGeneratorGitlab struct {
	// Gitlab group to scan. Required.  You can use either the project id (recommended) or the full namespaced path.
	Group string `json:"group" protobuf:"bytes,1,opt,name=group"`
	// Recurse through subgroups (true) or scan only the base group (false).  Defaults to "false"
	IncludeSubgroups bool `json:"includeSubgroups,omitempty" protobuf:"varint,2,opt,name=includeSubgroups"`
	// The Gitlab API URL to talk to.
	API string `json:"api,omitempty" protobuf:"bytes,3,opt,name=api"`
	// Authentication token reference.
	TokenRef *SecretRef `json:"tokenRef,omitempty" protobuf:"bytes,4,opt,name=tokenRef"`
	// Scan all branches instead of just the default branch.
	AllBranches bool `json:"allBranches,omitempty" protobuf:"varint,5,opt,name=allBranches"`
}

// SCMProviderGeneratorBitbucket defines connection info specific to Bitbucket Cloud (API version 2).
type SCMProviderGeneratorBitbucket struct {
	// Bitbucket workspace to scan. Required.
	Owner string `json:"owner" protobuf:"bytes,1,opt,name=owner"`
	// Bitbucket user to use when authenticating.  Should have a "member" role to be able to read all repositories and branches.  Required
	User string `json:"user" protobuf:"bytes,2,opt,name=user"`
	// The app password to use for the user.  Required. See: https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/
	AppPasswordRef *SecretRef `json:"appPasswordRef" protobuf:"bytes,3,opt,name=appPasswordRef"`
	// Scan all branches instead of just the main branch.
	AllBranches bool `json:"allBranches,omitempty" protobuf:"varint,4,opt,name=allBranches"`
}

// SCMProviderGeneratorBitbucketServer defines connection info specific to Bitbucket Server.
type SCMProviderGeneratorBitbucketServer struct {
	// Project to scan. Required.
	Project string `json:"project" protobuf:"bytes,1,opt,name=project"`
	// The Bitbucket Server REST API URL to talk to. Required.
	API string `json:"api" protobuf:"bytes,2,opt,name=api"`
	// Credentials for Basic auth
	BasicAuth *BasicAuthBitbucketServer `json:"basicAuth,omitempty" protobuf:"bytes,3,opt,name=basicAuth"`
	// Scan all branches instead of just the default branch.
	AllBranches bool `json:"allBranches,omitempty" protobuf:"varint,4,opt,name=allBranches"`
}

// SCMProviderGeneratorAzureDevOps defines connection info specific to Azure DevOps.
type SCMProviderGeneratorAzureDevOps struct {
	// Azure Devops organization. Required. E.g. "my-organization".
	Organization string `json:"organization" protobuf:"bytes,5,opt,name=organization"`
	// The URL to Azure DevOps. If blank, use https://dev.azure.com.
	API string `json:"api,omitempty" protobuf:"bytes,6,opt,name=api"`
	// Azure Devops team project. Required. E.g. "my-team".
	TeamProject string `json:"teamProject" protobuf:"bytes,7,opt,name=teamProject"`
	// The Personal Access Token (PAT) to use when connecting. Required.
	AccessTokenRef *SecretRef `json:"accessTokenRef" protobuf:"bytes,8,opt,name=accessTokenRef"`
	// Scan all branches instead of just the default branch.
	AllBranches bool `json:"allBranches,omitempty" protobuf:"varint,9,opt,name=allBranches"`
}

// SCMProviderGeneratorFilter is a single repository filter.
// If multiple filter types are set on a single struct, they will be AND'd together. All filters must
// pass for a repo to be included.
type SCMProviderGeneratorFilter struct {
	// A regex for repo names.
	RepositoryMatch *string `json:"repositoryMatch,omitempty" protobuf:"bytes,1,opt,name=repositoryMatch"`
	// An array of paths, all of which must exist.
	PathsExist []string `json:"pathsExist,omitempty" protobuf:"bytes,2,rep,name=pathsExist"`
	// An array of paths, all of which must not exist.
	PathsDoNotExist []string `json:"pathsDoNotExist,omitempty" protobuf:"bytes,3,rep,name=pathsDoNotExist"`
	// A regex which must match at least one label.
	LabelMatch *string `json:"labelMatch,omitempty" protobuf:"bytes,4,opt,name=labelMatch"`
	// A regex which must match the branch name.
	BranchMatch *string `json:"branchMatch,omitempty" protobuf:"bytes,5,opt,name=branchMatch"`
}

// PullRequestGenerator defines a generator that scrapes a PullRequest API to find candidate pull requests.
type PullRequestGenerator struct {
	// Which provider to use and config for it.
	Github          *PullRequestGeneratorGithub          `json:"github,omitempty" protobuf:"bytes,1,opt,name=github"`
	GitLab          *PullRequestGeneratorGitLab          `json:"gitlab,omitempty" protobuf:"bytes,2,opt,name=gitlab"`
	Gitea           *PullRequestGeneratorGitea           `json:"gitea,omitempty" protobuf:"bytes,3,opt,name=gitea"`
	BitbucketServer *PullRequestGeneratorBitbucketServer `json:"bitbucketServer,omitempty" protobuf:"bytes,4,opt,name=bitbucketServer"`
	// Filters for which pull requests should be considered.
	Filters []PullRequestGeneratorFilter `json:"filters,omitempty" protobuf:"bytes,5,rep,name=filters"`
	// Standard parameters.
	RequeueAfterSeconds *int64                 `json:"requeueAfterSeconds,omitempty" protobuf:"varint,6,opt,name=requeueAfterSeconds"`
	Template            ApplicationSetTemplate `json:"template,omitempty" protobuf:"bytes,7,opt,name=template"`
}

// PullRequestGenerator defines connection info specific to Gitea.
type PullRequestGeneratorGitea struct {
	// Gitea org or user to scan. Required.
	Owner string `json:"owner" protobuf:"bytes,1,opt,name=owner"`
	// Gitea repo name to scan. Required.
	Repo string `json:"repo" protobuf:"bytes,2,opt,name=repo"`
	// The Gitea API URL to talk to. Required
	API string `json:"api" protobuf:"bytes,3,opt,name=api"`
	// Authentication token reference.
	TokenRef *SecretRef `json:"tokenRef,omitempty" protobuf:"bytes,4,opt,name=tokenRef"`
	// Allow insecure tls, for self-signed certificates; default: false.
	Insecure bool `json:"insecure,omitempty" protobuf:"varint,5,opt,name=insecure"`
}

// PullRequestGenerator defines connection info specific to GitHub.
type PullRequestGeneratorGithub struct {
	// GitHub org or user to scan. Required.
	Owner string `json:"owner" protobuf:"bytes,1,opt,name=owner"`
	// GitHub repo name to scan. Required.
	Repo string `json:"repo" protobuf:"bytes,2,opt,name=repo"`
	// The GitHub API URL to talk to. If blank, use https://api.github.com/.
	API string `json:"api,omitempty" protobuf:"bytes,3,opt,name=api"`
	// Authentication token reference.
	TokenRef *SecretRef `json:"tokenRef,omitempty" protobuf:"bytes,4,opt,name=tokenRef"`
	// AppSecretName is a reference to a GitHub App repo-creds secret with permission to access pull requests.
	AppSecretName string `json:"appSecretName,omitempty" protobuf:"bytes,5,opt,name=appSecretName"`
	// Labels is used to filter the PRs that you want to target
	Labels []string `json:"labels,omitempty" protobuf:"bytes,6,rep,name=labels"`
}

// PullRequestGeneratorGitLab defines connection info specific to GitLab.
type PullRequestGeneratorGitLab struct {
	// GitLab project to scan. Required.
	Project string `json:"project" protobuf:"bytes,1,opt,name=project"`
	// The GitLab API URL to talk to. If blank, uses https://gitlab.com/.
	API string `json:"api,omitempty" protobuf:"bytes,2,opt,name=api"`
	// Authentication token reference.
	TokenRef *SecretRef `json:"tokenRef,omitempty" protobuf:"bytes,3,opt,name=tokenRef"`
	// Labels is used to filter the MRs that you want to target
	Labels []string `json:"labels,omitempty" protobuf:"bytes,4,rep,name=labels"`
	// PullRequestState is an additional MRs filter to get only those with a certain state. Default: "" (all states)
	PullRequestState string `json:"pullRequestState,omitempty" protobuf:"bytes,5,rep,name=pullRequestState"`
}

// PullRequestGenerator defines connection info specific to BitbucketServer.
type PullRequestGeneratorBitbucketServer struct {
	// Project to scan. Required.
	Project string `json:"project" protobuf:"bytes,1,opt,name=project"`
	// Repo name to scan. Required.
	Repo string `json:"repo" protobuf:"bytes,2,opt,name=repo"`
	// The Bitbucket REST API URL to talk to e.g. https://bitbucket.org/rest Required.
	API string `json:"api" protobuf:"bytes,3,opt,name=api"`
	// Credentials for Basic auth
	BasicAuth *BasicAuthBitbucketServer `json:"basicAuth,omitempty" protobuf:"bytes,4,opt,name=basicAuth"`
}

// BasicAuthBitbucketServer defines the username/(password or personal access token) for Basic auth.
type BasicAuthBitbucketServer struct {
	// Username for Basic auth
	Username string `json:"username" protobuf:"bytes,1,opt,name=username"`
	// Password (or personal access token) reference.
	PasswordRef *SecretRef `json:"passwordRef" protobuf:"bytes,2,opt,name=passwordRef"`
}

// PullRequestGeneratorFilter is a single pull request filter.
// If multiple filter types are set on a single struct, they will be AND'd together. All filters must
// pass for a pull request to be included.
type PullRequestGeneratorFilter struct {
	BranchMatch *string `json:"branchMatch,omitempty" protobuf:"bytes,1,opt,name=branchMatch"`
}

// ApplicationSetStatus defines the observed state of ApplicationSet
type ApplicationSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []ApplicationSetCondition `json:"conditions,omitempty" protobuf:"bytes,1,name=conditions"`
}

// ApplicationSetCondition contains details about an applicationset condition, which is usally an error or warning
type ApplicationSetCondition struct {
	// Type is an applicationset condition type
	Type ApplicationSetConditionType `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Message contains human-readable message indicating details about condition
	Message string `json:"message" protobuf:"bytes,2,opt,name=message"`
	// LastTransitionTime is the time the condition was last observed
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	// True/False/Unknown
	Status ApplicationSetConditionStatus `json:"status" protobuf:"bytes,4,opt,name=status"`
	//Single word camelcase representing the reason for the status eg ErrorOccurred
	Reason string `json:"reason" protobuf:"bytes,5,opt,name=reason"`
}

// SyncStatusCode is a type which represents possible comparison results
type ApplicationSetConditionStatus string

// Application Condition Status
const (
	// ApplicationSetConditionStatusTrue indicates that a application has been successfully established
	ApplicationSetConditionStatusTrue ApplicationSetConditionStatus = "True"
	// ApplicationSetConditionStatusFalse indicates that a application attempt has failed
	ApplicationSetConditionStatusFalse ApplicationSetConditionStatus = "False"
	// ApplicationSetConditionStatusUnknown indicates that the application condition status could not be reliably determined
	ApplicationSetConditionStatusUnknown ApplicationSetConditionStatus = "Unknown"
)

// ApplicationSetConditionType represents type of application condition. Type name has following convention:
// prefix "Error" means error condition
// prefix "Warning" means warning condition
// prefix "Info" means informational condition
type ApplicationSetConditionType string

//ErrorOccurred / ParametersGenerated / TemplateRendered / ResourcesUpToDate
const (
	ApplicationSetConditionErrorOccurred       ApplicationSetConditionType = "ErrorOccurred"
	ApplicationSetConditionParametersGenerated ApplicationSetConditionType = "ParametersGenerated"
	ApplicationSetConditionResourcesUpToDate   ApplicationSetConditionType = "ResourcesUpToDate"
)

type ApplicationSetReasonType string

const (
	ApplicationSetReasonErrorOccurred                    = "ErrorOccurred"
	ApplicationSetReasonApplicationSetUpToDate           = "ApplicationSetUpToDate"
	ApplicationSetReasonParametersGenerated              = "ParametersGenerated"
	ApplicationSetReasonApplicationGenerated             = "ApplicationGeneratedSuccessfully"
	ApplicationSetReasonUpdateApplicationError           = "UpdateApplicationError"
	ApplicationSetReasonApplicationParamsGenerationError = "ApplicationGenerationFromParamsError"
	ApplicationSetReasonRenderTemplateParamsError        = "RenderTemplateParamsError"
	ApplicationSetReasonCreateApplicationError           = "CreateApplicationError"
	ApplicationSetReasonDeleteApplicationError           = "DeleteApplicationError"
	ApplicationSetReasonRefreshApplicationError          = "RefreshApplicationError"
	ApplicationSetReasonApplicationValidationError       = "ApplicationValidationError"
)

// ApplicationSetList contains a list of ApplicationSet
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ApplicationSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []ApplicationSet `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// func init() {
// 	SchemeBuilder.Register(&ApplicationSet{}, &ApplicationSetList{})
// }

// RefreshRequired checks if the ApplicationSet needs to be refreshed
func (a *ApplicationSet) RefreshRequired() bool {
	_, found := a.Annotations[common.AnnotationApplicationSetRefresh]
	return found
}

// SetConditions updates the applicationset status conditions for a subset of evaluated types.
// If the applicationset has a pre-existing condition of a type that is not in the evaluated list,
// it will be preserved. If the applicationset has a pre-existing condition of a type, status, reason that
// is in the evaluated list, but not in the incoming conditions list, it will be removed.
func (status *ApplicationSetStatus) SetConditions(conditions []ApplicationSetCondition, evaluatedTypes map[ApplicationSetConditionType]bool) {
	applicationSetConditions := make([]ApplicationSetCondition, 0)
	now := metav1.Now()
	for i := range conditions {
		condition := conditions[i]
		if condition.LastTransitionTime == nil {
			condition.LastTransitionTime = &now
		}
		eci := findConditionIndex(status.Conditions, condition.Type)
		if eci >= 0 && status.Conditions[eci].Message == condition.Message && status.Conditions[eci].Reason == condition.Reason && status.Conditions[eci].Status == condition.Status {
			// If we already have a condition of this type, status and reason, only update the timestamp if something
			// has changed.
			applicationSetConditions = append(applicationSetConditions, status.Conditions[eci])
		} else {
			// Otherwise we use the new incoming condition with an updated timestamp:
			applicationSetConditions = append(applicationSetConditions, condition)
		}
	}
	sort.Slice(applicationSetConditions, func(i, j int) bool {
		left := applicationSetConditions[i]
		right := applicationSetConditions[j]
		return fmt.Sprintf("%s/%s/%s/%s/%v", left.Type, left.Message, left.Status, left.Reason, left.LastTransitionTime) < fmt.Sprintf("%s/%s/%s/%s/%v", right.Type, right.Message, right.Status, right.Reason, right.LastTransitionTime)
	})
	status.Conditions = applicationSetConditions
}

func findConditionIndex(conditions []ApplicationSetCondition, t ApplicationSetConditionType) int {
	for i := range conditions {
		if conditions[i].Type == t {
			return i
		}
	}
	return -1
}

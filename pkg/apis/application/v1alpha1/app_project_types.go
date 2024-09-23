package v1alpha1

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/glob"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ErrApplicationNotAllowedToUseProject struct {
	application string
	namespace   string
	project     string
}

func NewErrApplicationNotAllowedToUseProject(application, namespace, project string) error {
	return &ErrApplicationNotAllowedToUseProject{
		application: application,
		namespace:   namespace,
		project:     project,
	}
}

func (err *ErrApplicationNotAllowedToUseProject) Error() string {
	return fmt.Sprintf("application '%s' in namespace '%s' is not allowed to use project %s", err.application, err.namespace, err.project)
}

// AppProjectList is list of AppProject resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []AppProject `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// AppProject provides a logical grouping of applications, providing controls for:
// * where the apps may deploy to (cluster whitelist)
// * what may be deployed (repository whitelist, resource whitelist/blacklist)
// * who can access these applications (roles, OIDC group claims bindings)
// * and what they can do (RBAC policies)
// * automation access to these roles (JWT tokens)
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=appprojects,shortName=appproj;appprojs
type AppProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              AppProjectSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            AppProjectStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// AppProjectStatus contains status information for AppProject CRs
type AppProjectStatus struct {
	// JWTTokensByRole contains a list of JWT tokens issued for a given role
	JWTTokensByRole map[string]JWTTokens `json:"jwtTokensByRole,omitempty" protobuf:"bytes,1,opt,name=jwtTokensByRole"`
}

// GetRoleByName returns the role in a project by the name with its index
func (p *AppProject) GetRoleByName(name string) (*ProjectRole, int, error) {
	for i, role := range p.Spec.Roles {
		if name == role.Name {
			return &role, i, nil
		}
	}
	return nil, -1, fmt.Errorf("role '%s' does not exist in project '%s'", name, p.Name)
}

// GetJWTTokenFromSpec looks up the index of a JWTToken in a project by id (new token), if not then by the issue at time (old token)
func (p *AppProject) GetJWTTokenFromSpec(roleName string, issuedAt int64, id string) (*JWTToken, int, error) {
	// This is for backward compatibility. In the oder version, JWTTokens are stored under spec.role
	role, _, err := p.GetRoleByName(roleName)
	if err != nil {
		return nil, -1, err
	}

	if id != "" {
		for i, token := range role.JWTTokens {
			if id == token.ID {
				return &token, i, nil
			}
		}
	}

	if issuedAt != -1 {
		for i, token := range role.JWTTokens {
			if issuedAt == token.IssuedAt {
				return &token, i, nil
			}
		}
	}

	return nil, -1, fmt.Errorf("JWT token for role '%s' issued at '%d' does not exist in project '%s'", role.Name, issuedAt, p.Name)
}

// GetJWTToken looks up the index of a JWTToken in a project by id (new token), if not then by the issue at time (old token)
func (p *AppProject) GetJWTToken(roleName string, issuedAt int64, id string) (*JWTToken, int, error) {
	// This is for newer version, JWTTokens are stored under status
	if id != "" {
		for i, token := range p.Status.JWTTokensByRole[roleName].Items {
			if id == token.ID {
				return &token, i, nil
			}
		}
	}

	if issuedAt != -1 {
		for i, token := range p.Status.JWTTokensByRole[roleName].Items {
			if issuedAt == token.IssuedAt {
				return &token, i, nil
			}
		}
	}

	return nil, -1, fmt.Errorf("JWT token for role '%s' issued at '%d' does not exist in project '%s'", roleName, issuedAt, p.Name)
}

// RemoveJWTToken removes the specified JWT from an AppProject
func (p AppProject) RemoveJWTToken(roleIndex int, issuedAt int64, id string) error {
	roleName := p.Spec.Roles[roleIndex].Name
	// For backward compatibility
	_, jwtTokenIndex, err1 := p.GetJWTTokenFromSpec(roleName, issuedAt, id)
	if err1 == nil {
		p.Spec.Roles[roleIndex].JWTTokens[jwtTokenIndex] = p.Spec.Roles[roleIndex].JWTTokens[len(p.Spec.Roles[roleIndex].JWTTokens)-1]
		p.Spec.Roles[roleIndex].JWTTokens = p.Spec.Roles[roleIndex].JWTTokens[:len(p.Spec.Roles[roleIndex].JWTTokens)-1]
	}

	// New location for storing JWTToken
	_, jwtTokenIndex, err2 := p.GetJWTToken(roleName, issuedAt, id)
	if err2 == nil {
		p.Status.JWTTokensByRole[roleName].Items[jwtTokenIndex] = p.Status.JWTTokensByRole[roleName].Items[len(p.Status.JWTTokensByRole[roleName].Items)-1]
		p.Status.JWTTokensByRole[roleName] = JWTTokens{Items: p.Status.JWTTokensByRole[roleName].Items[:len(p.Status.JWTTokensByRole[roleName].Items)-1]}
	}

	if err1 == nil || err2 == nil {
		// If we find this token from either places, we can say there are no error
		return nil
	} else {
		// If we could not locate this taken from either places, we can return any of the errors
		return err2
	}
}

// TODO: document this method
func (p *AppProject) ValidateJWTTokenID(roleName string, id string) error {
	role, _, err := p.GetRoleByName(roleName)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	for _, token := range role.JWTTokens {
		if id == token.ID {
			return status.Errorf(codes.InvalidArgument, "Token id '%s' has been used. ", id)
		}
	}
	return nil
}

func (p *AppProject) ValidateProject() error {
	destKeys := make(map[string]bool)
	for _, dest := range p.Spec.Destinations {
		if dest.Name == "!*" {
			return status.Errorf(codes.InvalidArgument, "name has an invalid format, '!*'")
		}

		if dest.Server == "!*" {
			return status.Errorf(codes.InvalidArgument, "server has an invalid format, '!*'")
		}

		if dest.Namespace == "!*" {
			return status.Errorf(codes.InvalidArgument, "namespace has an invalid format, '!*'")
		}

		key := fmt.Sprintf("%s/%s", dest.Server, dest.Namespace)
		if dest.Server == "" && dest.Name != "" {
			// destination cluster set using name instead of server endpoint
			key = fmt.Sprintf("%s/%s", dest.Name, dest.Namespace)
		}
		if _, ok := destKeys[key]; ok {
			return status.Errorf(codes.InvalidArgument, "destination '%s' already added", key)
		}
		destKeys[key] = true
	}

	srcNamespaces := make(map[string]bool)
	for _, ns := range p.Spec.SourceNamespaces {
		if _, ok := srcNamespaces[ns]; ok {
			return status.Errorf(codes.InvalidArgument, "source namespace '%s' already added", ns)
		}
		srcNamespaces[ns] = true
	}

	srcRepos := make(map[string]bool)
	for _, src := range p.Spec.SourceRepos {
		if src == "!*" {
			return status.Errorf(codes.InvalidArgument, "source repository has an invalid format, '!*'")
		}

		if _, ok := srcRepos[src]; ok {
			return status.Errorf(codes.InvalidArgument, "source repository '%s' already added", src)
		}
		srcRepos[src] = true
	}

	roleNames := make(map[string]bool)
	for _, role := range p.Spec.Roles {
		if _, ok := roleNames[role.Name]; ok {
			return status.Errorf(codes.AlreadyExists, "role '%s' already exists", role.Name)
		}
		if err := validateRoleName(role.Name); err != nil {
			return err
		}
		existingPolicies := make(map[string]bool)
		for _, policy := range role.Policies {
			if _, ok := existingPolicies[policy]; ok {
				return status.Errorf(codes.AlreadyExists, "policy '%s' already exists for role '%s'", policy, role.Name)
			}
			if err := validatePolicy(p.Name, role.Name, policy); err != nil {
				return err
			}
			existingPolicies[policy] = true
		}
		existingGroups := make(map[string]bool)
		for _, group := range role.Groups {
			if _, ok := existingGroups[group]; ok {
				return status.Errorf(codes.AlreadyExists, "group '%s' already exists for role '%s'", group, role.Name)
			}
			if err := validateGroupName(group); err != nil {
				return err
			}
			existingGroups[group] = true
		}
		roleNames[role.Name] = true
	}

	if p.Spec.SyncWindows.HasWindows() {
		existingWindows := make(map[string]bool)
		for _, window := range p.Spec.SyncWindows {
			if window == nil {
				continue
			}
			if _, ok := existingWindows[window.Kind+window.Schedule+window.Duration]; ok {
				return status.Errorf(codes.AlreadyExists, "window '%s':'%s':'%s' already exists, update or edit", window.Kind, window.Schedule, window.Duration)
			}
			err := window.Validate()
			if err != nil {
				return err
			}
			if len(window.Applications) == 0 && len(window.Namespaces) == 0 && len(window.Clusters) == 0 {
				return status.Errorf(codes.OutOfRange, "window '%s':'%s':'%s' requires one of application, cluster or namespace", window.Kind, window.Schedule, window.Duration)
			}
			existingWindows[window.Kind+window.Schedule+window.Duration] = true
		}
	}

	destServiceAccts := make(map[string]bool)
	for _, destServiceAcct := range p.Spec.DestinationServiceAccounts {
		if destServiceAcct.Server == "!*" {
			return status.Errorf(codes.InvalidArgument, "server has an invalid format, '!*'")
		}

		if destServiceAcct.Namespace == "!*" {
			return status.Errorf(codes.InvalidArgument, "namespace has an invalid format, '!*'")
		}

		key := fmt.Sprintf("%s/%s", destServiceAcct.Server, destServiceAcct.Namespace)
		if _, ok := destServiceAccts[key]; ok {
			return status.Errorf(codes.InvalidArgument, "destinationServiceAccount '%s' already added", key)
		}
		destServiceAccts[key] = true
	}

	return nil
}

// AddGroupToRole adds an OIDC group to a role
func (p *AppProject) AddGroupToRole(roleName, group string) (bool, error) {
	role, roleIndex, err := p.GetRoleByName(roleName)
	if err != nil {
		return false, err
	}
	for _, roleGroup := range role.Groups {
		if group == roleGroup {
			return false, nil
		}
	}
	role.Groups = append(role.Groups, group)
	p.Spec.Roles[roleIndex] = *role
	return true, nil
}

// RemoveGroupFromRole removes an OIDC group from a role
func (p *AppProject) RemoveGroupFromRole(roleName, group string) (bool, error) {
	role, roleIndex, err := p.GetRoleByName(roleName)
	if err != nil {
		return false, err
	}
	for i, roleGroup := range role.Groups {
		if group == roleGroup {
			role.Groups = append(role.Groups[:i], role.Groups[i+1:]...)
			p.Spec.Roles[roleIndex] = *role
			return true, nil
		}
	}
	return false, nil
}

// NormalizePolicies normalizes the policies in the project
func (p *AppProject) NormalizePolicies() {
	for i, role := range p.Spec.Roles {
		var normalizedPolicies []string
		for _, policy := range role.Policies {
			normalizedPolicies = append(normalizedPolicies, p.normalizePolicy(policy))
		}
		p.Spec.Roles[i].Policies = normalizedPolicies
	}
}

func (p *AppProject) normalizePolicy(policy string) string {
	policyComponents := strings.Split(policy, ",")
	normalizedPolicy := ""
	for _, component := range policyComponents {
		if normalizedPolicy == "" {
			normalizedPolicy = component
		} else {
			normalizedPolicy = fmt.Sprintf("%s, %s", normalizedPolicy, strings.Trim(component, " "))
		}
	}
	return normalizedPolicy
}

// ProjectPoliciesString returns a Casbin formatted string of a project's policies for each role
func (proj *AppProject) ProjectPoliciesString() string {
	var policies []string
	for _, role := range proj.Spec.Roles {
		projectPolicy := fmt.Sprintf("p, proj:%s:%s, projects, get, %s, allow", proj.ObjectMeta.Name, role.Name, proj.ObjectMeta.Name)
		policies = append(policies, projectPolicy)
		policies = append(policies, role.Policies...)
		for _, groupName := range role.Groups {
			policies = append(policies, fmt.Sprintf("g, %s, proj:%s:%s", groupName, proj.ObjectMeta.Name, role.Name))
		}
	}
	return strings.Join(policies, "\n")
}

// IsGroupKindPermitted validates if the given resource group/kind is permitted to be deployed in the project
func (proj AppProject) IsGroupKindPermitted(gk schema.GroupKind, namespaced bool) bool {
	var isWhiteListed, isBlackListed bool
	res := metav1.GroupKind{Group: gk.Group, Kind: gk.Kind}

	if namespaced {
		namespaceWhitelist := proj.Spec.NamespaceResourceWhitelist
		namespaceBlacklist := proj.Spec.NamespaceResourceBlacklist

		isWhiteListed = namespaceWhitelist == nil || len(namespaceWhitelist) != 0 && isResourceInList(res, namespaceWhitelist)
		isBlackListed = len(namespaceBlacklist) != 0 && isResourceInList(res, namespaceBlacklist)
		return isWhiteListed && !isBlackListed
	}

	clusterWhitelist := proj.Spec.ClusterResourceWhitelist
	clusterBlacklist := proj.Spec.ClusterResourceBlacklist

	isWhiteListed = len(clusterWhitelist) != 0 && isResourceInList(res, clusterWhitelist)
	isBlackListed = len(clusterBlacklist) != 0 && isResourceInList(res, clusterBlacklist)
	return isWhiteListed && !isBlackListed
}

// IsLiveResourcePermitted returns whether a live resource found in the cluster is permitted by an AppProject
func (proj AppProject) IsLiveResourcePermitted(un *unstructured.Unstructured, server string, name string, projectClusters func(project string) ([]*Cluster, error)) (bool, error) {
	return proj.IsResourcePermitted(un.GroupVersionKind().GroupKind(), un.GetNamespace(), ApplicationDestination{Server: server, Name: name}, projectClusters)
}

func (proj AppProject) IsResourcePermitted(groupKind schema.GroupKind, namespace string, dest ApplicationDestination, projectClusters func(project string) ([]*Cluster, error)) (bool, error) {
	if !proj.IsGroupKindPermitted(groupKind, namespace != "") {
		return false, nil
	}
	if namespace != "" {
		return proj.IsDestinationPermitted(ApplicationDestination{Server: dest.Server, Name: dest.Name, Namespace: namespace}, projectClusters)
	}
	return true, nil
}

// HasFinalizer returns true if a resource finalizer is set on an AppProject
func (proj AppProject) HasFinalizer() bool {
	return getFinalizerIndex(proj.ObjectMeta, ResourcesFinalizerName) > -1
}

// RemoveFinalizer removes a resource finalizer from an AppProject
func (proj *AppProject) RemoveFinalizer() {
	setFinalizer(&proj.ObjectMeta, ResourcesFinalizerName, false)
}

func globMatch(pattern string, val string, allowNegation bool, separators ...rune) bool {
	if allowNegation && isDenyPattern(pattern) {
		return !glob.Match(pattern[1:], val, separators...)
	}

	if pattern == "*" {
		return true
	}
	return glob.Match(pattern, val, separators...)
}

// IsSourcePermitted validates if the provided application's source is a one of the allowed sources for the project.
func (proj AppProject) IsSourcePermitted(src ApplicationSource) bool {
	srcNormalized := git.NormalizeGitURL(src.RepoURL)

	var normalized string
	anySourceMatched := false

	for _, repoURL := range proj.Spec.SourceRepos {
		if isDenyPattern(repoURL) {
			normalized = "!" + git.NormalizeGitURL(strings.TrimPrefix(repoURL, "!"))
		} else {
			normalized = git.NormalizeGitURL(repoURL)
		}

		matched := globMatch(normalized, srcNormalized, true, '/')
		if matched {
			anySourceMatched = true
		} else if !matched && isDenyPattern(normalized) {
			return false
		}
	}

	return anySourceMatched
}

// IsDestinationPermitted validates if the provided application's destination is one of the allowed destinations for the project
func (proj AppProject) IsDestinationPermitted(dst ApplicationDestination, projectClusters func(project string) ([]*Cluster, error)) (bool, error) {
	destinationMatched := proj.isDestinationMatched(dst)
	if destinationMatched && proj.Spec.PermitOnlyProjectScopedClusters {
		clusters, err := projectClusters(proj.Name)
		if err != nil {
			return false, fmt.Errorf("could not retrieve project clusters: %w", err)
		}

		for _, cluster := range clusters {
			if cluster.Name == dst.Name || cluster.Server == dst.Server {
				return true, nil
			}
		}

		return false, nil
	}

	return destinationMatched, nil
}

func (proj AppProject) isDestinationMatched(dst ApplicationDestination) bool {
	anyDestinationMatched := false
	noDenyDestinationsMatched := true

	for _, item := range proj.Spec.Destinations {
		dstNameMatched := dst.Name != "" && globMatch(item.Name, dst.Name, true)
		dstServerMatched := dst.Server != "" && globMatch(item.Server, dst.Server, true)
		dstNamespaceMatched := globMatch(item.Namespace, dst.Namespace, true)

		matched := (dstServerMatched || dstNameMatched) && dstNamespaceMatched
		if matched {
			anyDestinationMatched = true
		} else if ((!dstNameMatched && isDenyPattern(item.Name)) || (!dstServerMatched && isDenyPattern(item.Server))) || (!dstNamespaceMatched && isDenyPattern(item.Namespace)) {
			noDenyDestinationsMatched = false
		}
	}

	return anyDestinationMatched && noDenyDestinationsMatched
}

func isDenyPattern(pattern string) bool {
	return strings.HasPrefix(pattern, "!")
}

// TODO: document this method
func (proj *AppProject) NormalizeJWTTokens() bool {
	needNormalize := false
	for i, role := range proj.Spec.Roles {
		for j, token := range role.JWTTokens {
			if token.ID == "" {
				token.ID = strconv.FormatInt(token.IssuedAt, 10)
				role.JWTTokens[j] = token
				needNormalize = true
			}
		}
		proj.Spec.Roles[i] = role
	}
	for _, roleTokenEntry := range proj.Status.JWTTokensByRole {
		for j, token := range roleTokenEntry.Items {
			if token.ID == "" {
				token.ID = strconv.FormatInt(token.IssuedAt, 10)
				roleTokenEntry.Items[j] = token
				needNormalize = true
			}
		}
	}
	needSync := syncJWTTokenBetweenStatusAndSpec(proj)
	return needNormalize || needSync
}

func syncJWTTokenBetweenStatusAndSpec(proj *AppProject) bool {
	existingRole := map[string]bool{}
	needSync := false
	for roleIndex, role := range proj.Spec.Roles {
		existingRole[role.Name] = true

		tokensInSpec := role.JWTTokens
		tokensInStatus := []JWTToken{}
		if proj.Status.JWTTokensByRole == nil {
			tokensByRole := make(map[string]JWTTokens)
			proj.Status.JWTTokensByRole = tokensByRole
		} else {
			tokensInStatus = proj.Status.JWTTokensByRole[role.Name].Items
		}
		tokens := jwtTokensCombine(tokensInStatus, tokensInSpec)

		sort.Slice(proj.Spec.Roles[roleIndex].JWTTokens, func(i, j int) bool {
			return proj.Spec.Roles[roleIndex].JWTTokens[i].IssuedAt > proj.Spec.Roles[roleIndex].JWTTokens[j].IssuedAt
		})
		sort.Slice(proj.Status.JWTTokensByRole[role.Name].Items, func(i, j int) bool {
			return proj.Status.JWTTokensByRole[role.Name].Items[i].IssuedAt > proj.Status.JWTTokensByRole[role.Name].Items[j].IssuedAt
		})
		if !cmp.Equal(tokens, proj.Spec.Roles[roleIndex].JWTTokens) || !cmp.Equal(tokens, proj.Status.JWTTokensByRole[role.Name].Items) {
			needSync = true
		}

		proj.Spec.Roles[roleIndex].JWTTokens = tokens
		proj.Status.JWTTokensByRole[role.Name] = JWTTokens{Items: tokens}
	}
	if proj.Status.JWTTokensByRole != nil {
		for role := range proj.Status.JWTTokensByRole {
			if !existingRole[role] {
				delete(proj.Status.JWTTokensByRole, role)
				needSync = true
			}
		}
	}

	return needSync
}

func jwtTokensCombine(tokens1 []JWTToken, tokens2 []JWTToken) []JWTToken {
	tokensMap := make(map[string]JWTToken)
	for _, token := range append(tokens1, tokens2...) {
		tokensMap[token.ID] = token
	}

	var tokens []JWTToken
	for _, v := range tokensMap {
		tokens = append(tokens, v)
	}

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].IssuedAt > tokens[j].IssuedAt
	})
	return tokens
}

// IsAppNamespacePermitted checks whether an application that associates with
// this AppProject is allowed by comparing the Application's namespace with
// the list of allowed namespaces in the AppProject.
//
// Applications in the installation namespace are always permitted. Also, at
// application creation time, its namespace may yet be empty to indicate that
// the application will be created in the controller's namespace.
func (p AppProject) IsAppNamespacePermitted(app *Application, controllerNs string) bool {
	if app.Namespace == "" || app.Namespace == controllerNs {
		return true
	}

	return glob.MatchStringInList(p.Spec.SourceNamespaces, app.Namespace, glob.REGEXP)
}

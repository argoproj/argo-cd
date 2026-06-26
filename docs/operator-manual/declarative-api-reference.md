# Declarative API Reference
This page documents the declarative configuration of Argo CD.
> [!WARNING]
> The internal Go struct representations of resources (such as Clusters or Repositories) can differ from their serialized representations in Kubernetes Secrets. Please refer to the specific field descriptions below for details on how each field is mapped, formatted, and stored in Secrets.
## CRD-backed Resources
The following Custom Resource Definitions (CRDs) define Argo CD resources:
<ul>
<li><a href="#argoproj.io/v1alpha1.Application">Application</a></li>
<li><a href="#argoproj.io/v1alpha1.ApplicationSet">ApplicationSet</a></li>
<li><a href="#argoproj.io/v1alpha1.AppProject">AppProject</a></li>
</ul>
## Secret-backed Configuration
The following configurations are stored as Kubernetes Secrets:
<ul>
<li><a href="#argoproj.io/v1alpha1.Cluster">Cluster</a></li>
<li><a href="#argoproj.io/v1alpha1.Repository">Repository</a></li>
<li><a href="#argoproj.io/v1alpha1.RepoCreds">RepoCreds (Repository Credentials)</a></li>
</ul>
<hr/>
<h3 id="argoproj.io/v1alpha1.AWSAuthConfig">AWSAuthConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ClusterConfig">ClusterConfig</a>)
</p>
<p>
<p>AWSAuthConfig is an AWS IAM authentication configuration</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterName</code><br/>
<em>
string
</em>
</td>
<td>
<p>ClusterName contains AWS cluster name</p>
</td>
</tr>
<tr>
<td>
<code>roleARN</code><br/>
<em>
string
</em>
</td>
<td>
<p>RoleARN contains optional role ARN. If set then AWS IAM Authenticator assume a role to perform cluster operations instead of the default AWS credential provider chain.</p>
</td>
</tr>
<tr>
<td>
<code>profile</code><br/>
<em>
string
</em>
</td>
<td>
<p>Profile contains optional role ARN. If set then AWS IAM Authenticator uses the profile to perform cluster operations instead of the default AWS credential provider chain.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.AppHealthStatus">AppHealthStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>AppHealthStatus contains information about the currently observed health state of an application</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>status</code><br/>
<em>
github.com/argoproj/argo-cd/gitops-engine/pkg/health.HealthStatusCode
</em>
</td>
<td>
<p>Status holds the status code of the application</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message is a human-readable informational message describing the health status</p>
<p>Deprecated: this field is not used and will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>LastTransitionTime is the time the HealthStatus was set or updated</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.AppProject">AppProject
</h3>
<p>
<p>AppProject provides a logical grouping of applications, providing controls for:
* where the apps may deploy to (cluster whitelist)
* what may be deployed (repository whitelist, resource whitelist/blacklist)
* who can access these applications (roles, OIDC group claims bindings)
* and what they can do (RBAC policies)
* automation access to these roles (JWT tokens)</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">
AppProjectSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>sourceRepos</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>SourceRepos contains list of repository URLs which can be used for deployment</p>
</td>
</tr>
<tr>
<td>
<code>destinations</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestination">
[]ApplicationDestination
</a>
</em>
</td>
<td>
<p>Destinations contains list of destinations available for deployment</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<p>Description contains optional project description</p>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ProjectRole">
[]ProjectRole
</a>
</em>
</td>
<td>
<p>Roles are user defined RBAC roles associated with this project</p>
</td>
</tr>
<tr>
<td>
<code>clusterResourceWhitelist</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterResourceRestrictionItem">
[]ClusterResourceRestrictionItem
</a>
</em>
</td>
<td>
<p>ClusterResourceWhitelist contains list of whitelisted cluster level resources</p>
</td>
</tr>
<tr>
<td>
<code>namespaceResourceBlacklist</code><br/>
<em>
[]k8s.io/apimachinery/pkg/apis/meta/v1.GroupKind
</em>
</td>
<td>
<p>NamespaceResourceBlacklist contains list of blacklisted namespace level resources</p>
</td>
</tr>
<tr>
<td>
<code>orphanedResources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OrphanedResourcesMonitorSettings">
OrphanedResourcesMonitorSettings
</a>
</em>
</td>
<td>
<p>OrphanedResources specifies if controller should monitor orphaned resources of apps in this project</p>
</td>
</tr>
<tr>
<td>
<code>syncWindows</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncWindows">
SyncWindows
</a>
</em>
</td>
<td>
<p>SyncWindows controls when syncs can be run for apps in this project</p>
</td>
</tr>
<tr>
<td>
<code>namespaceResourceWhitelist</code><br/>
<em>
[]k8s.io/apimachinery/pkg/apis/meta/v1.GroupKind
</em>
</td>
<td>
<p>NamespaceResourceWhitelist contains list of whitelisted namespace level resources</p>
</td>
</tr>
<tr>
<td>
<code>signatureKeys</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SignatureKey">
[]SignatureKey
</a>
</em>
</td>
<td>
<p>SignatureKeys contains a list of PGP key IDs that commits in Git must be signed with in order to be allowed for sync</p>
<p>Deprecated: Use SourceIntegrity instead. SignatureKeys will be removed with the next major version.</p>
</td>
</tr>
<tr>
<td>
<code>clusterResourceBlacklist</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterResourceRestrictionItem">
[]ClusterResourceRestrictionItem
</a>
</em>
</td>
<td>
<p>ClusterResourceBlacklist contains list of blacklisted cluster level resources</p>
</td>
</tr>
<tr>
<td>
<code>sourceNamespaces</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>SourceNamespaces defines the namespaces application resources are allowed to be created in</p>
</td>
</tr>
<tr>
<td>
<code>permitOnlyProjectScopedClusters</code><br/>
<em>
bool
</em>
</td>
<td>
<p>PermitOnlyProjectScopedClusters determines whether destinations can only reference clusters which are project-scoped</p>
</td>
</tr>
<tr>
<td>
<code>destinationServiceAccounts</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestinationServiceAccount">
[]ApplicationDestinationServiceAccount
</a>
</em>
</td>
<td>
<p>DestinationServiceAccounts holds information about the service accounts to be impersonated for the application sync operation for each destination.</p>
</td>
</tr>
<tr>
<td>
<code>sourceIntegrity</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrity">
SourceIntegrity
</a>
</em>
</td>
<td>
<p>SourceIntegrity represents a constraint on manifest sources integrity to be met before they can be used.
Do not access directly, use EffectiveSourceIntegrity() for correct backwards compatibility handling.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.AppProjectStatus">
AppProjectStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProject">AppProject</a>)
</p>
<p>
<p>AppProjectSpec is the specification of an AppProject</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sourceRepos</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>SourceRepos contains list of repository URLs which can be used for deployment</p>
</td>
</tr>
<tr>
<td>
<code>destinations</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestination">
[]ApplicationDestination
</a>
</em>
</td>
<td>
<p>Destinations contains list of destinations available for deployment</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<p>Description contains optional project description</p>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ProjectRole">
[]ProjectRole
</a>
</em>
</td>
<td>
<p>Roles are user defined RBAC roles associated with this project</p>
</td>
</tr>
<tr>
<td>
<code>clusterResourceWhitelist</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterResourceRestrictionItem">
[]ClusterResourceRestrictionItem
</a>
</em>
</td>
<td>
<p>ClusterResourceWhitelist contains list of whitelisted cluster level resources</p>
</td>
</tr>
<tr>
<td>
<code>namespaceResourceBlacklist</code><br/>
<em>
[]k8s.io/apimachinery/pkg/apis/meta/v1.GroupKind
</em>
</td>
<td>
<p>NamespaceResourceBlacklist contains list of blacklisted namespace level resources</p>
</td>
</tr>
<tr>
<td>
<code>orphanedResources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OrphanedResourcesMonitorSettings">
OrphanedResourcesMonitorSettings
</a>
</em>
</td>
<td>
<p>OrphanedResources specifies if controller should monitor orphaned resources of apps in this project</p>
</td>
</tr>
<tr>
<td>
<code>syncWindows</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncWindows">
SyncWindows
</a>
</em>
</td>
<td>
<p>SyncWindows controls when syncs can be run for apps in this project</p>
</td>
</tr>
<tr>
<td>
<code>namespaceResourceWhitelist</code><br/>
<em>
[]k8s.io/apimachinery/pkg/apis/meta/v1.GroupKind
</em>
</td>
<td>
<p>NamespaceResourceWhitelist contains list of whitelisted namespace level resources</p>
</td>
</tr>
<tr>
<td>
<code>signatureKeys</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SignatureKey">
[]SignatureKey
</a>
</em>
</td>
<td>
<p>SignatureKeys contains a list of PGP key IDs that commits in Git must be signed with in order to be allowed for sync</p>
<p>Deprecated: Use SourceIntegrity instead. SignatureKeys will be removed with the next major version.</p>
</td>
</tr>
<tr>
<td>
<code>clusterResourceBlacklist</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterResourceRestrictionItem">
[]ClusterResourceRestrictionItem
</a>
</em>
</td>
<td>
<p>ClusterResourceBlacklist contains list of blacklisted cluster level resources</p>
</td>
</tr>
<tr>
<td>
<code>sourceNamespaces</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>SourceNamespaces defines the namespaces application resources are allowed to be created in</p>
</td>
</tr>
<tr>
<td>
<code>permitOnlyProjectScopedClusters</code><br/>
<em>
bool
</em>
</td>
<td>
<p>PermitOnlyProjectScopedClusters determines whether destinations can only reference clusters which are project-scoped</p>
</td>
</tr>
<tr>
<td>
<code>destinationServiceAccounts</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestinationServiceAccount">
[]ApplicationDestinationServiceAccount
</a>
</em>
</td>
<td>
<p>DestinationServiceAccounts holds information about the service accounts to be impersonated for the application sync operation for each destination.</p>
</td>
</tr>
<tr>
<td>
<code>sourceIntegrity</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrity">
SourceIntegrity
</a>
</em>
</td>
<td>
<p>SourceIntegrity represents a constraint on manifest sources integrity to be met before they can be used.
Do not access directly, use EffectiveSourceIntegrity() for correct backwards compatibility handling.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.AppProjectStatus">AppProjectStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProject">AppProject</a>)
</p>
<p>
<p>AppProjectStatus contains status information for AppProject CRs</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>jwtTokensByRole</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.JWTTokens">
map[string]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.JWTTokens
</a>
</em>
</td>
<td>
<p>JWTTokensByRole contains a list of JWT tokens issued for a given role</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.Application">Application
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationWatchEvent">ApplicationWatchEvent</a>)
</p>
<p>
<p>Application is a definition of Application resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
</em>
</td>
<td>
<p>Common: shared with ApplicationSet</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">
ApplicationSpec
</a>
</em>
</td>
<td>
<p>Common: shared with ApplicationSet</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">
ApplicationSource
</a>
</em>
</td>
<td>
<p>Source is a reference to the location of the application&rsquo;s manifests or chart</p>
</td>
</tr>
<tr>
<td>
<code>destination</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestination">
ApplicationDestination
</a>
</em>
</td>
<td>
<p>Destination is a reference to the target Kubernetes server and namespace</p>
</td>
</tr>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Project is a reference to the project this application belongs to.
The empty string means that application belongs to the &lsquo;default&rsquo; project.</p>
</td>
</tr>
<tr>
<td>
<code>syncPolicy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncPolicy">
SyncPolicy
</a>
</em>
</td>
<td>
<p>SyncPolicy controls when and how a sync will be performed</p>
</td>
</tr>
<tr>
<td>
<code>ignoreDifferences</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.IgnoreDifferences">
IgnoreDifferences
</a>
</em>
</td>
<td>
<p>IgnoreDifferences is a list of resources and their fields which should be ignored during comparison</p>
</td>
</tr>
<tr>
<td>
<code>info</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Info">
[]Info
</a>
</em>
</td>
<td>
<p>Info contains a list of information (URLs, email addresses, and plain text) that relates to the application</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code><br/>
<em>
int64
</em>
</td>
<td>
<p>RevisionHistoryLimit limits the number of items kept in the application&rsquo;s revision history, which is used for informational purposes as well as for rollbacks to previous versions.
This should only be changed in exceptional circumstances.
Setting to zero will store no history. This will reduce storage used.
Increasing will increase the space used to store the history, so we do not recommend increasing it.
Default is 10.</p>
</td>
</tr>
<tr>
<td>
<code>sources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSources">
ApplicationSources
</a>
</em>
</td>
<td>
<p>Sources is a reference to the location of the application&rsquo;s manifests or chart</p>
</td>
</tr>
<tr>
<td>
<code>sourceHydrator</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">
SourceHydrator
</a>
</em>
</td>
<td>
<p>SourceHydrator provides a way to push hydrated manifests back to git before syncing them to the cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">
ApplicationStatus
</a>
</em>
</td>
<td>
<p>Common: shared with ApplicationSet (different type)</p>
</td>
</tr>
<tr>
<td>
<code>operation</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Operation">
Operation
</a>
</em>
</td>
<td>
<p>Common: shared with ApplicationSet (different type)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationCondition">ApplicationCondition
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>ApplicationCondition contains details about an application condition, which is usually an error or warning</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>Type is an application condition type</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message contains human-readable message indicating details about condition</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>LastTransitionTime is the time the condition was last observed</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationDestination">ApplicationDestination
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec</a>, 
<a href="#argoproj.io/v1alpha1.ComparedTo">ComparedTo</a>)
</p>
<p>
<p>ApplicationDestination holds information about the application&rsquo;s destination</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>server</code><br/>
<em>
string
</em>
</td>
<td>
<p>Server specifies the URL of the target cluster&rsquo;s Kubernetes control plane API. This must be set if Name is not set.</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace specifies the target namespace for the application&rsquo;s resources.
The namespace will only be set for namespace-scoped resources that have not set a value for .metadata.namespace</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is an alternate way of specifying the target cluster by its symbolic name. This must be set if Server is not set.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationDestinationServiceAccount">ApplicationDestinationServiceAccount
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>)
</p>
<p>
<p>ApplicationDestinationServiceAccount holds information about the service account to be impersonated for the application sync operation.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>server</code><br/>
<em>
string
</em>
</td>
<td>
<p>Server specifies the URL of the target cluster&rsquo;s Kubernetes control plane API.</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace specifies the target namespace for the application&rsquo;s resources.</p>
</td>
</tr>
<tr>
<td>
<code>defaultServiceAccount</code><br/>
<em>
string
</em>
</td>
<td>
<p>DefaultServiceAccount to be used for impersonation during the sync operation</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationMatchExpression">ApplicationMatchExpression
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetRolloutStep">ApplicationSetRolloutStep</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>operator</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationPreservedFields">ApplicationPreservedFields
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSpec">ApplicationSetSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>annotations</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSet">ApplicationSet
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetWatchEvent">ApplicationSetWatchEvent</a>)
</p>
<p>
<p>ApplicationSet is a set of Application resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSpec">
ApplicationSetSpec
</a>
</em>
</td>
<td>
<p>Common: shared with Application</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>goTemplate</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>generators</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">
[]ApplicationSetGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>syncPolicy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSyncPolicy">
ApplicationSetSyncPolicy
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>strategy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStrategy">
ApplicationSetStrategy
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>preservedFields</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationPreservedFields">
ApplicationPreservedFields
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>goTemplateOptions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>applyNestedSelectors</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ApplyNestedSelectors enables selectors defined within the generators of two level-nested matrix or merge generators.</p>
<p>Deprecated: This field is ignored, and the behavior is always enabled. The field will be removed in a future
version of the ApplicationSet CRD.</p>
</td>
</tr>
<tr>
<td>
<code>ignoreApplicationDifferences</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetIgnoreDifferences">
ApplicationSetIgnoreDifferences
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>templatePatch</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStatus">
ApplicationSetStatus
</a>
</em>
</td>
<td>
<p>Common: shared with Application (different type)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetApplicationStatus">ApplicationSetApplicationStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStatus">ApplicationSetStatus</a>)
</p>
<p>
<p>ApplicationSetApplicationStatus contains details about each Application managed by the ApplicationSet</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>application</code><br/>
<em>
string
</em>
</td>
<td>
<p>Application contains the name of the Application resource</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>LastTransitionTime is the time the status was last updated</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message contains human-readable message indicating details about the status</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ProgressiveSyncStatusCode">
ProgressiveSyncStatusCode
</a>
</em>
</td>
<td>
<p>Status contains the AppSet&rsquo;s perceived status of the managed Application resource</p>
</td>
</tr>
<tr>
<td>
<code>step</code><br/>
<em>
string
</em>
</td>
<td>
<p>Step tracks which step this Application should be updated in</p>
</td>
</tr>
<tr>
<td>
<code>targetRevisions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>TargetRevision tracks the desired revisions the Application should be synced to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetCondition">ApplicationSetCondition
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStatus">ApplicationSetStatus</a>)
</p>
<p>
<p>ApplicationSetCondition contains details about an applicationset condition, which is usually an error or warning</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetConditionType">
ApplicationSetConditionType
</a>
</em>
</td>
<td>
<p>Type is an applicationset condition type</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message contains human-readable message indicating details about condition</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>LastTransitionTime is the time the condition was last observed</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetConditionStatus">
ApplicationSetConditionStatus
</a>
</em>
</td>
<td>
<p>True/False/Unknown</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
<p>Single word camelcase representing the reason for the status eg ErrorOccurred</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetConditionStatus">ApplicationSetConditionStatus
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetCondition">ApplicationSetCondition</a>)
</p>
<p>
<p>ApplicationSetConditionStatus is a type which represents possible comparison results</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;False&#34;</p></td>
<td><p>ApplicationSetConditionStatusFalse indicates that a application attempt has failed</p>
</td>
</tr><tr><td><p>&#34;True&#34;</p></td>
<td><p>ApplicationSetConditionStatusTrue indicates that a application has been successfully established</p>
</td>
</tr><tr><td><p>&#34;Unknown&#34;</p></td>
<td><p>ApplicationSetConditionStatusUnknown indicates that the application condition status could not be reliably determined</p>
</td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetConditionType">ApplicationSetConditionType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetCondition">ApplicationSetCondition</a>)
</p>
<p>
<p>ApplicationSetConditionType represents type of application condition. Type name has following convention:
prefix &ldquo;Error&rdquo; means error condition
prefix &ldquo;Warning&rdquo; means warning condition
prefix &ldquo;Info&rdquo; means informational condition</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;ErrorOccurred&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;InvalidRolloutConfig&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ParametersGenerated&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ResourcesUpToDate&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;RolloutProgressing&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSpec">ApplicationSetSpec</a>)
</p>
<p>
<p>ApplicationSetGenerator represents a generator at the top level of an ApplicationSet.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>list</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ListGenerator">
ListGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterGenerator">
ClusterGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>git</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.GitGenerator">
GitGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scmProvider</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">
SCMProviderGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusterDecisionResource</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.DuckTypeGenerator">
DuckTypeGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>pullRequest</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">
PullRequestGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>matrix</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.MatrixGenerator">
MatrixGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>merge</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.MergeGenerator">
MergeGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
</em>
</td>
<td>
<p>Selector allows to post-filter all generator.</p>
</td>
</tr>
<tr>
<td>
<code>plugin</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PluginGenerator">
PluginGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetIgnoreDifferences">ApplicationSetIgnoreDifferences
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.ApplicationSetResourceIgnoreDifferences</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSpec">ApplicationSetSpec</a>)
</p>
<p>
<p>ApplicationSetIgnoreDifferences configures how the ApplicationSet controller will ignore differences in live
applications when applying changes from generated applications.</p>
</p>
<h3 id="argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.MatrixGenerator">MatrixGenerator</a>, 
<a href="#argoproj.io/v1alpha1.MergeGenerator">MergeGenerator</a>)
</p>
<p>
<p>ApplicationSetNestedGenerator represents a generator nested within a combination-type generator (MatrixGenerator or
MergeGenerator).</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>list</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ListGenerator">
ListGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterGenerator">
ClusterGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>git</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.GitGenerator">
GitGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scmProvider</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">
SCMProviderGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusterDecisionResource</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.DuckTypeGenerator">
DuckTypeGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>pullRequest</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">
PullRequestGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>matrix</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Matrix should have the form of NestedMatrixGenerator</p>
</td>
</tr>
<tr>
<td>
<code>merge</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Merge should have the form of NestedMergeGenerator</p>
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
</em>
</td>
<td>
<p>Selector allows to post-filter all generator.</p>
</td>
</tr>
<tr>
<td>
<code>plugin</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PluginGenerator">
PluginGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetNestedGenerators">ApplicationSetNestedGenerators
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.ApplicationSetNestedGenerator</code> alias)</p></h3>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.ApplicationSetReasonType">ApplicationSetReasonType
(<code>string</code> alias)</p></h3>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.ApplicationSetResourceIgnoreDifferences">ApplicationSetResourceIgnoreDifferences
</h3>
<p>
<p>ApplicationSetResourceIgnoreDifferences configures how the ApplicationSet controller will ignore differences in live
applications when applying changes from generated applications.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the application to ignore differences for. If not specified, the rule applies to all applications.</p>
</td>
</tr>
<tr>
<td>
<code>jsonPointers</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>JSONPointers is a list of JSON pointers to fields to ignore differences for.</p>
</td>
</tr>
<tr>
<td>
<code>jqPathExpressions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>JQPathExpressions is a list of JQ path expressions to fields to ignore differences for.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetRolloutStep">ApplicationSetRolloutStep
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetRolloutStrategy">ApplicationSetRolloutStrategy</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>matchExpressions</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationMatchExpression">
[]ApplicationMatchExpression
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>maxUpdate</code><br/>
<em>
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetRolloutStrategy">ApplicationSetRolloutStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStrategy">ApplicationSetStrategy</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetRolloutStep">
[]ApplicationSetRolloutStep
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetSpec">ApplicationSetSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSet">ApplicationSet</a>)
</p>
<p>
<p>ApplicationSetSpec represents a class of application set state.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>goTemplate</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>generators</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">
[]ApplicationSetGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>syncPolicy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSyncPolicy">
ApplicationSetSyncPolicy
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>strategy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStrategy">
ApplicationSetStrategy
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>preservedFields</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationPreservedFields">
ApplicationPreservedFields
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>goTemplateOptions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>applyNestedSelectors</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ApplyNestedSelectors enables selectors defined within the generators of two level-nested matrix or merge generators.</p>
<p>Deprecated: This field is ignored, and the behavior is always enabled. The field will be removed in a future
version of the ApplicationSet CRD.</p>
</td>
</tr>
<tr>
<td>
<code>ignoreApplicationDifferences</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetIgnoreDifferences">
ApplicationSetIgnoreDifferences
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>templatePatch</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetStatus">ApplicationSetStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSet">ApplicationSet</a>)
</p>
<p>
<p>ApplicationSetStatus defines the observed state of ApplicationSet</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetCondition">
[]ApplicationSetCondition
</a>
</em>
</td>
<td>
<p>INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
Important: Run &ldquo;make&rdquo; to regenerate code after modifying this file</p>
</td>
</tr>
<tr>
<td>
<code>applicationStatus</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetApplicationStatus">
[]ApplicationSetApplicationStatus
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceStatus">
[]ResourceStatus
</a>
</em>
</td>
<td>
<p>Resources is a list of Applications resources managed by this application set.</p>
</td>
</tr>
<tr>
<td>
<code>resourcesCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>ResourcesCount is the total number of resources managed by this application set. The count may be higher than actual number of items in the Resources field when
the number of managed resources exceeds the limit imposed by the controller (to avoid making the status field too large).</p>
</td>
</tr>
<tr>
<td>
<code>health</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HealthStatus">
HealthStatus
</a>
</em>
</td>
<td>
<p>Health contains information about the applicationset&rsquo;s current health status based on the applicationset conditions</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetStrategy">ApplicationSetStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSpec">ApplicationSetSpec</a>)
</p>
<p>
<p>ApplicationSetStrategy configures how generated Applications are updated in sequence.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>rollingSync</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetRolloutStrategy">
ApplicationSetRolloutStrategy
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>deletionOrder</code><br/>
<em>
string
</em>
</td>
<td>
<p>DeletionOrder allows specifying the order for deleting generated apps when progressive sync is enabled.
accepts values &ldquo;AllAtOnce&rdquo; and &ldquo;Reverse&rdquo;</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetSyncPolicy">ApplicationSetSyncPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSpec">ApplicationSetSpec</a>)
</p>
<p>
<p>ApplicationSetSyncPolicy configures how generated Applications will relate to their
ApplicationSet.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>preserveResourcesOnDeletion</code><br/>
<em>
bool
</em>
</td>
<td>
<p>PreserveResourcesOnDeletion will preserve resources on deletion. If PreserveResourcesOnDeletion is set to true, these Applications will not be deleted.</p>
</td>
</tr>
<tr>
<td>
<code>applicationsSync</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationsSyncPolicy">
ApplicationsSyncPolicy
</a>
</em>
</td>
<td>
<p>ApplicationsSync represents the policy applied on the generated applications. Possible values are create-only, create-update, create-delete, sync</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetTemplate">ApplicationSetTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSpec">ApplicationSetSpec</a>, 
<a href="#argoproj.io/v1alpha1.ClusterGenerator">ClusterGenerator</a>, 
<a href="#argoproj.io/v1alpha1.DuckTypeGenerator">DuckTypeGenerator</a>, 
<a href="#argoproj.io/v1alpha1.GitGenerator">GitGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ListGenerator">ListGenerator</a>, 
<a href="#argoproj.io/v1alpha1.MatrixGenerator">MatrixGenerator</a>, 
<a href="#argoproj.io/v1alpha1.MergeGenerator">MergeGenerator</a>, 
<a href="#argoproj.io/v1alpha1.PluginGenerator">PluginGenerator</a>, 
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>ApplicationSetTemplate represents argocd ApplicationSpec</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplateMeta">
ApplicationSetTemplateMeta
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">
ApplicationSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">
ApplicationSource
</a>
</em>
</td>
<td>
<p>Source is a reference to the location of the application&rsquo;s manifests or chart</p>
</td>
</tr>
<tr>
<td>
<code>destination</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestination">
ApplicationDestination
</a>
</em>
</td>
<td>
<p>Destination is a reference to the target Kubernetes server and namespace</p>
</td>
</tr>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Project is a reference to the project this application belongs to.
The empty string means that application belongs to the &lsquo;default&rsquo; project.</p>
</td>
</tr>
<tr>
<td>
<code>syncPolicy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncPolicy">
SyncPolicy
</a>
</em>
</td>
<td>
<p>SyncPolicy controls when and how a sync will be performed</p>
</td>
</tr>
<tr>
<td>
<code>ignoreDifferences</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.IgnoreDifferences">
IgnoreDifferences
</a>
</em>
</td>
<td>
<p>IgnoreDifferences is a list of resources and their fields which should be ignored during comparison</p>
</td>
</tr>
<tr>
<td>
<code>info</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Info">
[]Info
</a>
</em>
</td>
<td>
<p>Info contains a list of information (URLs, email addresses, and plain text) that relates to the application</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code><br/>
<em>
int64
</em>
</td>
<td>
<p>RevisionHistoryLimit limits the number of items kept in the application&rsquo;s revision history, which is used for informational purposes as well as for rollbacks to previous versions.
This should only be changed in exceptional circumstances.
Setting to zero will store no history. This will reduce storage used.
Increasing will increase the space used to store the history, so we do not recommend increasing it.
Default is 10.</p>
</td>
</tr>
<tr>
<td>
<code>sources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSources">
ApplicationSources
</a>
</em>
</td>
<td>
<p>Sources is a reference to the location of the application&rsquo;s manifests or chart</p>
</td>
</tr>
<tr>
<td>
<code>sourceHydrator</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">
SourceHydrator
</a>
</em>
</td>
<td>
<p>SourceHydrator provides a way to push hydrated manifests back to git before syncing them to the cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetTemplateMeta">ApplicationSetTemplateMeta
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">ApplicationSetTemplate</a>)
</p>
<p>
<p>ApplicationSetTemplateMeta represents the Argo CD application fields that may
be used for Applications generated from the ApplicationSet (based on metav1.ObjectMeta)</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>finalizers</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator
</h3>
<p>
<p>ApplicationSetTerminalGenerator represents a generator nested within a nested generator (for example, a list within
a merge within a matrix). A generator at this level may not be a combination-type generator (MatrixGenerator or
MergeGenerator). ApplicationSet enforces this nesting depth limit because CRDs do not support recursive types.
<a href="https://github.com/kubernetes-sigs/controller-tools/issues/477">https://github.com/kubernetes-sigs/controller-tools/issues/477</a></p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>list</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ListGenerator">
ListGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterGenerator">
ClusterGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>git</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.GitGenerator">
GitGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scmProvider</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">
SCMProviderGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusterDecisionResource</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.DuckTypeGenerator">
DuckTypeGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>pullRequest</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">
PullRequestGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>plugin</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PluginGenerator">
PluginGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
</em>
</td>
<td>
<p>Selector allows to post-filter all generator.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetTerminalGenerators">ApplicationSetTerminalGenerators
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.ApplicationSetTerminalGenerator</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.NestedMatrixGenerator">NestedMatrixGenerator</a>, 
<a href="#argoproj.io/v1alpha1.NestedMergeGenerator">NestedMergeGenerator</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.ApplicationSetTree">ApplicationSetTree
</h3>
<p>
<p>ApplicationSetTree holds nodes which belongs to the application
Used to build a tree of an ApplicationSet and its children</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodes</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceNode">
[]ResourceNode
</a>
</em>
</td>
<td>
<p>Nodes contains list of nodes which are directly managed by the applicationset</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSetWatchEvent">ApplicationSetWatchEvent
</h3>
<p>
<p>ApplicationSetWatchEvent contains information about application change.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
k8s.io/apimachinery/pkg/watch.EventType
</em>
</td>
<td>
<p>Type represents the Kubernetes watch event type. The protobuf tag uses
casttype to ensure the generated Go code keeps this field as
watch.EventType (a strong Go type) instead of falling back to a plain string</p>
</td>
</tr>
<tr>
<td>
<code>applicationSet</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSet">
ApplicationSet
</a>
</em>
</td>
<td>
<p>ApplicationSet is:
* If Type is Added or Modified: the new state of the object.
* If Type is Deleted: the state of the object immediately before deletion.
* If Type is Error: *api.Status is recommended; other types may make sense
depending on context</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSource">ApplicationSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec</a>, 
<a href="#argoproj.io/v1alpha1.ComparedTo">ComparedTo</a>, 
<a href="#argoproj.io/v1alpha1.RevisionHistory">RevisionHistory</a>, 
<a href="#argoproj.io/v1alpha1.SyncOperation">SyncOperation</a>, 
<a href="#argoproj.io/v1alpha1.SyncOperationResult">SyncOperationResult</a>)
</p>
<p>
<p>ApplicationSource contains all required information about the source of an application</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repoURL</code><br/>
<em>
string
</em>
</td>
<td>
<p>RepoURL is the URL to the repository (Git or Helm) that contains the application manifests</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<p>Path is a directory path within the Git repository, and is only valid for applications sourced from Git.</p>
</td>
</tr>
<tr>
<td>
<code>targetRevision</code><br/>
<em>
string
</em>
</td>
<td>
<p>TargetRevision defines the revision of the source to sync the application to.
In case of Git, this can be commit, tag, or branch. If omitted, will equal to HEAD.
In case of Helm, this is a semver tag for the Chart&rsquo;s version.</p>
</td>
</tr>
<tr>
<td>
<code>helm</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceHelm">
ApplicationSourceHelm
</a>
</em>
</td>
<td>
<p>Helm holds helm specific options</p>
</td>
</tr>
<tr>
<td>
<code>kustomize</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceKustomize">
ApplicationSourceKustomize
</a>
</em>
</td>
<td>
<p>Kustomize holds kustomize specific options</p>
</td>
</tr>
<tr>
<td>
<code>directory</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceDirectory">
ApplicationSourceDirectory
</a>
</em>
</td>
<td>
<p>Directory holds path/directory specific options</p>
</td>
</tr>
<tr>
<td>
<code>plugin</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourcePlugin">
ApplicationSourcePlugin
</a>
</em>
</td>
<td>
<p>Plugin holds config management plugin specific options</p>
</td>
</tr>
<tr>
<td>
<code>chart</code><br/>
<em>
string
</em>
</td>
<td>
<p>Chart is a Helm chart name, and must be specified for applications sourced from a Helm repo.</p>
</td>
</tr>
<tr>
<td>
<code>ref</code><br/>
<em>
string
</em>
</td>
<td>
<p>Ref is reference to another source within sources field. This field will not be used if used with a <code>source</code> tag.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is used to refer to a source and is displayed in the UI. It is used in multi-source Applications.</p>
</td>
</tr>
<tr>
<td>
<code>tagPrefix</code><br/>
<em>
string
</em>
</td>
<td>
<p>TagPrefix filters git tags to only those with this prefix before evaluating targetRevision as a semver constraint.
The prefix is stripped from tag names before comparison and re-added to the resolved version.
For example, with tagPrefix &ldquo;component-b/&rdquo; and targetRevision &ldquo;1.0.*&rdquo;, tags like &ldquo;component-b/1.0.0&rdquo; and
&ldquo;component-b/1.0.1&rdquo; are candidates, and the constraint resolves to &ldquo;component-b/1.0.1&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSourceDirectory">ApplicationSourceDirectory
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">ApplicationSource</a>, 
<a href="#argoproj.io/v1alpha1.DrySource">DrySource</a>)
</p>
<p>
<p>ApplicationSourceDirectory holds options for applications of type plain YAML or Jsonnet</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>recurse</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Recurse specifies whether to scan a directory recursively for manifests</p>
</td>
</tr>
<tr>
<td>
<code>jsonnet,omitempty,omitzero</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceJsonnet">
ApplicationSourceJsonnet
</a>
</em>
</td>
<td>
<p>Jsonnet holds options specific to Jsonnet</p>
</td>
</tr>
<tr>
<td>
<code>exclude</code><br/>
<em>
string
</em>
</td>
<td>
<p>Exclude contains a glob pattern to match paths against that should be explicitly excluded from being used during manifest generation</p>
</td>
</tr>
<tr>
<td>
<code>include</code><br/>
<em>
string
</em>
</td>
<td>
<p>Include contains a glob pattern to match paths against that should be explicitly included during manifest generation</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSourceHelm">ApplicationSourceHelm
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">ApplicationSource</a>, 
<a href="#argoproj.io/v1alpha1.DrySource">DrySource</a>)
</p>
<p>
<p>ApplicationSourceHelm holds helm specific options</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>valueFiles</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>ValuesFiles is a list of Helm value files to use when generating a template</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HelmParameter">
[]HelmParameter
</a>
</em>
</td>
<td>
<p>Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation</p>
</td>
</tr>
<tr>
<td>
<code>releaseName</code><br/>
<em>
string
</em>
</td>
<td>
<p>ReleaseName is the Helm release name to use. If omitted it will use the application name</p>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
string
</em>
</td>
<td>
<p>Values specifies Helm values to be passed to helm template, typically defined as a block. ValuesObject takes precedence over Values, so use one or the other.</p>
</td>
</tr>
<tr>
<td>
<code>fileParameters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HelmFileParameter">
[]HelmFileParameter
</a>
</em>
</td>
<td>
<p>FileParameters are file parameters to the helm template</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<p>Version is the Helm version to use for templating (&ldquo;3&rdquo;)</p>
</td>
</tr>
<tr>
<td>
<code>passCredentials</code><br/>
<em>
bool
</em>
</td>
<td>
<p>PassCredentials pass credentials to all domains (Helm&rsquo;s &ndash;pass-credentials)</p>
</td>
</tr>
<tr>
<td>
<code>ignoreMissingValueFiles</code><br/>
<em>
bool
</em>
</td>
<td>
<p>IgnoreMissingValueFiles prevents helm template from failing when valueFiles do not exist locally by not appending them to helm template &ndash;values</p>
</td>
</tr>
<tr>
<td>
<code>skipCrds</code><br/>
<em>
bool
</em>
</td>
<td>
<p>SkipCrds skips custom resource definition installation step (Helm&rsquo;s &ndash;skip-crds)</p>
</td>
</tr>
<tr>
<td>
<code>valuesObject</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>ValuesObject specifies Helm values to be passed to helm template, defined as a map. This takes precedence over Values.</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace is an optional namespace to template with. If left empty, defaults to the app&rsquo;s destination namespace.</p>
</td>
</tr>
<tr>
<td>
<code>kubeVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
uses the Kubernetes version of the target cluster.</p>
</td>
</tr>
<tr>
<td>
<code>apiVersions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>APIVersions specifies the Kubernetes resource API versions to pass to Helm when templating manifests. By default,
Argo CD uses the API versions of the target cluster. The format is [group/]version/kind.</p>
</td>
</tr>
<tr>
<td>
<code>skipTests</code><br/>
<em>
bool
</em>
</td>
<td>
<p>SkipTests skips test manifest installation step (Helm&rsquo;s &ndash;skip-tests).</p>
</td>
</tr>
<tr>
<td>
<code>skipSchemaValidation</code><br/>
<em>
bool
</em>
</td>
<td>
<p>SkipSchemaValidation skips JSON schema validation (Helm&rsquo;s &ndash;skip-schema-validation)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSourceJsonnet">ApplicationSourceJsonnet
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceDirectory">ApplicationSourceDirectory</a>)
</p>
<p>
<p>ApplicationSourceJsonnet holds options specific to applications of type Jsonnet</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>extVars</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.JsonnetVar">
[]JsonnetVar
</a>
</em>
</td>
<td>
<p>ExtVars is a list of Jsonnet External Variables</p>
</td>
</tr>
<tr>
<td>
<code>tlas</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.JsonnetVar">
[]JsonnetVar
</a>
</em>
</td>
<td>
<p>TLAS is a list of Jsonnet Top-level Arguments</p>
</td>
</tr>
<tr>
<td>
<code>libs</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Additional library search dirs</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSourceKustomize">ApplicationSourceKustomize
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">ApplicationSource</a>, 
<a href="#argoproj.io/v1alpha1.DrySource">DrySource</a>)
</p>
<p>
<p>ApplicationSourceKustomize holds options specific to an Application source specific to Kustomize</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>namePrefix</code><br/>
<em>
string
</em>
</td>
<td>
<p>NamePrefix overrides the namePrefix in the kustomization.yaml for Kustomize apps</p>
</td>
</tr>
<tr>
<td>
<code>nameSuffix</code><br/>
<em>
string
</em>
</td>
<td>
<p>NameSuffix overrides the nameSuffix in the kustomization.yaml for Kustomize apps</p>
</td>
</tr>
<tr>
<td>
<code>images</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KustomizeImages">
KustomizeImages
</a>
</em>
</td>
<td>
<p>Images is a list of Kustomize image override specifications</p>
</td>
</tr>
<tr>
<td>
<code>commonLabels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>CommonLabels is a list of additional labels to add to rendered manifests</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<p>Version controls which version of Kustomize to use for rendering manifests</p>
</td>
</tr>
<tr>
<td>
<code>commonAnnotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>CommonAnnotations is a list of additional annotations to add to rendered manifests</p>
</td>
</tr>
<tr>
<td>
<code>forceCommonLabels</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ForceCommonLabels specifies whether to force applying common labels to resources for Kustomize apps</p>
</td>
</tr>
<tr>
<td>
<code>forceCommonAnnotations</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ForceCommonAnnotations specifies whether to force applying common annotations to resources for Kustomize apps</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace sets the namespace that Kustomize adds to all resources</p>
</td>
</tr>
<tr>
<td>
<code>commonAnnotationsEnvsubst</code><br/>
<em>
bool
</em>
</td>
<td>
<p>CommonAnnotationsEnvsubst specifies whether to apply env variables substitution for annotation values</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KustomizeReplicas">
KustomizeReplicas
</a>
</em>
</td>
<td>
<p>Replicas is a list of Kustomize Replicas override specifications</p>
</td>
</tr>
<tr>
<td>
<code>patches</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KustomizePatches">
KustomizePatches
</a>
</em>
</td>
<td>
<p>Patches is a list of Kustomize patches</p>
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Components specifies a list of kustomize components to add to the kustomization before building</p>
</td>
</tr>
<tr>
<td>
<code>ignoreMissingComponents</code><br/>
<em>
bool
</em>
</td>
<td>
<p>IgnoreMissingComponents prevents kustomize from failing when components do not exist locally by not appending them to kustomization file</p>
</td>
</tr>
<tr>
<td>
<code>labelWithoutSelector</code><br/>
<em>
bool
</em>
</td>
<td>
<p>LabelWithoutSelector specifies whether to apply common labels to resource selectors or not</p>
</td>
</tr>
<tr>
<td>
<code>kubeVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
uses the Kubernetes version of the target cluster.</p>
</td>
</tr>
<tr>
<td>
<code>apiVersions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>APIVersions specifies the Kubernetes resource API versions to pass to Helm when templating manifests. By default,
Argo CD uses the API versions of the target cluster. The format is [group/]version/kind.</p>
</td>
</tr>
<tr>
<td>
<code>labelIncludeTemplates</code><br/>
<em>
bool
</em>
</td>
<td>
<p>LabelIncludeTemplates specifies whether to apply common labels to resource templates or not</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSourcePlugin">ApplicationSourcePlugin
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">ApplicationSource</a>, 
<a href="#argoproj.io/v1alpha1.DrySource">DrySource</a>)
</p>
<p>
<p>ApplicationSourcePlugin holds options specific to config management plugins</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Env">
Env
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourcePluginParameters">
ApplicationSourcePluginParameters
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSourcePluginParameter">ApplicationSourcePluginParameter
</h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name identifying a parameter.</p>
</td>
</tr>
<tr>
<td>
<code>string</code><br/>
<em>
string
</em>
</td>
<td>
<p>String_ is the value of a string type parameter.</p>
</td>
</tr>
<tr>
<td>
<code>OptionalMap</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OptionalMap">
OptionalMap
</a>
</em>
</td>
<td>
<p>Map is the value of a map type parameter.</p>
</td>
</tr>
<tr>
<td>
<code>OptionalArray</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OptionalArray">
OptionalArray
</a>
</em>
</td>
<td>
<p>Array is the value of an array type parameter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSourcePluginParameters">ApplicationSourcePluginParameters
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.ApplicationSourcePluginParameter</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourcePlugin">ApplicationSourcePlugin</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.ApplicationSourceType">ApplicationSourceType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>ApplicationSourceType specifies the type of the application&rsquo;s source</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Directory&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Helm&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Kustomize&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Plugin&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSources">ApplicationSources
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.ApplicationSource</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec</a>, 
<a href="#argoproj.io/v1alpha1.ComparedTo">ComparedTo</a>, 
<a href="#argoproj.io/v1alpha1.RevisionHistory">RevisionHistory</a>, 
<a href="#argoproj.io/v1alpha1.SyncOperation">SyncOperation</a>, 
<a href="#argoproj.io/v1alpha1.SyncOperationResult">SyncOperationResult</a>)
</p>
<p>
<p>ApplicationSources contains list of required information about the sources of an application</p>
</p>
<h3 id="argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Application">Application</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">ApplicationSetTemplate</a>)
</p>
<p>
<p>ApplicationSpec represents desired application state. Contains link to repository with application definition and additional parameters link definition revision.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">
ApplicationSource
</a>
</em>
</td>
<td>
<p>Source is a reference to the location of the application&rsquo;s manifests or chart</p>
</td>
</tr>
<tr>
<td>
<code>destination</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestination">
ApplicationDestination
</a>
</em>
</td>
<td>
<p>Destination is a reference to the target Kubernetes server and namespace</p>
</td>
</tr>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Project is a reference to the project this application belongs to.
The empty string means that application belongs to the &lsquo;default&rsquo; project.</p>
</td>
</tr>
<tr>
<td>
<code>syncPolicy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncPolicy">
SyncPolicy
</a>
</em>
</td>
<td>
<p>SyncPolicy controls when and how a sync will be performed</p>
</td>
</tr>
<tr>
<td>
<code>ignoreDifferences</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.IgnoreDifferences">
IgnoreDifferences
</a>
</em>
</td>
<td>
<p>IgnoreDifferences is a list of resources and their fields which should be ignored during comparison</p>
</td>
</tr>
<tr>
<td>
<code>info</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Info">
[]Info
</a>
</em>
</td>
<td>
<p>Info contains a list of information (URLs, email addresses, and plain text) that relates to the application</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code><br/>
<em>
int64
</em>
</td>
<td>
<p>RevisionHistoryLimit limits the number of items kept in the application&rsquo;s revision history, which is used for informational purposes as well as for rollbacks to previous versions.
This should only be changed in exceptional circumstances.
Setting to zero will store no history. This will reduce storage used.
Increasing will increase the space used to store the history, so we do not recommend increasing it.
Default is 10.</p>
</td>
</tr>
<tr>
<td>
<code>sources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSources">
ApplicationSources
</a>
</em>
</td>
<td>
<p>Sources is a reference to the location of the application&rsquo;s manifests or chart</p>
</td>
</tr>
<tr>
<td>
<code>sourceHydrator</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">
SourceHydrator
</a>
</em>
</td>
<td>
<p>SourceHydrator provides a way to push hydrated manifests back to git before syncing them to the cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Application">Application</a>)
</p>
<p>
<p>ApplicationStatus contains status information for the application</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceStatus">
[]ResourceStatus
</a>
</em>
</td>
<td>
<p>Resources is a list of Kubernetes resources managed by this application</p>
</td>
</tr>
<tr>
<td>
<code>sync</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncStatus">
SyncStatus
</a>
</em>
</td>
<td>
<p>Sync contains information about the application&rsquo;s current sync status</p>
</td>
</tr>
<tr>
<td>
<code>health</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.AppHealthStatus">
AppHealthStatus
</a>
</em>
</td>
<td>
<p>Health contains information about the application&rsquo;s current health status</p>
</td>
</tr>
<tr>
<td>
<code>history</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.RevisionHistories">
RevisionHistories
</a>
</em>
</td>
<td>
<p>History contains information about the application&rsquo;s sync history</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationCondition">
[]ApplicationCondition
</a>
</em>
</td>
<td>
<p>Conditions is a list of currently observed application conditions</p>
</td>
</tr>
<tr>
<td>
<code>reconciledAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>ReconciledAt indicates when the application state was reconciled using the latest git version</p>
</td>
</tr>
<tr>
<td>
<code>operationState</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OperationState">
OperationState
</a>
</em>
</td>
<td>
<p>OperationState contains information about any ongoing operations, such as a sync</p>
</td>
</tr>
<tr>
<td>
<code>observedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>ObservedAt indicates when the application state was updated without querying latest git state</p>
<p>Deprecated: controller no longer updates ObservedAt field</p>
</td>
</tr>
<tr>
<td>
<code>sourceType</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceType">
ApplicationSourceType
</a>
</em>
</td>
<td>
<p>SourceType specifies the type of this application</p>
</td>
</tr>
<tr>
<td>
<code>summary</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSummary">
ApplicationSummary
</a>
</em>
</td>
<td>
<p>Summary contains a list of URLs and container images used by this application</p>
</td>
</tr>
<tr>
<td>
<code>resourceHealthSource</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceHealthLocation">
ResourceHealthLocation
</a>
</em>
</td>
<td>
<p>ResourceHealthSource indicates where the resource health status is stored: inline if not set or appTree</p>
</td>
</tr>
<tr>
<td>
<code>sourceTypes</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceType">
[]ApplicationSourceType
</a>
</em>
</td>
<td>
<p>SourceTypes specifies the type of the sources included in the application</p>
</td>
</tr>
<tr>
<td>
<code>controllerNamespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>ControllerNamespace indicates the namespace in which the application controller is located</p>
</td>
</tr>
<tr>
<td>
<code>sourceHydrator</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceHydratorStatus">
SourceHydratorStatus
</a>
</em>
</td>
<td>
<p>SourceHydrator stores information about the current state of source hydration</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationSummary">ApplicationSummary
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>ApplicationSummary contains information about URLs and container images used by an application</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>externalURLs</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>ExternalURLs holds all external URLs of application child resources.</p>
</td>
</tr>
<tr>
<td>
<code>images</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Images holds all images of application child resources.</p>
</td>
</tr>
<tr>
<td>
<code>isAppOfApps</code><br/>
<em>
bool
</em>
</td>
<td>
<p>IsAppOfApps holds true if the application has any application for child resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationTree">ApplicationTree
</h3>
<p>
<p>ApplicationTree represents the hierarchical structure of resources associated with an Argo CD application.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodes</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceNode">
[]ResourceNode
</a>
</em>
</td>
<td>
<p>Nodes contains a list of resources that are either directly managed by the application
or are children of directly managed resources.</p>
</td>
</tr>
<tr>
<td>
<code>orphanedNodes</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceNode">
[]ResourceNode
</a>
</em>
</td>
<td>
<p>OrphanedNodes contains resources that exist in the same namespace as the application
but are not managed by it. This list is populated only if orphaned resource tracking
is enabled in the application&rsquo;s project settings.</p>
</td>
</tr>
<tr>
<td>
<code>hosts</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HostInfo">
[]HostInfo
</a>
</em>
</td>
<td>
<p>Hosts provides a list of Kubernetes nodes that are running pods related to the application.</p>
</td>
</tr>
<tr>
<td>
<code>shardsCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>ShardsCount represents the total number of shards the application tree is split into.
This is used to distribute resource processing across multiple shards.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationWatchEvent">ApplicationWatchEvent
</h3>
<p>
<p>ApplicationWatchEvent contains information about application change.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
k8s.io/apimachinery/pkg/watch.EventType
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>application</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Application">
Application
</a>
</em>
</td>
<td>
<p>Application is:
* If Type is Added or Modified: the new state of the object.
* If Type is Deleted: the state of the object immediately before deletion.
* If Type is Error: *api.Status is recommended; other types may make sense
depending on context.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ApplicationsSyncPolicy">ApplicationsSyncPolicy
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetSyncPolicy">ApplicationSetSyncPolicy</a>)
</p>
<p>
<p>ApplicationsSyncPolicy representation
&ldquo;create-only&rdquo; means applications are only created. If the generator&rsquo;s result contains update, applications won&rsquo;t be updated
&ldquo;create-update&rdquo; means applications are only created/Updated. If the generator&rsquo;s result contains update, applications will be updated, but not deleted
&ldquo;create-delete&rdquo; means applications are only created/deleted. If the generator&rsquo;s result contains update, applications won&rsquo;t be updated, if it results in deleted applications, the applications will be deleted
&ldquo;sync&rdquo; means create/update/deleted. If the generator&rsquo;s result contains update, applications will be updated, if it results in deleted applications, the applications will be deleted
If no ApplicationsSyncPolicy is defined, it defaults it to sync</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;create-delete&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;create-only&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;create-update&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;sync&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.Backoff">Backoff
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.RetryStrategy">RetryStrategy</a>)
</p>
<p>
<p>Backoff is the backoff strategy to use on subsequent retries for failing syncs</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>duration</code><br/>
<em>
string
</em>
</td>
<td>
<p>Duration is the amount to back off. Default unit is seconds, but could also be a duration (e.g. &ldquo;2m&rdquo;, &ldquo;1h&rdquo;)</p>
</td>
</tr>
<tr>
<td>
<code>factor</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Factor is a factor to multiply the base duration after each failed retry</p>
</td>
</tr>
<tr>
<td>
<code>maxDuration</code><br/>
<em>
string
</em>
</td>
<td>
<p>MaxDuration is the maximum amount of time allowed for the backoff strategy</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.BasicAuthBitbucketServer">BasicAuthBitbucketServer
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorBitbucket">PullRequestGeneratorBitbucket</a>, 
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorBitbucketServer">PullRequestGeneratorBitbucketServer</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorBitbucketServer">SCMProviderGeneratorBitbucketServer</a>)
</p>
<p>
<p>BasicAuthBitbucketServer defines the username/(password or personal access token) for Basic auth.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>username</code><br/>
<em>
string
</em>
</td>
<td>
<p>Username for Basic auth</p>
</td>
</tr>
<tr>
<td>
<code>passwordRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Password (or personal access token) reference.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.BearerTokenBitbucket">BearerTokenBitbucket
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorBitbucketServer">PullRequestGeneratorBitbucketServer</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorBitbucketServer">SCMProviderGeneratorBitbucketServer</a>)
</p>
<p>
<p>BearerTokenBitbucket defines the Bearer token for BitBucket AppToken auth.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Password (or personal access token) reference.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.BearerTokenBitbucketCloud">BearerTokenBitbucketCloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorBitbucket">PullRequestGeneratorBitbucket</a>)
</p>
<p>
<p>BearerTokenBitbucketCloud defines the Bearer token for BitBucket AppToken auth.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Password (or personal access token) reference.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ChartDetails">ChartDetails
</h3>
<p>
<p>ChartDetails contains helm chart metadata for a specific version</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>home</code><br/>
<em>
string
</em>
</td>
<td>
<p>The URL of this projects home page, e.g. &ldquo;<a href="http://example.com&quot;">http://example.com&rdquo;</a></p>
</td>
</tr>
<tr>
<td>
<code>maintainers</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>List of maintainer details, name and email, e.g. [&ldquo;John Doe <john_doe@my-company.com>&rdquo;]</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.Cluster">Cluster
</h3>
<p>
<p>Cluster is the definition of a cluster resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>-</code><br/>
<em>
string
</em>
</td>
<td>
<p>ID is an internal field cluster identifier. Not exposed via API.</p>
</td>
</tr>
<tr>
<td>
<code>server</code><br/>
<em>
string
</em>
</td>
<td>
<p>Server is the API server URL of the Kubernetes cluster</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the cluster. If omitted, will use the server address</p>
</td>
</tr>
<tr>
<td>
<code>config</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterConfig">
ClusterConfig
</a>
</em>
</td>
<td>
<p>Config holds cluster information for connecting to a cluster.
In a cluster Secret, marshaled as JSON under the key &ldquo;config&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>connectionState</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ConnectionState">
ConnectionState
</a>
</em>
</td>
<td>
<p>Deprecated: use Info.ConnectionState field instead.
ConnectionState contains information about cluster connection state.
Not stored in cluster Secrets; populated at runtime by the API.</p>
</td>
</tr>
<tr>
<td>
<code>serverVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>Deprecated: use Info.ServerVersion field instead.
The server version.
Not stored in cluster Secrets; populated at runtime by the API.</p>
</td>
</tr>
<tr>
<td>
<code>namespaces</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Holds list of namespaces which are accessible in that cluster. Cluster level resources will be ignored if namespace list is not empty.
In a cluster Secret, stored as a comma-separated string under the key &ldquo;namespaces&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>refreshRequestedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>RefreshRequestedAt holds time when cluster cache refresh has been requested.
In a cluster Secret, stored as an RFC3339 timestamp in the annotation &ldquo;argocd.argoproj.io/refresh&rdquo;, not in stringData.</p>
</td>
</tr>
<tr>
<td>
<code>info</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterInfo">
ClusterInfo
</a>
</em>
</td>
<td>
<p>Info holds information about cluster cache and state.
Not stored in cluster Secrets; populated at runtime by the API.</p>
</td>
</tr>
<tr>
<td>
<code>shard</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Shard contains optional shard number. Calculated on the fly by the application controller if not specified.
In a cluster Secret, stored as a decimal string under the key &ldquo;shard&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>clusterResources</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Indicates if cluster level resources should be managed. This setting is used only if cluster is connected in a namespaced mode.
In a cluster Secret, stored as the string &ldquo;true&rdquo; or &ldquo;false&rdquo; under the key &ldquo;clusterResources&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Reference between project and cluster that allow you automatically to be added as item inside Destinations project entity</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Labels for cluster secret metadata.
In a cluster Secret, stored in metadata.labels (not stringData).</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations for cluster secret metadata.
In a cluster Secret, stored in metadata.annotations (not stringData).</p>
</td>
</tr>
<tr>
<td>
<code>-</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
</em>
</td>
<td>
<em>(Optional)</em>
<p>The embedded metav1.ObjectMeta field is purely here to please the informer when converting from a v1.Secret to a Cluster.
More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata</a></p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ClusterCacheInfo">ClusterCacheInfo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ClusterInfo">ClusterInfo</a>)
</p>
<p>
<p>ClusterCacheInfo contains information about the cluster cache</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resourcesCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>ResourcesCount holds number of observed Kubernetes resources</p>
</td>
</tr>
<tr>
<td>
<code>apisCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>APIsCount holds number of observed Kubernetes API count</p>
</td>
</tr>
<tr>
<td>
<code>lastCacheSyncTime</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>LastCacheSyncTime holds time of most recent cache synchronization</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ClusterConfig">ClusterConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Cluster">Cluster</a>)
</p>
<p>
<p>ClusterConfig is the configuration attributes. This structure is subset of the go-client
rest.Config with annotations added for marshalling.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>username</code><br/>
<em>
string
</em>
</td>
<td>
<p>Server requires Basic authentication</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bearerToken</code><br/>
<em>
string
</em>
</td>
<td>
<p>Server requires Bearer authentication. This client will not attempt to use
refresh tokens for an OAuth2 flow.
TODO: demonstrate an OAuth2 compatible client.</p>
</td>
</tr>
<tr>
<td>
<code>tlsClientConfig</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.TLSClientConfig">
TLSClientConfig
</a>
</em>
</td>
<td>
<p>TLSClientConfig contains settings to enable transport layer security</p>
</td>
</tr>
<tr>
<td>
<code>awsAuthConfig</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.AWSAuthConfig">
AWSAuthConfig
</a>
</em>
</td>
<td>
<p>AWSAuthConfig contains IAM authentication configuration</p>
</td>
</tr>
<tr>
<td>
<code>execProviderConfig</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ExecProviderConfig">
ExecProviderConfig
</a>
</em>
</td>
<td>
<p>ExecProviderConfig contains configuration for an exec provider</p>
</td>
</tr>
<tr>
<td>
<code>disableCompression</code><br/>
<em>
bool
</em>
</td>
<td>
<p>DisableCompression bypasses automatic GZip compression requests to the server.</p>
</td>
</tr>
<tr>
<td>
<code>proxyUrl</code><br/>
<em>
string
</em>
</td>
<td>
<p>ProxyURL is the URL to the proxy to be used for all requests send to the server</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ClusterGenerator">ClusterGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator</a>)
</p>
<p>
<p>ClusterGenerator defines a generator to match against clusters registered with ArgoCD.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>selector</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
</em>
</td>
<td>
<p>Selector defines a label selector to match against all clusters registered with ArgoCD.
Clusters today are stored as Kubernetes Secrets, thus the Secret labels will be used
for matching the selector.</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Values contains key/value pairs which are passed directly as parameters to the template</p>
</td>
</tr>
<tr>
<td>
<code>flatList</code><br/>
<em>
bool
</em>
</td>
<td>
<p>returns the clusters a single &lsquo;clusters&rsquo; value in the template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ClusterInfo">ClusterInfo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Cluster">Cluster</a>)
</p>
<p>
<p>ClusterInfo contains information about the cluster</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>connectionState</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ConnectionState">
ConnectionState
</a>
</em>
</td>
<td>
<p>ConnectionState contains information about the connection to the cluster</p>
</td>
</tr>
<tr>
<td>
<code>serverVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>ServerVersion contains information about the Kubernetes version of the cluster</p>
</td>
</tr>
<tr>
<td>
<code>cacheInfo</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ClusterCacheInfo">
ClusterCacheInfo
</a>
</em>
</td>
<td>
<p>CacheInfo contains information about the cluster cache</p>
</td>
</tr>
<tr>
<td>
<code>applicationsCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>ApplicationsCount is the number of applications managed by Argo CD on the cluster</p>
</td>
</tr>
<tr>
<td>
<code>apiVersions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>APIVersions contains list of API versions supported by the cluster</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ClusterResourceRestrictionItem">ClusterResourceRestrictionItem
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>)
</p>
<p>
<p>ClusterResourceRestrictionItem is a cluster resource that is restricted by the project&rsquo;s whitelist or blacklist</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the restricted resource. Glob patterns using Go&rsquo;s filepath.Match syntax are supported.
Unlike the group and kind fields, if no name is specified, all resources of the specified group/kind are matched.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.Command">Command
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ConfigManagementPlugin">ConfigManagementPlugin</a>)
</p>
<p>
<p>Command holds binary path and arguments list</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.CommitMetadata">CommitMetadata
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.RevisionReference">RevisionReference</a>)
</p>
<p>
<p>CommitMetadata contains metadata about a commit that is related in some way to another commit.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>author</code><br/>
<em>
string
</em>
</td>
<td>
<p>Author is the author of the commit, i.e. <code>git show -s --format=%an &lt;%ae&gt;</code>.
Must be formatted according to RFC 5322 (mail.Address.String()).
Comes from the Argocd-reference-commit-author trailer.</p>
</td>
</tr>
<tr>
<td>
<code>date</code><br/>
<em>
string
</em>
</td>
<td>
<p>Date is the date of the commit, formatted as by <code>git show -s --format=%aI</code> (RFC 3339).
It can also be an empty string if the date is unknown.
Comes from the Argocd-reference-commit-date trailer.</p>
</td>
</tr>
<tr>
<td>
<code>subject</code><br/>
<em>
string
</em>
</td>
<td>
<p>Subject is the commit message subject line, i.e. <code>git show -s --format=%s</code>.
Comes from the Argocd-reference-commit-subject trailer.</p>
</td>
</tr>
<tr>
<td>
<code>body</code><br/>
<em>
string
</em>
</td>
<td>
<p>Body is the commit message body minus the subject line, i.e. <code>git show -s --format=%b</code>.
Comes from the Argocd-reference-commit-body trailer.</p>
</td>
</tr>
<tr>
<td>
<code>sha</code><br/>
<em>
string
</em>
</td>
<td>
<p>SHA is the commit hash.
Comes from the Argocd-reference-commit-sha trailer.</p>
</td>
</tr>
<tr>
<td>
<code>repoUrl</code><br/>
<em>
string
</em>
</td>
<td>
<p>RepoURL is the URL of the repository where the commit is located.
Comes from the Argocd-reference-commit-repourl trailer.
This value is not validated and should not be used to construct UI links unless it is properly
validated and/or sanitized first.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ComparedTo">ComparedTo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncStatus">SyncStatus</a>)
</p>
<p>
<p>ComparedTo contains application source and target which was used for resources comparison</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">
ApplicationSource
</a>
</em>
</td>
<td>
<p>Source is a reference to the application&rsquo;s source used for comparison</p>
</td>
</tr>
<tr>
<td>
<code>destination</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationDestination">
ApplicationDestination
</a>
</em>
</td>
<td>
<p>Destination is a reference to the application&rsquo;s destination used for comparison</p>
</td>
</tr>
<tr>
<td>
<code>sources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSources">
ApplicationSources
</a>
</em>
</td>
<td>
<p>Sources is a reference to the application&rsquo;s multiple sources used for comparison</p>
</td>
</tr>
<tr>
<td>
<code>ignoreDifferences</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.IgnoreDifferences">
IgnoreDifferences
</a>
</em>
</td>
<td>
<p>IgnoreDifferences is a reference to the application&rsquo;s ignored differences used for comparison</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ComponentParameter">ComponentParameter
</h3>
<p>
<p>ComponentParameter contains information about component parameter value</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>component</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ConfigManagementPlugin">ConfigManagementPlugin
</h3>
<p>
<p>ConfigManagementPlugin contains config management plugin configuration</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>init</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Command">
Command
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>generate</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Command">
Command
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>lockRepo</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ConfigMapKeyRef">ConfigMapKeyRef
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorBitbucketServer">PullRequestGeneratorBitbucketServer</a>, 
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorGitLab">PullRequestGeneratorGitLab</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorBitbucketServer">SCMProviderGeneratorBitbucketServer</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorGitlab">SCMProviderGeneratorGitlab</a>)
</p>
<p>
<p>ConfigMapKeyRef struct for a reference to a configmap key.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>configMapName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ConnectionState">ConnectionState
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Cluster">Cluster</a>, 
<a href="#argoproj.io/v1alpha1.ClusterInfo">ClusterInfo</a>, 
<a href="#argoproj.io/v1alpha1.Repository">Repository</a>)
</p>
<p>
<p>ConnectionState contains information about remote resource connection state, currently used for clusters and repositories</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>status</code><br/>
<em>
string
</em>
</td>
<td>
<p>Status contains the current status indicator for the connection</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message contains human readable information about the connection status</p>
</td>
</tr>
<tr>
<td>
<code>attemptedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>ModifiedAt contains the timestamp when this connection status has been determined</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.DrySource">DrySource
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">SourceHydrator</a>)
</p>
<p>
<p>DrySource specifies a location for dry &ldquo;don&rsquo;t repeat yourself&rdquo; manifest source information.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repoURL</code><br/>
<em>
string
</em>
</td>
<td>
<p>RepoURL is the URL to the git repository that contains the application manifests</p>
</td>
</tr>
<tr>
<td>
<code>targetRevision</code><br/>
<em>
string
</em>
</td>
<td>
<p>TargetRevision defines the revision of the source to hydrate</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<p>Path is a directory path within the Git repository where the manifests are located</p>
</td>
</tr>
<tr>
<td>
<code>helm</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceHelm">
ApplicationSourceHelm
</a>
</em>
</td>
<td>
<p>Helm specifies helm specific options</p>
</td>
</tr>
<tr>
<td>
<code>kustomize</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceKustomize">
ApplicationSourceKustomize
</a>
</em>
</td>
<td>
<p>Kustomize specifies kustomize specific options</p>
</td>
</tr>
<tr>
<td>
<code>directory</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceDirectory">
ApplicationSourceDirectory
</a>
</em>
</td>
<td>
<p>Directory specifies path/directory specific options</p>
</td>
</tr>
<tr>
<td>
<code>plugin</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSourcePlugin">
ApplicationSourcePlugin
</a>
</em>
</td>
<td>
<p>Plugin specifies config management plugin specific options</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.DuckTypeGenerator">DuckTypeGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator</a>)
</p>
<p>
<p>DuckType defines a generator to match against clusters registered with ArgoCD.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>configMapRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>ConfigMapRef is a ConfigMap with the duck type definitions needed to retrieve the data
this includes apiVersion(group/version), kind, matchKey and validation settings
Name is the resource name of the kind, group and version, defined in the ConfigMapRef
RequeueAfterSeconds is how long before the duckType will be rechecked for a change</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>requeueAfterSeconds</code><br/>
<em>
int64
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labelSelector</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Values contains key/value pairs which are passed directly as parameters to the template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.Env">Env
(<code>[]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.EnvEntry</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourcePlugin">ApplicationSourcePlugin</a>)
</p>
<p>
<p>Env is a list of environment variable entries</p>
</p>
<h3 id="argoproj.io/v1alpha1.EnvEntry">EnvEntry
</h3>
<p>
<p>EnvEntry represents an entry in the application&rsquo;s environment</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the variable, usually expressed in uppercase</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<p>Value is the value of the variable</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ExecProviderConfig">ExecProviderConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ClusterConfig">ClusterConfig</a>)
</p>
<p>
<p>ExecProviderConfig is config used to call an external command to perform cluster authentication
See: <a href="https://godoc.org/k8s.io/client-go/tools/clientcmd/api#ExecConfig">https://godoc.org/k8s.io/client-go/tools/clientcmd/api#ExecConfig</a></p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>command</code><br/>
<em>
string
</em>
</td>
<td>
<p>Command to execute</p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Arguments to pass to the command when executing it</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Env defines additional environment variables to expose to the process</p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>Preferred input version of the ExecInfo</p>
</td>
</tr>
<tr>
<td>
<code>installHint</code><br/>
<em>
string
</em>
</td>
<td>
<p>This text is shown to the user when the executable doesn&rsquo;t seem to be present</p>
</td>
</tr>
<tr>
<td>
<code>provideClusterInfo</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ProvideClusterInfo determines whether or not to provide cluster information,
which could potentially contain very large CA data, to this exec plugin as a
part of the KUBERNETES_EXEC_INFO environment variable.
Comment mirrored from k8s.io/client-go/tools/clientcmd/api.ExecConfig.ProvideClusterInfo</p>
</td>
</tr>
<tr>
<td>
<code>config</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>Config holds cluster-specific configuration data that will be passed to the exec plugin
via ExecCredential.Spec.Cluster.Config. This is typically used to pass information like
the cluster name to credential plugins that need it for multi-cluster authentication.</p>
<p>This data is sourced from the kubeconfig cluster&rsquo;s extensions field with the reserved key
&ldquo;client.authentication.k8s.io/exec&rdquo;, as defined by the Kubernetes client authentication API.
Reference: <a href="https://kubernetes.io/docs/reference/config-api/kubeconfig.v1/#ExecConfig">https://kubernetes.io/docs/reference/config-api/kubeconfig.v1/#ExecConfig</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.GitDirectoryGeneratorItem">GitDirectoryGeneratorItem
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.GitGenerator">GitGenerator</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>exclude</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.GitFileGeneratorItem">GitFileGeneratorItem
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.GitGenerator">GitGenerator</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>exclude</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.GitGenerator">GitGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repoURL</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>directories</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.GitDirectoryGeneratorItem">
[]GitDirectoryGeneratorItem
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>files</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.GitFileGeneratorItem">
[]GitFileGeneratorItem
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>revision</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>requeueAfterSeconds</code><br/>
<em>
int64
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>pathParamPrefix</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Values contains key/value pairs which are passed directly as parameters to the template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.GnuPGPublicKey">GnuPGPublicKey
</h3>
<p>
<p>GnuPGPublicKey is a representation of a GnuPG public key</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>keyID</code><br/>
<em>
string
</em>
</td>
<td>
<p>KeyID specifies the key ID, in hexadecimal string format</p>
</td>
</tr>
<tr>
<td>
<code>fingerprint</code><br/>
<em>
string
</em>
</td>
<td>
<p>Fingerprint is the fingerprint of the key</p>
</td>
</tr>
<tr>
<td>
<code>owner</code><br/>
<em>
string
</em>
</td>
<td>
<p>Owner holds the owner identification, e.g. a name and e-mail address</p>
</td>
</tr>
<tr>
<td>
<code>trust</code><br/>
<em>
string
</em>
</td>
<td>
<p>Trust holds the level of trust assigned to this key</p>
</td>
</tr>
<tr>
<td>
<code>subType</code><br/>
<em>
string
</em>
</td>
<td>
<p>SubType holds the key&rsquo;s subtype (e.g. rsa4096)</p>
</td>
</tr>
<tr>
<td>
<code>keyData</code><br/>
<em>
string
</em>
</td>
<td>
<p>KeyData holds the raw key data, in base64 encoded format</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HealthStatus">HealthStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStatus">ApplicationSetStatus</a>, 
<a href="#argoproj.io/v1alpha1.ResourceNode">ResourceNode</a>, 
<a href="#argoproj.io/v1alpha1.ResourceStatus">ResourceStatus</a>)
</p>
<p>
<p>HealthStatus contains information about the currently observed health state of a resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>status</code><br/>
<em>
github.com/argoproj/argo-cd/gitops-engine/pkg/health.HealthStatusCode
</em>
</td>
<td>
<p>Status holds the status code of the resource</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message is a human-readable informational message describing the health status</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>LastTransitionTime is the time the HealthStatus was set or updated</p>
<p>Deprecated: this field is not used and will be removed in a future release.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HelmFileParameter">HelmFileParameter
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceHelm">ApplicationSourceHelm</a>)
</p>
<p>
<p>HelmFileParameter is a file parameter that&rsquo;s passed to helm template during manifest generation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the Helm parameter</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<p>Path is the path to the file containing the values for the Helm parameter</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HelmOptions">HelmOptions
</h3>
<p>
<p>HelmOptions holds helm options</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ValuesFileSchemes</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HelmParameter">HelmParameter
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceHelm">ApplicationSourceHelm</a>)
</p>
<p>
<p>HelmParameter is a parameter that&rsquo;s passed to helm template during manifest generation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the Helm parameter</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<p>Value is the value for the Helm parameter</p>
</td>
</tr>
<tr>
<td>
<code>forceString</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ForceString determines whether to tell Helm to interpret booleans and numbers as strings</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HostInfo">HostInfo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationTree">ApplicationTree</a>)
</p>
<p>
<p>HostInfo holds metadata and resource usage metrics for a specific host in the cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the hostname or node name in the Kubernetes cluster.</p>
</td>
</tr>
<tr>
<td>
<code>resourcesInfo</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HostResourceInfo">
[]HostResourceInfo
</a>
</em>
</td>
<td>
<p>ResourcesInfo provides a list of resource usage details for different resource types on this host.</p>
</td>
</tr>
<tr>
<td>
<code>systemInfo</code><br/>
<em>
k8s.io/api/core/v1.NodeSystemInfo
</em>
</td>
<td>
<p>SystemInfo contains detailed system-level information about the host, such as OS, kernel version, and architecture.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Labels holds the labels attached to the host.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HostResourceInfo">HostResourceInfo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.HostInfo">HostInfo</a>)
</p>
<p>
<p>HostResourceInfo represents resource usage details for a specific resource type on a host.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resourceName</code><br/>
<em>
k8s.io/api/core/v1.ResourceName
</em>
</td>
<td>
<p>ResourceName specifies the type of resource (e.g., CPU, memory, storage).</p>
</td>
</tr>
<tr>
<td>
<code>requestedByApp</code><br/>
<em>
int64
</em>
</td>
<td>
<p>RequestedByApp indicates the total amount of this resource requested by the application running on the host.</p>
</td>
</tr>
<tr>
<td>
<code>requestedByNeighbors</code><br/>
<em>
int64
</em>
</td>
<td>
<p>RequestedByNeighbors indicates the total amount of this resource requested by other workloads on the same host.</p>
</td>
</tr>
<tr>
<td>
<code>capacity</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Capacity represents the total available capacity of this resource on the host.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HydrateOperation">HydrateOperation
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceHydratorStatus">SourceHydratorStatus</a>)
</p>
<p>
<p>HydrateOperation contains information about the most recent hydrate operation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>startedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>StartedAt indicates when the hydrate operation started</p>
</td>
</tr>
<tr>
<td>
<code>finishedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>FinishedAt indicates when the hydrate operation finished</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HydrateOperationPhase">
HydrateOperationPhase
</a>
</em>
</td>
<td>
<p>Phase indicates the status of the hydrate operation</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message contains a message describing the current status of the hydrate operation</p>
</td>
</tr>
<tr>
<td>
<code>drySHA</code><br/>
<em>
string
</em>
</td>
<td>
<p>DrySHA holds the resolved revision (sha) of the dry source as of the most recent reconciliation</p>
</td>
</tr>
<tr>
<td>
<code>hydratedSHA</code><br/>
<em>
string
</em>
</td>
<td>
<p>HydratedSHA holds the resolved revision (sha) of the hydrated source as of the most recent reconciliation</p>
</td>
</tr>
<tr>
<td>
<code>sourceHydrator</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">
SourceHydrator
</a>
</em>
</td>
<td>
<p>SourceHydrator holds the hydrator config used for the hydrate operation</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HydrateOperationPhase">HydrateOperationPhase
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.HydrateOperation">HydrateOperation</a>)
</p>
<p>
<p>HydrateOperationPhase indicates the status of a hydrate operation</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Hydrated&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Hydrating&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HydrateTo">HydrateTo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">SourceHydrator</a>)
</p>
<p>
<p>HydrateTo specifies a branch to which hydrated manifests should be pushed as a &ldquo;staging area&rdquo; before being moved to
the SyncSource. The repository and path are inherited from SyncSource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>targetBranch</code><br/>
<em>
string
</em>
</td>
<td>
<p>TargetBranch is the branch to which hydrated manifests should be committed</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.HydrateType">HydrateType
(<code>string</code> alias)</p></h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;hard&#34;</p></td>
<td><p>HydrateTypeHard forces an app hydration of the dry source</p>
</td>
</tr><tr><td><p>&#34;normal&#34;</p></td>
<td><p>HydrateTypeNormal forces reevaluation of whether the dry requires hydration</p>
</td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.IgnoreDifferences">IgnoreDifferences
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.ResourceIgnoreDifferences</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec</a>, 
<a href="#argoproj.io/v1alpha1.ComparedTo">ComparedTo</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.Info">Info
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.InfoItem">InfoItem
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceNode">ResourceNode</a>)
</p>
<p>
<p>InfoItem contains arbitrary, human readable information about an application</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is a human readable title for this piece of information.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<p>Value is human readable content.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.JWTToken">JWTToken
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.JWTTokens">JWTTokens</a>, 
<a href="#argoproj.io/v1alpha1.ProjectRole">ProjectRole</a>)
</p>
<p>
<p>JWTToken holds the issuedAt and expiresAt values of a token</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>iat</code><br/>
<em>
int64
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>exp</code><br/>
<em>
int64
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>id</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.JWTTokens">JWTTokens
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectStatus">AppProjectStatus</a>)
</p>
<p>
<p>JWTTokens represents a list of JWT tokens</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>items</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.JWTToken">
[]JWTToken
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.JsonnetVar">JsonnetVar
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceJsonnet">ApplicationSourceJsonnet</a>)
</p>
<p>
<p>JsonnetVar represents a variable to be passed to jsonnet during manifest generation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>code</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KnownTypeField">KnownTypeField
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceOverride">ResourceOverride</a>)
</p>
<p>
<p>KnownTypeField contains a mapping between a Custom Resource Definition (CRD) field
and a well-known Kubernetes type. This mapping is primarily used for unit conversions
in resources where the type is not explicitly defined (e.g., converting &ldquo;0.1&rdquo; to &ldquo;100m&rdquo; for CPU requests).</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>field</code><br/>
<em>
string
</em>
</td>
<td>
<p>Field represents the JSON path to the specific field in the CRD that requires type conversion.
Example: &ldquo;spec.resources.requests.cpu&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>Type specifies the expected Kubernetes type for the field, such as &ldquo;cpu&rdquo; or &ldquo;memory&rdquo;.
This helps in converting values between different formats (e.g., &ldquo;0.1&rdquo; to &ldquo;100m&rdquo; for CPU).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KustomizeGvk">KustomizeGvk
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.KustomizeResId">KustomizeResId</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KustomizeImage">KustomizeImage
(<code>string</code> alias)</p></h3>
<p>
<p>KustomizeImage represents a Kustomize image definition in the format [old_image_name=]<image_name>:<image_tag></p>
</p>
<h3 id="argoproj.io/v1alpha1.KustomizeImages">KustomizeImages
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.KustomizeImage</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceKustomize">ApplicationSourceKustomize</a>)
</p>
<p>
<p>KustomizeImages is a list of Kustomize images</p>
</p>
<h3 id="argoproj.io/v1alpha1.KustomizeOptions">KustomizeOptions
</h3>
<p>
<p>KustomizeOptions are options for kustomize to use when building manifests</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>BuildOptions</code><br/>
<em>
string
</em>
</td>
<td>
<p>BuildOptions is a string of build parameters to use when calling <code>kustomize build</code></p>
</td>
</tr>
<tr>
<td>
<code>BinaryPath</code><br/>
<em>
string
</em>
</td>
<td>
<p>BinaryPath holds optional path to kustomize binary</p>
<p>Deprecated: Use settings.Settings instead. See: settings.Settings.KustomizeVersions.
If this field is set, it will be used as the Kustomize binary path.
Otherwise, Versions is used.</p>
</td>
</tr>
<tr>
<td>
<code>Versions</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KustomizeVersion">
[]KustomizeVersion
</a>
</em>
</td>
<td>
<p>Versions is a list of Kustomize versions and their corresponding binary paths and build options.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KustomizePatch">KustomizePatch
</h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>patch</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KustomizeSelector">
KustomizeSelector
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>options</code><br/>
<em>
map[string]bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KustomizePatches">KustomizePatches
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.KustomizePatch</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceKustomize">ApplicationSourceKustomize</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.KustomizeReplica">KustomizeReplica
</h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of Deployment or StatefulSet</p>
</td>
</tr>
<tr>
<td>
<code>count</code><br/>
<em>
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</em>
</td>
<td>
<p>Number of replicas</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KustomizeReplicas">KustomizeReplicas
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.KustomizeReplica</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourceKustomize">ApplicationSourceKustomize</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.KustomizeResId">KustomizeResId
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.KustomizeSelector">KustomizeSelector</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>KustomizeGvk</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KustomizeGvk">
KustomizeGvk
</a>
</em>
</td>
<td>
<p>
(Members of <code>KustomizeGvk</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KustomizeSelector">KustomizeSelector
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.KustomizePatch">KustomizePatch</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>KustomizeResId</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KustomizeResId">
KustomizeResId
</a>
</em>
</td>
<td>
<p>
(Members of <code>KustomizeResId</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>annotationSelector</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labelSelector</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.KustomizeVersion">KustomizeVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.KustomizeOptions">KustomizeOptions</a>)
</p>
<p>
<p>KustomizeVersion holds information about additional Kustomize versions</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name holds Kustomize version name</p>
</td>
</tr>
<tr>
<td>
<code>Path</code><br/>
<em>
string
</em>
</td>
<td>
<p>Path holds the corresponding binary path</p>
</td>
</tr>
<tr>
<td>
<code>BuildOptions</code><br/>
<em>
string
</em>
</td>
<td>
<p>BuildOptions that are specific to a Kustomize version</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ListGenerator">ListGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator</a>)
</p>
<p>
<p>ListGenerator include items info</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>elements</code><br/>
<em>
[]k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>elementsYaml</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ManagedNamespaceMetadata">ManagedNamespaceMetadata
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncOperationResult">SyncOperationResult</a>, 
<a href="#argoproj.io/v1alpha1.SyncPolicy">SyncPolicy</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.MatrixGenerator">MatrixGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>)
</p>
<p>
<p>MatrixGenerator generates the cartesian product of two sets of parameters. The parameters are defined by two nested
generators.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>generators</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">
[]ApplicationSetNestedGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.MergeGenerator">MergeGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>)
</p>
<p>
<p>MergeGenerator merges the output of two or more generators. Where the values for all specified merge keys are equal
between two sets of generated parameters, the parameter sets will be merged with the parameters from the latter
generator taking precedence. Parameter sets with merge keys not present in the base generator&rsquo;s params will be
ignored.
For example, if the first generator produced [{a: &lsquo;1&rsquo;, b: &lsquo;2&rsquo;}, {c: &lsquo;1&rsquo;, d: &lsquo;1&rsquo;}] and the second generator produced
[{&lsquo;a&rsquo;: &lsquo;override&rsquo;}], the united parameters for merge keys = [&lsquo;a&rsquo;] would be
[{a: &lsquo;override&rsquo;, b: &lsquo;1&rsquo;}, {c: &lsquo;1&rsquo;, d: &lsquo;1&rsquo;}].</p>
<p>MergeGenerator supports template overriding. If a MergeGenerator is one of multiple top-level generators, its
template will be merged with the top-level generator before the parameters are applied.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>generators</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">
[]ApplicationSetNestedGenerator
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>mergeKeys</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.NestedMatrixGenerator">NestedMatrixGenerator
</h3>
<p>
<p>NestedMatrixGenerator is a MatrixGenerator nested under another combination-type generator (MatrixGenerator or
MergeGenerator). NestedMatrixGenerator does not have an override template, because template overriding has no meaning
within the constituent generators of combination-type generators.</p>
<p>NOTE: Nested matrix generator is not included directly in the CRD struct, instead it is included
as a generic &lsquo;apiextensionsv1.JSON&rsquo; object, and then marshalled into a NestedMatrixGenerator
when processed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>generators</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerators">
ApplicationSetTerminalGenerators
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.NestedMergeGenerator">NestedMergeGenerator
</h3>
<p>
<p>NestedMergeGenerator is a MergeGenerator nested under another combination-type generator (MatrixGenerator or
MergeGenerator). NestedMergeGenerator does not have an override template, because template overriding has no meaning
within the constituent generators of combination-type generators.</p>
<p>NOTE: Nested merge generator is not included directly in the CRD struct, instead it is included
as a generic &lsquo;apiextensionsv1.JSON&rsquo; object, and then marshalled into a NestedMergeGenerator
when processed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>generators</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerators">
ApplicationSetTerminalGenerators
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>mergeKeys</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OCIMetadata">OCIMetadata
</h3>
<p>
<p>OCIMetadata contains metadata for a specific revision in an OCI repository</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>createdAt</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>authors</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imageUrl</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>docsUrl</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sourceUrl</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.Operation">Operation
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Application">Application</a>, 
<a href="#argoproj.io/v1alpha1.OperationState">OperationState</a>)
</p>
<p>
<p>Operation contains information about a requested or running operation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sync</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncOperation">
SyncOperation
</a>
</em>
</td>
<td>
<p>Sync contains parameters for the operation</p>
</td>
</tr>
<tr>
<td>
<code>initiatedBy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OperationInitiator">
OperationInitiator
</a>
</em>
</td>
<td>
<p>InitiatedBy contains information about who initiated the operations</p>
</td>
</tr>
<tr>
<td>
<code>info</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.Info">
[]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.Info
</a>
</em>
</td>
<td>
<p>Info is a list of informational items for this operation</p>
</td>
</tr>
<tr>
<td>
<code>retry</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.RetryStrategy">
RetryStrategy
</a>
</em>
</td>
<td>
<p>Retry controls the strategy to apply if a sync fails</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OperationInitiator">OperationInitiator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Operation">Operation</a>, 
<a href="#argoproj.io/v1alpha1.RevisionHistory">RevisionHistory</a>)
</p>
<p>
<p>OperationInitiator contains information about the initiator of an operation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>username</code><br/>
<em>
string
</em>
</td>
<td>
<p>Username contains the name of a user who started operation</p>
</td>
</tr>
<tr>
<td>
<code>automated</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Automated is set to true if operation was initiated automatically by the application controller.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OperationState">OperationState
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>OperationState contains information about state of a running operation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>operation</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Operation">
Operation
</a>
</em>
</td>
<td>
<p>Operation is the original requested operation</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common.OperationPhase
</em>
</td>
<td>
<p>Phase is the current phase of the operation</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message holds any pertinent messages when attempting to perform operation (typically errors).</p>
</td>
</tr>
<tr>
<td>
<code>syncResult</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncOperationResult">
SyncOperationResult
</a>
</em>
</td>
<td>
<p>SyncResult is the result of a Sync operation</p>
</td>
</tr>
<tr>
<td>
<code>startedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>StartedAt contains time of operation start</p>
</td>
</tr>
<tr>
<td>
<code>finishedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>FinishedAt contains time of operation completion</p>
</td>
</tr>
<tr>
<td>
<code>retryCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>RetryCount contains time of operation retries</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OptionalArray">OptionalArray
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourcePluginParameter">ApplicationSourcePluginParameter</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>array</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Array is the value of an array type parameter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OptionalMap">OptionalMap
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSourcePluginParameter">ApplicationSourcePluginParameter</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>map</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Map is the value of a map type parameter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OrphanedResourceKey">OrphanedResourceKey
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.OrphanedResourcesMonitorSettings">OrphanedResourcesMonitorSettings</a>)
</p>
<p>
<p>OrphanedResourceKey is a reference to a resource to be ignored from</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OrphanedResourcesMonitorSettings">OrphanedResourcesMonitorSettings
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>)
</p>
<p>
<p>OrphanedResourcesMonitorSettings holds settings of orphaned resources monitoring</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>warn</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Warn indicates if warning condition should be created for apps which have orphaned resources</p>
</td>
</tr>
<tr>
<td>
<code>ignore</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OrphanedResourceKey">
[]OrphanedResourceKey
</a>
</em>
</td>
<td>
<p>Ignore contains a list of resources that are to be excluded from orphaned resources monitoring</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.OverrideIgnoreDiff">OverrideIgnoreDiff
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceOverride">ResourceOverride</a>)
</p>
<p>
<p>OverrideIgnoreDiff contains configurations about how fields should be ignored during diffs between
the desired state and live state</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>jsonPointers</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>JSONPointers is a JSON path list following the format defined in RFC4627 (<a href="https://datatracker.ietf.org/doc/html/rfc6902#section-3">https://datatracker.ietf.org/doc/html/rfc6902#section-3</a>)</p>
</td>
</tr>
<tr>
<td>
<code>jqPathExpressions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>JQPathExpressions is a JQ path list that will be evaludated during the diff process</p>
</td>
</tr>
<tr>
<td>
<code>managedFieldsManagers</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>ManagedFieldsManagers is a list of trusted managers. Fields mutated by those managers will take precedence over the
desired state defined in the SCM and won&rsquo;t be displayed in diffs</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PluginConfigMapRef">PluginConfigMapRef
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PluginGenerator">PluginGenerator</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the ConfigMap</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PluginGenerator">PluginGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator</a>)
</p>
<p>
<p>PluginGenerator defines connection info specific to Plugin.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>configMapRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PluginConfigMapRef">
PluginConfigMapRef
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>input</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PluginInput">
PluginInput
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>requeueAfterSeconds</code><br/>
<em>
int64
</em>
</td>
<td>
<p>RequeueAfterSeconds determines how long the ApplicationSet controller will wait before reconciling the ApplicationSet again.</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Values contains key/value pairs which are passed directly as parameters to the template. These values will not be
sent as parameters to the plugin.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PluginInput">PluginInput
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PluginGenerator">PluginGenerator</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>parameters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PluginParameters">
PluginParameters
</a>
</em>
</td>
<td>
<p>Parameters contains the information to pass to the plugin. It is a map. The keys must be strings, and the
values can be any type.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PluginParameters">PluginParameters
(<code>map[string]k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PluginInput">PluginInput</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.ProgressiveSyncStatusCode">ProgressiveSyncStatusCode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetApplicationStatus">ApplicationSetApplicationStatus</a>)
</p>
<p>
<p>Represents resource health status</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Healthy&#34;</p></td>
<td><p>Indicates that the application has reached an Healthy state in regards to the requested sync</p>
</td>
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td><p>Indicates that a sync has been trigerred, but the application did not report any status</p>
</td>
</tr><tr><td><p>&#34;Progressing&#34;</p></td>
<td><p>Indicates that the application has not yet reached an Healthy state in regards to the requested sync</p>
</td>
</tr><tr><td><p>&#34;Waiting&#34;</p></td>
<td><p>Indicates that an Application sync is waiting to be trigerred</p>
</td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ProjectRole">ProjectRole
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>)
</p>
<p>
<p>ProjectRole represents a role that has access to a project</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is a name for this role</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<p>Description is a description of the role</p>
</td>
</tr>
<tr>
<td>
<code>policies</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Policies Stores a list of casbin formatted strings that define access policies for the role in the project</p>
</td>
</tr>
<tr>
<td>
<code>jwtTokens</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.JWTToken">
[]JWTToken
</a>
</em>
</td>
<td>
<p>JWTTokens are a list of generated JWT tokens bound to this role</p>
</td>
</tr>
<tr>
<td>
<code>groups</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Groups are a list of OIDC group claims bound to this role</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator</a>)
</p>
<p>
<p>PullRequestGenerator defines a generator that scrapes a PullRequest API to find candidate pull requests.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>github</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorGithub">
PullRequestGeneratorGithub
</a>
</em>
</td>
<td>
<p>Which provider to use and config for it.</p>
</td>
</tr>
<tr>
<td>
<code>gitlab</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorGitLab">
PullRequestGeneratorGitLab
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>gitea</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorGitea">
PullRequestGeneratorGitea
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bitbucketServer</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorBitbucketServer">
PullRequestGeneratorBitbucketServer
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>filters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorFilter">
[]PullRequestGeneratorFilter
</a>
</em>
</td>
<td>
<p>Filters for which pull requests should be considered.</p>
</td>
</tr>
<tr>
<td>
<code>requeueAfterSeconds</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Standard parameters.</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bitbucket</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorBitbucket">
PullRequestGeneratorBitbucket
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>azuredevops</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorAzureDevOps">
PullRequestGeneratorAzureDevOps
</a>
</em>
</td>
<td>
<p>Additional provider to use and config for it.</p>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Values contains key/value pairs which are passed directly as parameters to the template</p>
</td>
</tr>
<tr>
<td>
<code>continueOnRepoNotFoundError</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ContinueOnRepoNotFoundError is a flag to continue the ApplicationSet Pull Request generator parameters generation even if the repository is not found.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGeneratorAzureDevOps">PullRequestGeneratorAzureDevOps
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>)
</p>
<p>
<p>PullRequestGeneratorAzureDevOps defines connection info specific to AzureDevOps.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>organization</code><br/>
<em>
string
</em>
</td>
<td>
<p>Azure DevOps org to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Azure DevOps project name to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>repo</code><br/>
<em>
string
</em>
</td>
<td>
<p>Azure DevOps repo name to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Azure DevOps API URL to talk to. If blank, use <a href="https://dev.azure.com/">https://dev.azure.com/</a>.</p>
</td>
</tr>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Authentication token reference.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Labels is used to filter the PRs that you want to target</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGeneratorBitbucket">PullRequestGeneratorBitbucket
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>)
</p>
<p>
<p>PullRequestGeneratorBitbucket defines connection info specific to Bitbucket.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>owner</code><br/>
<em>
string
</em>
</td>
<td>
<p>Workspace to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>repo</code><br/>
<em>
string
</em>
</td>
<td>
<p>Repo name to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Bitbucket REST API URL to talk to. If blank, uses <a href="https://api.bitbucket.org/2.0">https://api.bitbucket.org/2.0</a>.</p>
</td>
</tr>
<tr>
<td>
<code>basicAuth</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.BasicAuthBitbucketServer">
BasicAuthBitbucketServer
</a>
</em>
</td>
<td>
<p>Credentials for Basic auth</p>
</td>
</tr>
<tr>
<td>
<code>bearerToken</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.BearerTokenBitbucketCloud">
BearerTokenBitbucketCloud
</a>
</em>
</td>
<td>
<p>Credentials for AppToken (Bearer auth)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGeneratorBitbucketServer">PullRequestGeneratorBitbucketServer
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>)
</p>
<p>
<p>PullRequestGeneratorBitbucketServer defines connection info specific to BitbucketServer.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Project to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>repo</code><br/>
<em>
string
</em>
</td>
<td>
<p>Repo name to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Bitbucket REST API URL to talk to e.g. <a href="https://bitbucket.org/rest">https://bitbucket.org/rest</a> Required.</p>
</td>
</tr>
<tr>
<td>
<code>basicAuth</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.BasicAuthBitbucketServer">
BasicAuthBitbucketServer
</a>
</em>
</td>
<td>
<p>Credentials for Basic auth</p>
</td>
</tr>
<tr>
<td>
<code>bearerToken</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.BearerTokenBitbucket">
BearerTokenBitbucket
</a>
</em>
</td>
<td>
<p>Credentials for AccessToken (Bearer auth)</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Allow self-signed TLS / Certificates; default: false</p>
</td>
</tr>
<tr>
<td>
<code>caRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ConfigMapKeyRef">
ConfigMapKeyRef
</a>
</em>
</td>
<td>
<p>ConfigMap key holding the trusted certificates</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGeneratorFilter">PullRequestGeneratorFilter
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>)
</p>
<p>
<p>PullRequestGeneratorFilter is a single pull request filter.
If multiple filter types are set on a single struct, they will be AND&rsquo;d together. All filters must
pass for a pull request to be included.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>branchMatch</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>targetBranchMatch</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>titleMatch</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGeneratorGitLab">PullRequestGeneratorGitLab
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>)
</p>
<p>
<p>PullRequestGeneratorGitLab defines connection info specific to GitLab.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>GitLab project to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The GitLab API URL to talk to. If blank, uses <a href="https://gitlab.com/">https://gitlab.com/</a>.</p>
</td>
</tr>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Authentication token reference.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Labels is used to filter the MRs that you want to target</p>
</td>
</tr>
<tr>
<td>
<code>pullRequestState</code><br/>
<em>
string
</em>
</td>
<td>
<p>PullRequestState is an additional MRs filter to get only those with a certain state. Default: &ldquo;&rdquo; (all states).
Valid values: opened, closed, merged, locked&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Skips validating the SCM provider&rsquo;s TLS certificate - useful for self-signed certificates.; default: false</p>
</td>
</tr>
<tr>
<td>
<code>caRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ConfigMapKeyRef">
ConfigMapKeyRef
</a>
</em>
</td>
<td>
<p>ConfigMap key holding the trusted certificates</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGeneratorGitea">PullRequestGeneratorGitea
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>)
</p>
<p>
<p>PullRequestGeneratorGitea defines connection info specific to Gitea.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>owner</code><br/>
<em>
string
</em>
</td>
<td>
<p>Gitea org or user to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>repo</code><br/>
<em>
string
</em>
</td>
<td>
<p>Gitea repo name to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Gitea API URL to talk to. Required</p>
</td>
</tr>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Authentication token reference.</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Allow insecure tls, for self-signed certificates; default: false.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Labels is used to filter the PRs that you want to target</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.PullRequestGeneratorGithub">PullRequestGeneratorGithub
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.PullRequestGenerator">PullRequestGenerator</a>)
</p>
<p>
<p>PullRequestGeneratorGithub defines connection info specific to GitHub.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>owner</code><br/>
<em>
string
</em>
</td>
<td>
<p>GitHub org or user to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>repo</code><br/>
<em>
string
</em>
</td>
<td>
<p>GitHub repo name to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The GitHub API URL to talk to. If blank, use <a href="https://api.github.com/">https://api.github.com/</a>.</p>
</td>
</tr>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Authentication token reference.</p>
</td>
</tr>
<tr>
<td>
<code>appSecretName</code><br/>
<em>
string
</em>
</td>
<td>
<p>AppSecretName is a reference to a GitHub App repo-creds secret with permission to access pull requests.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Labels is used to filter the PRs that you want to target</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RefTarget">RefTarget
</h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Repo</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Repository">
Repository
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>TargetRevision</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>Chart</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RefTargetRevisionMapping">RefTargetRevisionMapping
(<code>map[string]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.RefTarget</code> alias)</p></h3>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.RefreshType">RefreshType
(<code>string</code> alias)</p></h3>
<p>
<p>RefreshType specifies how to refresh the sources of a given application</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;hard&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;normal&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RepoCreds">RepoCreds
</h3>
<p>
<p>RepoCreds holds the definition for repository credentials</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>url</code><br/>
<em>
string
</em>
</td>
<td>
<p>URL is the URL to which these credentials match</p>
</td>
</tr>
<tr>
<td>
<code>username</code><br/>
<em>
string
</em>
</td>
<td>
<p>Username for authenticating at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
string
</em>
</td>
<td>
<p>Password for authenticating at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>sshPrivateKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>SSHPrivateKey contains the private key data for authenticating at the repo server using SSH (only Git repos)</p>
</td>
</tr>
<tr>
<td>
<code>tlsClientCertData</code><br/>
<em>
string
</em>
</td>
<td>
<p>TLSClientCertData specifies the TLS client cert data for authenticating at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>tlsClientCertKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>TLSClientCertKey specifies the TLS client cert key for authenticating at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>githubAppPrivateKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>GithubAppPrivateKey specifies the private key PEM data for authentication via GitHub app</p>
</td>
</tr>
<tr>
<td>
<code>githubAppID</code><br/>
<em>
int64
</em>
</td>
<td>
<p>GithubAppId specifies the Github App ID of the app used to access the repo for GitHub app authentication</p>
</td>
</tr>
<tr>
<td>
<code>githubAppInstallationID</code><br/>
<em>
int64
</em>
</td>
<td>
<p>GithubAppInstallationId specifies the ID of the installed GitHub App for GitHub app authentication</p>
</td>
</tr>
<tr>
<td>
<code>githubAppEnterpriseBaseUrl</code><br/>
<em>
string
</em>
</td>
<td>
<p>GithubAppEnterpriseBaseURL specifies the GitHub API URL for GitHub app authentication. If empty will default to <a href="https://api.github.com">https://api.github.com</a></p>
</td>
</tr>
<tr>
<td>
<code>enableOCI</code><br/>
<em>
bool
</em>
</td>
<td>
<p>EnableOCI specifies whether helm-oci support should be enabled for this repo</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>Type specifies the type of the repoCreds. Can be either &ldquo;git&rdquo;, &ldquo;helm&rdquo; or &ldquo;oci&rdquo;. &ldquo;git&rdquo; is assumed if empty or absent.</p>
</td>
</tr>
<tr>
<td>
<code>gcpServiceAccountKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>GCPServiceAccountKey specifies the service account key in JSON format to be used for getting credentials to Google Cloud Source repos</p>
</td>
</tr>
<tr>
<td>
<code>proxy</code><br/>
<em>
string
</em>
</td>
<td>
<p>Proxy specifies the HTTP/HTTPS proxy used to access repos at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>forceHttpBasicAuth</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ForceHttpBasicAuth specifies whether Argo CD should attempt to force basic auth for HTTP connections</p>
</td>
</tr>
<tr>
<td>
<code>noProxy</code><br/>
<em>
string
</em>
</td>
<td>
<p>NoProxy specifies a list of targets where the proxy isn&rsquo;t used, applies only in cases where the proxy is applied</p>
</td>
</tr>
<tr>
<td>
<code>useAzureWorkloadIdentity</code><br/>
<em>
bool
</em>
</td>
<td>
<p>UseAzureWorkloadIdentity specifies whether to use Azure Workload Identity for authentication</p>
</td>
</tr>
<tr>
<td>
<code>bearerToken</code><br/>
<em>
string
</em>
</td>
<td>
<p>BearerToken contains the bearer token used for Git BitBucket Data Center auth at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>insecureOCIForceHttp</code><br/>
<em>
bool
</em>
</td>
<td>
<p>InsecureOCIForceHttp specifies whether the connection to the repository uses TLS at <em>all</em>. If true, no TLS. This flag is applicable for OCI repos only.</p>
</td>
</tr>
<tr>
<td>
<code>azureServicePrincipalClientId</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureServicePrincipalClientId specifies the client ID of the Azure Service Principal used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>azureServicePrincipalClientSecret</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureServicePrincipalClientSecret specifies the client secret of the Azure Service Principal used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>azureServicePrincipalTenantId</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureServicePrincipalTenantId specifies the tenant ID of the Azure Service Principal used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>azureActiveDirectoryEndpoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureActiveDirectoryEndpoint specifies the Azure Active Directory endpoint used for Service Principal authentication. If empty will default to <a href="https://login.microsoftonline.com">https://login.microsoftonline.com</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.Repositories">Repositories
(<code>[]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.Repository</code> alias)</p></h3>
<p>
<p>Repositories defines a list of Repository configurations</p>
</p>
<h3 id="argoproj.io/v1alpha1.Repository">Repository
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.RefTarget">RefTarget</a>)
</p>
<p>
<p>Repository is a repository holding application configurations</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repo</code><br/>
<em>
string
</em>
</td>
<td>
<p>Repo contains the URL to the remote repository</p>
</td>
</tr>
<tr>
<td>
<code>username</code><br/>
<em>
string
</em>
</td>
<td>
<p>Username contains the user name used for authenticating at the remote repository</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
string
</em>
</td>
<td>
<p>Password contains the password or PAT used for authenticating at the remote repository</p>
</td>
</tr>
<tr>
<td>
<code>sshPrivateKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>SSHPrivateKey contains the PEM data for authenticating at the repo server. Only used with Git repos.</p>
</td>
</tr>
<tr>
<td>
<code>connectionState</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ConnectionState">
ConnectionState
</a>
</em>
</td>
<td>
<p>ConnectionState contains information about the current state of connection to the repository server</p>
</td>
</tr>
<tr>
<td>
<code>insecureIgnoreHostKey</code><br/>
<em>
bool
</em>
</td>
<td>
<p>InsecureIgnoreHostKey should not be used anymore, Insecure is favoured
Used only for Git repos</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Insecure specifies whether the connection to the repository ignores any errors when verifying TLS certificates or SSH host keys</p>
</td>
</tr>
<tr>
<td>
<code>enableLfs</code><br/>
<em>
bool
</em>
</td>
<td>
<p>EnableLFS specifies whether git-lfs support should be enabled for this repo. Only valid for Git repositories.</p>
</td>
</tr>
<tr>
<td>
<code>tlsClientCertData</code><br/>
<em>
string
</em>
</td>
<td>
<p>TLSClientCertData contains a certificate in PEM format for authenticating at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>tlsClientCertKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>TLSClientCertKey contains a private key in PEM format for authenticating at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>Type specifies the type of the repo. Can be either &ldquo;git&rdquo; or &ldquo;helm. &ldquo;git&rdquo; is assumed if empty or absent.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name specifies a name to be used for this repo. Only used with Helm repos</p>
</td>
</tr>
<tr>
<td>
<code>inheritedCreds</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Whether credentials were inherited from a credential set</p>
</td>
</tr>
<tr>
<td>
<code>enableOCI</code><br/>
<em>
bool
</em>
</td>
<td>
<p>EnableOCI specifies whether helm-oci support should be enabled for this repo</p>
</td>
</tr>
<tr>
<td>
<code>githubAppPrivateKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>Github App Private Key PEM data</p>
</td>
</tr>
<tr>
<td>
<code>githubAppID</code><br/>
<em>
int64
</em>
</td>
<td>
<p>GithubAppId specifies the ID of the GitHub app used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>githubAppInstallationID</code><br/>
<em>
int64
</em>
</td>
<td>
<p>GithubAppInstallationId specifies the installation ID of the GitHub App used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>githubAppEnterpriseBaseUrl</code><br/>
<em>
string
</em>
</td>
<td>
<p>GithubAppEnterpriseBaseURL specifies the base URL of GitHub Enterprise installation. If empty will default to <a href="https://api.github.com">https://api.github.com</a></p>
</td>
</tr>
<tr>
<td>
<code>proxy</code><br/>
<em>
string
</em>
</td>
<td>
<p>Proxy specifies the HTTP/HTTPS proxy used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Reference between project and repository that allows it to be automatically added as an item inside SourceRepos project entity</p>
</td>
</tr>
<tr>
<td>
<code>gcpServiceAccountKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>GCPServiceAccountKey specifies the service account key in JSON format to be used for getting credentials to Google Cloud Source repos</p>
</td>
</tr>
<tr>
<td>
<code>forceHttpBasicAuth</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ForceHttpBasicAuth specifies whether Argo CD should attempt to force basic auth for HTTP connections</p>
</td>
</tr>
<tr>
<td>
<code>noProxy</code><br/>
<em>
string
</em>
</td>
<td>
<p>NoProxy specifies a list of targets where the proxy isn&rsquo;t used, applies only in cases where the proxy is applied</p>
</td>
</tr>
<tr>
<td>
<code>useAzureWorkloadIdentity</code><br/>
<em>
bool
</em>
</td>
<td>
<p>UseAzureWorkloadIdentity specifies whether to use Azure Workload Identity for authentication</p>
</td>
</tr>
<tr>
<td>
<code>bearerToken</code><br/>
<em>
string
</em>
</td>
<td>
<p>BearerToken contains the bearer token used for Git BitBucket Data Center auth at the repo server</p>
</td>
</tr>
<tr>
<td>
<code>insecureOCIForceHttp</code><br/>
<em>
bool
</em>
</td>
<td>
<p>InsecureOCIForceHttp specifies whether the connection to the repository uses TLS at <em>all</em>. If true, no TLS. This flag is applicable for OCI repos only.</p>
</td>
</tr>
<tr>
<td>
<code>depth</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Depth specifies the depth for shallow clones. A value of 0 or omitting the field indicates a full clone.</p>
</td>
</tr>
<tr>
<td>
<code>webhookManifestCacheWarmDisabled</code><br/>
<em>
bool
</em>
</td>
<td>
<p>WebhookManifestCacheWarmDisabled disables manifest cache warming during webhook processing for this repository.
When set, webhook handlers will only trigger reconciliation for affected applications and skip Redis cache
operations for unaffected ones. Recommended for large monorepos with plain YAML manifests.</p>
</td>
</tr>
<tr>
<td>
<code>azureServicePrincipalClientId</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureServicePrincipalClientId specifies the client ID of the Azure Service Principal used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>azureServicePrincipalClientSecret</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureServicePrincipalClientSecret specifies the client secret of the Azure Service Principal used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>azureServicePrincipalTenantId</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureServicePrincipalTenantId specifies the tenant ID of the Azure Service Principal used to access the repo</p>
</td>
</tr>
<tr>
<td>
<code>azureActiveDirectoryEndpoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>AzureActiveDirectoryEndpoint specifies the Azure Active Directory endpoint used for Service Principal authentication. If empty will default to <a href="https://login.microsoftonline.com">https://login.microsoftonline.com</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RepositoryCertificate">RepositoryCertificate
</h3>
<p>
<p>A RepositoryCertificate is either SSH known hosts entry or TLS certificate</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>serverName</code><br/>
<em>
string
</em>
</td>
<td>
<p>ServerName specifies the DNS name of the server this certificate is intended for</p>
</td>
</tr>
<tr>
<td>
<code>certType</code><br/>
<em>
string
</em>
</td>
<td>
<p>CertType specifies the type of the certificate - currently one of &ldquo;https&rdquo; or &ldquo;ssh&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>certSubType</code><br/>
<em>
string
</em>
</td>
<td>
<p>CertSubType specifies the sub type of the cert, i.e. &ldquo;ssh-rsa&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>certData</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>CertData contains the actual certificate data, dependent on the certificate type</p>
</td>
</tr>
<tr>
<td>
<code>certInfo</code><br/>
<em>
string
</em>
</td>
<td>
<p>CertInfo will hold additional certificate info, depdendent on the certificate type (e.g. SSH fingerprint, X509 CommonName)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceAction">ResourceAction
</h3>
<p>
<p>ResourceAction represents an individual action that can be performed on a resource.
It includes parameters, an optional disabled flag, an icon for display, and a name for the action.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name or identifier for the action.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceActionParam">
[]ResourceActionParam
</a>
</em>
</td>
<td>
<p>Params contains the parameters required to execute the action.</p>
</td>
</tr>
<tr>
<td>
<code>disabled</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Disabled indicates whether the action is disabled.</p>
</td>
</tr>
<tr>
<td>
<code>iconClass</code><br/>
<em>
string
</em>
</td>
<td>
<p>IconClass specifies the CSS class for the action&rsquo;s icon.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<p>DisplayName provides a user-friendly name for the action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceActionDefinition">ResourceActionDefinition
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceActions">ResourceActions</a>)
</p>
<p>
<p>ResourceActionDefinition defines an individual action that can be executed on a resource.
It includes a name for the action and a Lua script that defines the action&rsquo;s behavior.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the identifier for the action.</p>
</td>
</tr>
<tr>
<td>
<code>action.lua</code><br/>
<em>
string
</em>
</td>
<td>
<p>ActionLua contains the Lua script that defines the behavior of the action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceActionParam">ResourceActionParam
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceAction">ResourceAction</a>)
</p>
<p>
<p>ResourceActionParam represents a parameter for a resource action.
It includes a name, value, type, and an optional default value for the parameter.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the parameter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceActions">ResourceActions
</h3>
<p>
<p>ResourceActions holds the set of actions that can be applied to a resource.
It defines custom Lua scripts for discovery and action execution, as well as options
for merging built-in actions with custom ones.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>discovery.lua</code><br/>
<em>
string
</em>
</td>
<td>
<p>ActionDiscoveryLua contains a Lua script for discovering actions.</p>
</td>
</tr>
<tr>
<td>
<code>definitions</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceActionDefinition">
[]ResourceActionDefinition
</a>
</em>
</td>
<td>
<p>Definitions holds the list of action definitions available for the resource.</p>
</td>
</tr>
<tr>
<td>
<code>mergeBuiltinActions</code><br/>
<em>
bool
</em>
</td>
<td>
<p>MergeBuiltinActions indicates whether built-in actions should be merged with custom actions.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceDiff">ResourceDiff
</h3>
<p>
<p>ResourceDiff holds the diff between a live and target resource object in Argo CD.
It is used to compare the desired state (from Git/Helm) with the actual state in the cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
<p>Group represents the API group of the resource (e.g., &ldquo;apps&rdquo; for Deployments).</p>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
<p>Kind represents the Kubernetes resource kind (e.g., &ldquo;Deployment&rdquo;, &ldquo;Service&rdquo;).</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace specifies the namespace where the resource exists.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the resource.</p>
</td>
</tr>
<tr>
<td>
<code>targetState</code><br/>
<em>
string
</em>
</td>
<td>
<p>TargetState contains the JSON-serialized resource manifest as defined in the Git/Helm repository.</p>
</td>
</tr>
<tr>
<td>
<code>liveState</code><br/>
<em>
string
</em>
</td>
<td>
<p>LiveState contains the JSON-serialized resource manifest of the resource currently running in the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>diff</code><br/>
<em>
string
</em>
</td>
<td>
<p>Diff contains the JSON patch representing the difference between the live and target resource.</p>
<p>Deprecated: Use NormalizedLiveState and PredictedLiveState instead to compute differences.</p>
</td>
</tr>
<tr>
<td>
<code>hook</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Hook indicates whether this resource is a hook resource (e.g., pre-sync or post-sync hooks).</p>
</td>
</tr>
<tr>
<td>
<code>normalizedLiveState</code><br/>
<em>
string
</em>
</td>
<td>
<p>NormalizedLiveState contains the JSON-serialized live resource state after applying normalizations.
Normalizations may include ignoring irrelevant fields like timestamps or defaults applied by Kubernetes.</p>
</td>
</tr>
<tr>
<td>
<code>predictedLiveState</code><br/>
<em>
string
</em>
</td>
<td>
<p>PredictedLiveState contains the JSON-serialized resource state that Argo CD predicts based on the
combination of the normalized live state and the desired target state.</p>
</td>
</tr>
<tr>
<td>
<code>resourceVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>ResourceVersion is the Kubernetes resource version, which helps in tracking changes.</p>
</td>
</tr>
<tr>
<td>
<code>modified</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Modified indicates whether the live resource has changes compared to the target resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceHealthLocation">ResourceHealthLocation
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.ResourceIgnoreDifferences">ResourceIgnoreDifferences
</h3>
<p>
<p>ResourceIgnoreDifferences contains resource filter and list of json paths which should be ignored during comparison with live state.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>jsonPointers</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>jqPathExpressions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedFieldsManagers</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>ManagedFieldsManagers is a list of trusted managers. Fields mutated by those managers will take precedence over the
desired state defined in the SCM and won&rsquo;t be displayed in diffs</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceNetworkingInfo">ResourceNetworkingInfo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceNode">ResourceNode</a>)
</p>
<p>
<p>ResourceNetworkingInfo holds networking-related information for a resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>targetLabels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>TargetLabels represents labels associated with the target resources that this resource communicates with.</p>
</td>
</tr>
<tr>
<td>
<code>targetRefs</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceRef">
[]ResourceRef
</a>
</em>
</td>
<td>
<p>TargetRefs contains references to other resources that this resource interacts with, such as Services or Pods.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Labels holds the labels associated with this networking resource.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code><br/>
<em>
[]k8s.io/api/core/v1.LoadBalancerIngress
</em>
</td>
<td>
<p>Ingress provides information about external access points (e.g., load balancer ingress) for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>externalURLs</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>ExternalURLs holds a list of URLs that should be accessible externally.
This field is typically populated for Ingress resources based on their hostname rules.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceNode">ResourceNode
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTree">ApplicationSetTree</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationTree">ApplicationTree</a>)
</p>
<p>
<p>ResourceNode contains information about a live Kubernetes resource and its relationships with other resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ResourceRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceRef">
ResourceRef
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceRef</code> are embedded into this type.)
</p>
<p>ResourceRef uniquely identifies the resource using its group, kind, namespace, and name.</p>
</td>
</tr>
<tr>
<td>
<code>parentRefs</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceRef">
[]ResourceRef
</a>
</em>
</td>
<td>
<p>ParentRefs lists the parent resources that reference this resource.
This helps in understanding ownership and hierarchical relationships.</p>
</td>
</tr>
<tr>
<td>
<code>info</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.InfoItem">
[]InfoItem
</a>
</em>
</td>
<td>
<p>Info provides additional metadata or annotations about the resource.</p>
</td>
</tr>
<tr>
<td>
<code>networkingInfo</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceNetworkingInfo">
ResourceNetworkingInfo
</a>
</em>
</td>
<td>
<p>NetworkingInfo contains details about the resource&rsquo;s networking attributes,
such as ingress information and external URLs.</p>
</td>
</tr>
<tr>
<td>
<code>resourceVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>ResourceVersion indicates the version of the resource, used to track changes.</p>
</td>
</tr>
<tr>
<td>
<code>images</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Images lists container images associated with the resource.
This is primarily useful for pods and other workload resources.</p>
</td>
</tr>
<tr>
<td>
<code>health</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HealthStatus">
HealthStatus
</a>
</em>
</td>
<td>
<p>Health represents the health status of the resource (e.g., Healthy, Degraded, Progressing).</p>
</td>
</tr>
<tr>
<td>
<code>createdAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>CreatedAt records the timestamp when the resource was created.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceOverride">ResourceOverride
</h3>
<p>
<p>ResourceOverride holds configuration to customize resource diffing and health assessment</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>HealthLua</code><br/>
<em>
string
</em>
</td>
<td>
<p>HealthLua contains a Lua script that defines custom health checks for the resource.</p>
</td>
</tr>
<tr>
<td>
<code>UseOpenLibs</code><br/>
<em>
bool
</em>
</td>
<td>
<p>UseOpenLibs indicates whether to use open-source libraries for the resource.</p>
</td>
</tr>
<tr>
<td>
<code>Actions</code><br/>
<em>
string
</em>
</td>
<td>
<p>Actions defines the set of actions that can be performed on the resource, as a Lua script.</p>
</td>
</tr>
<tr>
<td>
<code>IgnoreDifferences</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OverrideIgnoreDiff">
OverrideIgnoreDiff
</a>
</em>
</td>
<td>
<p>IgnoreDifferences contains configuration for which differences should be ignored during the resource diffing.</p>
</td>
</tr>
<tr>
<td>
<code>IgnoreResourceUpdates</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OverrideIgnoreDiff">
OverrideIgnoreDiff
</a>
</em>
</td>
<td>
<p>IgnoreResourceUpdates holds configuration for ignoring updates to specific resource fields.</p>
</td>
</tr>
<tr>
<td>
<code>KnownTypeFields</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.KnownTypeField">
[]KnownTypeField
</a>
</em>
</td>
<td>
<p>KnownTypeFields lists fields for which unit conversions should be applied.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceRef">ResourceRef
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceNetworkingInfo">ResourceNetworkingInfo</a>, 
<a href="#argoproj.io/v1alpha1.ResourceNode">ResourceNode</a>)
</p>
<p>
<p>ResourceRef includes fields which uniquely identify a resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>uid</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceResult">ResourceResult
</h3>
<p>
<p>ResourceResult holds the operation result details of a specific resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
<p>Group specifies the API group of the resource</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<p>Version specifies the API version of the resource</p>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
<p>Kind specifies the API kind of the resource</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace specifies the target namespace of the resource</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name specifies the name of the resource</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common.ResultCode
</em>
</td>
<td>
<p>Status holds the final result of the sync. Will be empty if the resources is yet to be applied/pruned and is always zero-value for hooks</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message contains an informational or error message for the last sync OR operation</p>
</td>
</tr>
<tr>
<td>
<code>hookType</code><br/>
<em>
github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common.HookType
</em>
</td>
<td>
<p>HookType specifies the type of the hook. Empty for non-hook resources</p>
</td>
</tr>
<tr>
<td>
<code>hookPhase</code><br/>
<em>
github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common.OperationPhase
</em>
</td>
<td>
<p>HookPhase contains the state of any operation associated with this resource OR hook
This can also contain values for non-hook resources.</p>
</td>
</tr>
<tr>
<td>
<code>syncPhase</code><br/>
<em>
github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common.SyncPhase
</em>
</td>
<td>
<p>SyncPhase indicates the particular phase of the sync that this result was acquired in</p>
</td>
</tr>
<tr>
<td>
<code>images</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Images contains the images related to the ResourceResult</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ResourceResults">ResourceResults
(<code>[]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.ResourceResult</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncOperationResult">SyncOperationResult</a>)
</p>
<p>
<p>ResourceResults defines a list of resource results for a given operation</p>
</p>
<h3 id="argoproj.io/v1alpha1.ResourceStatus">ResourceStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetStatus">ApplicationSetStatus</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>ResourceStatus holds the current synchronization and health status of a Kubernetes resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
<p>Group represents the API group of the resource (e.g., &ldquo;apps&rdquo; for Deployments).</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<p>Version indicates the API version of the resource (e.g., &ldquo;v1&rdquo;, &ldquo;v1beta1&rdquo;).</p>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
<p>Kind specifies the type of the resource (e.g., &ldquo;Deployment&rdquo;, &ldquo;Service&rdquo;).</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace defines the Kubernetes namespace where the resource is located.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the unique name of the resource within the namespace.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncStatusCode">
SyncStatusCode
</a>
</em>
</td>
<td>
<p>Status represents the synchronization state of the resource (e.g., Synced, OutOfSync).</p>
</td>
</tr>
<tr>
<td>
<code>health</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HealthStatus">
HealthStatus
</a>
</em>
</td>
<td>
<p>Health indicates the health status of the resource (e.g., Healthy, Degraded, Progressing).</p>
</td>
</tr>
<tr>
<td>
<code>hook</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Hook is true if the resource is used as a lifecycle hook in an Argo CD application.</p>
</td>
</tr>
<tr>
<td>
<code>requiresPruning</code><br/>
<em>
bool
</em>
</td>
<td>
<p>RequiresPruning is true if the resource needs to be pruned (deleted) as part of synchronization.</p>
</td>
</tr>
<tr>
<td>
<code>syncWave</code><br/>
<em>
int64
</em>
</td>
<td>
<p>SyncWave determines the order in which resources are applied during a sync operation.
Lower values are applied first.</p>
</td>
</tr>
<tr>
<td>
<code>requiresDeletionConfirmation</code><br/>
<em>
bool
</em>
</td>
<td>
<p>RequiresDeletionConfirmation is true if the resource requires explicit user confirmation before deletion.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RetryStrategy">RetryStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Operation">Operation</a>, 
<a href="#argoproj.io/v1alpha1.SyncPolicy">SyncPolicy</a>)
</p>
<p>
<p>RetryStrategy contains information about the strategy to apply when a sync failed</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>limit</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Limit is the maximum number of attempts for retrying a failed sync. If set to 0, no retries will be performed.</p>
</td>
</tr>
<tr>
<td>
<code>backoff</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.Backoff">
Backoff
</a>
</em>
</td>
<td>
<p>Backoff controls how to backoff on subsequent retries of failed syncs</p>
</td>
</tr>
<tr>
<td>
<code>refresh</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Refresh indicates if the latest revision should be used on retry instead of the initial one (default: false)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RevisionHistories">RevisionHistories
(<code>[]github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.RevisionHistory</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>RevisionHistories is a array of history, oldest first and newest last</p>
</p>
<h3 id="argoproj.io/v1alpha1.RevisionHistory">RevisionHistory
</h3>
<p>
<p>RevisionHistory contains history information about a previous sync</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>revision</code><br/>
<em>
string
</em>
</td>
<td>
<p>Revision holds the revision the sync was performed against</p>
</td>
</tr>
<tr>
<td>
<code>deployedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>DeployedAt holds the time the sync operation completed</p>
</td>
</tr>
<tr>
<td>
<code>id</code><br/>
<em>
int64
</em>
</td>
<td>
<p>ID is an auto incrementing identifier of the RevisionHistory</p>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">
ApplicationSource
</a>
</em>
</td>
<td>
<p>Source is a reference to the application source used for the sync operation</p>
</td>
</tr>
<tr>
<td>
<code>deployStartedAt</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>DeployStartedAt holds the time the sync operation started</p>
</td>
</tr>
<tr>
<td>
<code>sources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSources">
ApplicationSources
</a>
</em>
</td>
<td>
<p>Sources is a reference to the application sources used for the sync operation</p>
</td>
</tr>
<tr>
<td>
<code>revisions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Revisions holds the revision of each source in sources field the sync was performed against</p>
</td>
</tr>
<tr>
<td>
<code>initiatedBy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.OperationInitiator">
OperationInitiator
</a>
</em>
</td>
<td>
<p>InitiatedBy contains information about who initiated the operations</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RevisionMetadata">RevisionMetadata
</h3>
<p>
<p>RevisionMetadata contains metadata for a specific revision in a Git repository. This field is used by the
Source Hydrator feature which may be removed in the future.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>author</code><br/>
<em>
string
</em>
</td>
<td>
<p>who authored this revision,
typically their name and email, e.g. &ldquo;John Doe <john_doe@my-company.com>&rdquo;,
but might not match this example</p>
</td>
</tr>
<tr>
<td>
<code>date</code><br/>
<em>
k8s.io/apimachinery/pkg/apis/meta/v1.Time
</em>
</td>
<td>
<p>Date specifies when the revision was authored</p>
</td>
</tr>
<tr>
<td>
<code>tags</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Tags specifies any tags currently attached to the revision
Floating tags can move from one revision to another</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Message contains the message associated with the revision, most likely the commit message.</p>
</td>
</tr>
<tr>
<td>
<code>signatureInfo</code><br/>
<em>
string
</em>
</td>
<td>
<p>SignatureInfo contains a hint on the signer if the revision was signed with GPG, and signature verification is enabled.</p>
<p>Deprecated: Use SourceIntegrityResult for more detailed information. SignatureInfo will be removed with the next major version.</p>
</td>
</tr>
<tr>
<td>
<code>references</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.RevisionReference">
[]RevisionReference
</a>
</em>
</td>
<td>
<p>References contains references to information that&rsquo;s related to this commit in some way.</p>
</td>
</tr>
<tr>
<td>
<code>sourceIntegrityResult</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityCheckResult">
SourceIntegrityCheckResult
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.RevisionReference">RevisionReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.RevisionMetadata">RevisionMetadata</a>)
</p>
<p>
<p>RevisionReference contains a reference to a some information that is related in some way to another commit. For now,
it supports only references to a commit. In the future, it may support other types of references.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>commit</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.CommitMetadata">
CommitMetadata
</a>
</em>
</td>
<td>
<p>Commit contains metadata about the commit that is related in some way to another commit.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSetGenerator">ApplicationSetGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetNestedGenerator">ApplicationSetNestedGenerator</a>, 
<a href="#argoproj.io/v1alpha1.ApplicationSetTerminalGenerator">ApplicationSetTerminalGenerator</a>)
</p>
<p>
<p>SCMProviderGenerator defines a generator that scrapes a SCMaaS API to find candidate repos.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>github</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorGithub">
SCMProviderGeneratorGithub
</a>
</em>
</td>
<td>
<p>Which provider to use and config for it.</p>
</td>
</tr>
<tr>
<td>
<code>gitlab</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorGitlab">
SCMProviderGeneratorGitlab
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bitbucket</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorBitbucket">
SCMProviderGeneratorBitbucket
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bitbucketServer</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorBitbucketServer">
SCMProviderGeneratorBitbucketServer
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>gitea</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorGitea">
SCMProviderGeneratorGitea
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>azureDevOps</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorAzureDevOps">
SCMProviderGeneratorAzureDevOps
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>filters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorFilter">
[]SCMProviderGeneratorFilter
</a>
</em>
</td>
<td>
<p>Filters for which repos should be considered.</p>
</td>
</tr>
<tr>
<td>
<code>cloneProtocol</code><br/>
<em>
string
</em>
</td>
<td>
<p>Which protocol to use for the SCM URL. Default is provider-specific but ssh if possible. Not all providers
necessarily support all protocols.</p>
</td>
</tr>
<tr>
<td>
<code>requeueAfterSeconds</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Standard parameters.</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSetTemplate">
ApplicationSetTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Values contains key/value pairs which are passed directly as parameters to the template</p>
</td>
</tr>
<tr>
<td>
<code>awsCodeCommit</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorAWSCodeCommit">
SCMProviderGeneratorAWSCodeCommit
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorAWSCodeCommit">SCMProviderGeneratorAWSCodeCommit
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorAWSCodeCommit defines connection info specific to AWS CodeCommit.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tagFilters</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.TagFilter">
[]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.TagFilter
</a>
</em>
</td>
<td>
<p>TagFilters provides the tag filter(s) for repo discovery</p>
</td>
</tr>
<tr>
<td>
<code>role</code><br/>
<em>
string
</em>
</td>
<td>
<p>Role provides the AWS IAM role to assume, for cross-account repo discovery
if not provided, AppSet controller will use its pod/node identity to discover.</p>
</td>
</tr>
<tr>
<td>
<code>region</code><br/>
<em>
string
</em>
</td>
<td>
<p>Region provides the AWS region to discover repos.
if not provided, AppSet controller will infer the current region from environment.</p>
</td>
</tr>
<tr>
<td>
<code>allBranches</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Scan all branches instead of just the default branch.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorAzureDevOps">SCMProviderGeneratorAzureDevOps
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorAzureDevOps defines connection info specific to Azure DevOps.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>organization</code><br/>
<em>
string
</em>
</td>
<td>
<p>Azure Devops organization. Required. E.g. &ldquo;my-organization&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The URL to Azure DevOps. If blank, use <a href="https://dev.azure.com">https://dev.azure.com</a>.</p>
</td>
</tr>
<tr>
<td>
<code>teamProject</code><br/>
<em>
string
</em>
</td>
<td>
<p>Azure Devops team project. Required. E.g. &ldquo;my-team&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>accessTokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>The Personal Access Token (PAT) to use when connecting. Required.</p>
</td>
</tr>
<tr>
<td>
<code>allBranches</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Scan all branches instead of just the default branch.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorBitbucket">SCMProviderGeneratorBitbucket
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorBitbucket defines connection info specific to Bitbucket Cloud (API version 2).</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>owner</code><br/>
<em>
string
</em>
</td>
<td>
<p>Bitbucket workspace to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>user</code><br/>
<em>
string
</em>
</td>
<td>
<p>Bitbucket user to use when authenticating.  Should have a &ldquo;member&rdquo; role to be able to read all repositories and branches.  Required</p>
</td>
</tr>
<tr>
<td>
<code>appPasswordRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>The app password to use for the user.  Required. See: <a href="https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/">https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/</a></p>
</td>
</tr>
<tr>
<td>
<code>allBranches</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Scan all branches instead of just the main branch.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorBitbucketServer">SCMProviderGeneratorBitbucketServer
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorBitbucketServer defines connection info specific to Bitbucket Server.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>project</code><br/>
<em>
string
</em>
</td>
<td>
<p>Project to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Bitbucket Server REST API URL to talk to. Required.</p>
</td>
</tr>
<tr>
<td>
<code>basicAuth</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.BasicAuthBitbucketServer">
BasicAuthBitbucketServer
</a>
</em>
</td>
<td>
<p>Credentials for Basic auth</p>
</td>
</tr>
<tr>
<td>
<code>allBranches</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Scan all branches instead of just the default branch.</p>
</td>
</tr>
<tr>
<td>
<code>bearerToken</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.BearerTokenBitbucket">
BearerTokenBitbucket
</a>
</em>
</td>
<td>
<p>Credentials for AccessToken (Bearer auth)</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Allow self-signed TLS / Certificates; default: false</p>
</td>
</tr>
<tr>
<td>
<code>caRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ConfigMapKeyRef">
ConfigMapKeyRef
</a>
</em>
</td>
<td>
<p>ConfigMap key holding the trusted certificates</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorFilter">SCMProviderGeneratorFilter
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorFilter is a single repository filter.
If multiple filter types are set on a single struct, they will be AND&rsquo;d together. All filters must
pass for a repo to be included.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repositoryMatch</code><br/>
<em>
string
</em>
</td>
<td>
<p>A regex for repo names.</p>
</td>
</tr>
<tr>
<td>
<code>pathsExist</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>An array of paths, all of which must exist.</p>
</td>
</tr>
<tr>
<td>
<code>pathsDoNotExist</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>An array of paths, all of which must not exist.</p>
</td>
</tr>
<tr>
<td>
<code>labelMatch</code><br/>
<em>
string
</em>
</td>
<td>
<p>A regex which must match at least one label.</p>
</td>
</tr>
<tr>
<td>
<code>branchMatch</code><br/>
<em>
string
</em>
</td>
<td>
<p>A regex which must match the branch name.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorGitea">SCMProviderGeneratorGitea
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorGitea defines a connection info specific to Gitea.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>owner</code><br/>
<em>
string
</em>
</td>
<td>
<p>Gitea organization or user to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Gitea URL to talk to. For example <a href="https://gitea.mydomain.com/">https://gitea.mydomain.com/</a>.</p>
</td>
</tr>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Authentication token reference.</p>
</td>
</tr>
<tr>
<td>
<code>allBranches</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Scan all branches instead of just the default branch.</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Allow self-signed TLS / Certificates; default: false</p>
</td>
</tr>
<tr>
<td>
<code>excludeArchivedRepos</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Exclude repositories that are archived.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorGithub">SCMProviderGeneratorGithub
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorGithub defines connection info specific to GitHub.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>organization</code><br/>
<em>
string
</em>
</td>
<td>
<p>GitHub org to scan. Required.</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The GitHub API URL to talk to. If blank, use <a href="https://api.github.com/">https://api.github.com/</a>.</p>
</td>
</tr>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Authentication token reference.</p>
</td>
</tr>
<tr>
<td>
<code>appSecretName</code><br/>
<em>
string
</em>
</td>
<td>
<p>AppSecretName is a reference to a GitHub App repo-creds secret.</p>
</td>
</tr>
<tr>
<td>
<code>allBranches</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Scan all branches instead of just the default branch.</p>
</td>
</tr>
<tr>
<td>
<code>excludeArchivedRepos</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Exclude repositories that are archived.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SCMProviderGeneratorGitlab">SCMProviderGeneratorGitlab
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SCMProviderGenerator">SCMProviderGenerator</a>)
</p>
<p>
<p>SCMProviderGeneratorGitlab defines connection info specific to Gitlab.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
<p>Gitlab group to scan. Required.  You can use either the project id (recommended) or the full namespaced path.</p>
</td>
</tr>
<tr>
<td>
<code>includeSubgroups</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Recurse through subgroups (true) or scan only the base group (false).  Defaults to &ldquo;false&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>api</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Gitlab API URL to talk to.</p>
</td>
</tr>
<tr>
<td>
<code>tokenRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SecretRef">
SecretRef
</a>
</em>
</td>
<td>
<p>Authentication token reference.</p>
</td>
</tr>
<tr>
<td>
<code>allBranches</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Scan all branches instead of just the default branch.</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Skips validating the SCM provider&rsquo;s TLS certificate - useful for self-signed certificates.; default: false</p>
</td>
</tr>
<tr>
<td>
<code>includeSharedProjects</code><br/>
<em>
bool
</em>
</td>
<td>
<p>When recursing through subgroups, also include shared Projects (true) or scan only the subgroups under same path (false).  Defaults to &ldquo;true&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>topic</code><br/>
<em>
string
</em>
</td>
<td>
<p>Filter repos list based on Gitlab Topic.</p>
</td>
</tr>
<tr>
<td>
<code>caRef</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ConfigMapKeyRef">
ConfigMapKeyRef
</a>
</em>
</td>
<td>
<p>ConfigMap key holding the trusted certificates</p>
</td>
</tr>
<tr>
<td>
<code>includeArchivedRepos</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Include repositories that are archived.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SecretRef">SecretRef
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.BasicAuthBitbucketServer">BasicAuthBitbucketServer</a>, 
<a href="#argoproj.io/v1alpha1.BearerTokenBitbucket">BearerTokenBitbucket</a>, 
<a href="#argoproj.io/v1alpha1.BearerTokenBitbucketCloud">BearerTokenBitbucketCloud</a>, 
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorAzureDevOps">PullRequestGeneratorAzureDevOps</a>, 
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorGitLab">PullRequestGeneratorGitLab</a>, 
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorGitea">PullRequestGeneratorGitea</a>, 
<a href="#argoproj.io/v1alpha1.PullRequestGeneratorGithub">PullRequestGeneratorGithub</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorAzureDevOps">SCMProviderGeneratorAzureDevOps</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorBitbucket">SCMProviderGeneratorBitbucket</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorGitea">SCMProviderGeneratorGitea</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorGithub">SCMProviderGeneratorGithub</a>, 
<a href="#argoproj.io/v1alpha1.SCMProviderGeneratorGitlab">SCMProviderGeneratorGitlab</a>)
</p>
<p>
<p>SecretRef struct for a reference to a secret key.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SignatureKey">SignatureKey
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>)
</p>
<p>
<p>SignatureKey is the specification of a key required to verify commit signatures with</p>
<p>Deprecated: Use SourceIntegrity instead. SignatureKeys will be removed with the next major version.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>keyID</code><br/>
<em>
string
</em>
</td>
<td>
<p>The ID of the key in hexadecimal notation</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceHydrator">SourceHydrator
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec</a>, 
<a href="#argoproj.io/v1alpha1.HydrateOperation">HydrateOperation</a>, 
<a href="#argoproj.io/v1alpha1.SuccessfulHydrateOperation">SuccessfulHydrateOperation</a>)
</p>
<p>
<p>SourceHydrator specifies a dry &ldquo;don&rsquo;t repeat yourself&rdquo; source for manifests, a sync source from which to sync
hydrated manifests, and an optional hydrateTo location to act as a &ldquo;staging&rdquo; aread for hydrated manifests.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>drySource</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.DrySource">
DrySource
</a>
</em>
</td>
<td>
<p>DrySource specifies where the dry &ldquo;don&rsquo;t repeat yourself&rdquo; manifest source lives.</p>
</td>
</tr>
<tr>
<td>
<code>syncSource</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncSource">
SyncSource
</a>
</em>
</td>
<td>
<p>SyncSource specifies where to sync hydrated manifests from.</p>
</td>
</tr>
<tr>
<td>
<code>hydrateTo</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HydrateTo">
HydrateTo
</a>
</em>
</td>
<td>
<p>HydrateTo specifies an optional &ldquo;staging&rdquo; location to push hydrated manifests to. An external system would then
have to move manifests to the SyncSource, e.g. by pull request.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceHydratorStatus">SourceHydratorStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>SourceHydratorStatus contains information about the current state of source hydration</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>lastSuccessfulOperation</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SuccessfulHydrateOperation">
SuccessfulHydrateOperation
</a>
</em>
</td>
<td>
<p>LastSuccessfulOperation holds info about the most recent successful hydration</p>
</td>
</tr>
<tr>
<td>
<code>currentOperation</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.HydrateOperation">
HydrateOperation
</a>
</em>
</td>
<td>
<p>CurrentOperation holds the status of the hydrate operation</p>
</td>
</tr>
<tr>
<td>
<code>lastComparedDryRevision</code><br/>
<em>
string
</em>
</td>
<td>
<p>LastComparedDryRevision holds the resolved revision from the most recent dry source comparison.
This is updated on every evaluation, even when hydration is skipped due to no changes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceIntegrity">SourceIntegrity
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>git</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityGit">
SourceIntegrityGit
</a>
</em>
</td>
<td>
<p>Git - policies for git source verification</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceIntegrityCheckResult">SourceIntegrityCheckResult
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.RevisionMetadata">RevisionMetadata</a>)
</p>
<p>
<p>SourceIntegrityCheckResult represents a conclusion of the SourceIntegrity evaluation.
Each check performed on a source(es), holds a check item representing all checks performed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Checks</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityCheckResultItem">
[]SourceIntegrityCheckResultItem
</a>
</em>
</td>
<td>
<p>Checks holds a list of checks performed, with their eventual problems. If a check is not specified here,
it means it was not performed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceIntegrityCheckResultItem">SourceIntegrityCheckResultItem
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityCheckResult">SourceIntegrityCheckResult</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the check that is human-understandable pointing out to the kind of verification performed.</p>
</td>
</tr>
<tr>
<td>
<code>Problems</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Problems is a list of messages explaining why the check failed. Empty list means the check has succeeded.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceIntegrityGit">SourceIntegrityGit
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceIntegrity">SourceIntegrity</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>policies</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.SourceIntegrityGitPolicy">
[]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.SourceIntegrityGitPolicy
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceIntegrityGitPolicy">SourceIntegrityGitPolicy
</h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repos</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityGitPolicyRepo">
[]SourceIntegrityGitPolicyRepo
</a>
</em>
</td>
<td>
<p>List of repository criteria restricting repositories the policy will apply to</p>
</td>
</tr>
<tr>
<td>
<code>gpg</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityGitPolicyGPG">
SourceIntegrityGitPolicyGPG
</a>
</em>
</td>
<td>
<p>Verify GPG commit/tag signatures</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceIntegrityGitPolicyGPG">SourceIntegrityGitPolicyGPG
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityGitPolicy">SourceIntegrityGitPolicy</a>)
</p>
<p>
<p>SourceIntegrityGitPolicyGPG verifies that the commit(s) are both correctly signed by a key in the repo-server keyring,
and that they are signed by one of the key listed in Keys.</p>
<p>This policy can be deactivated through the ARGOCD_GPG_ENABLED environment variable.</p>
<p>Note the listing of problematic commits/signatures reported when &ldquo;strict&rdquo; mode validation fails may not be complete.
This means that a user that has addressed all problems reported by source integrity check can run into
further problematic signatures on a subsequent attempt. That happens namely when history contains seal commits signed
with gpg keys that are in the keyring, but not listed in Keys.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>mode</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityGitPolicyGPGMode">
SourceIntegrityGitPolicyGPGMode
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keys</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>List of key IDs to trust. The keys need to be in the repository server keyring.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SourceIntegrityGitPolicyGPGMode">SourceIntegrityGitPolicyGPGMode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityGitPolicyGPG">SourceIntegrityGitPolicyGPG</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.SourceIntegrityGitPolicyRepo">SourceIntegrityGitPolicyRepo
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceIntegrityGitPolicy">SourceIntegrityGitPolicy</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>url</code><br/>
<em>
string
</em>
</td>
<td>
<p>URL specifier, glob.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SuccessfulHydrateOperation">SuccessfulHydrateOperation
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceHydratorStatus">SourceHydratorStatus</a>)
</p>
<p>
<p>SuccessfulHydrateOperation contains information about the most recent successful hydrate operation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>drySHA</code><br/>
<em>
string
</em>
</td>
<td>
<p>DrySHA holds the resolved revision (sha) of the dry source as of the most recent reconciliation</p>
</td>
</tr>
<tr>
<td>
<code>hydratedSHA</code><br/>
<em>
string
</em>
</td>
<td>
<p>HydratedSHA holds the resolved revision (sha) of the hydrated source as of the most recent reconciliation</p>
</td>
</tr>
<tr>
<td>
<code>sourceHydrator</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">
SourceHydrator
</a>
</em>
</td>
<td>
<p>SourceHydrator holds the hydrator config used for the hydrate operation</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncOperation">SyncOperation
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.Operation">Operation</a>)
</p>
<p>
<p>SyncOperation contains details about a sync operation.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>revision</code><br/>
<em>
string
</em>
</td>
<td>
<p>Revision is the revision (Git) or chart version (Helm) which to sync the application to
If omitted, will use the revision specified in app spec.</p>
</td>
</tr>
<tr>
<td>
<code>prune</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Prune specifies to delete resources from the cluster that are no longer tracked in git</p>
</td>
</tr>
<tr>
<td>
<code>dryRun</code><br/>
<em>
bool
</em>
</td>
<td>
<p>DryRun specifies to perform a <code>kubectl apply --dry-run</code> without actually performing the sync</p>
</td>
</tr>
<tr>
<td>
<code>syncStrategy</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncStrategy">
SyncStrategy
</a>
</em>
</td>
<td>
<p>SyncStrategy describes how to perform the sync</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncOperationResource">
[]SyncOperationResource
</a>
</em>
</td>
<td>
<p>Resources describes which resources shall be part of the sync</p>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">
ApplicationSource
</a>
</em>
</td>
<td>
<p>Source overrides the source definition set in the application.
This is typically set in a Rollback operation and is nil during a Sync operation</p>
</td>
</tr>
<tr>
<td>
<code>manifests</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Manifests is an optional field that overrides sync source with a local directory for development</p>
</td>
</tr>
<tr>
<td>
<code>syncOptions</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncOptions">
SyncOptions
</a>
</em>
</td>
<td>
<p>SyncOptions provide per-sync sync-options, e.g. Validate=false</p>
</td>
</tr>
<tr>
<td>
<code>sources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSources">
ApplicationSources
</a>
</em>
</td>
<td>
<p>Sources overrides the source definition set in the application.
This is typically set in a Rollback operation and is nil during a Sync operation</p>
</td>
</tr>
<tr>
<td>
<code>revisions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Revisions is the list of revision (Git) or chart version (Helm) which to sync each source in sources field for the application to
If omitted, will use the revision specified in app spec.</p>
</td>
</tr>
<tr>
<td>
<code>autoHealAttemptsCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>SelfHealAttemptsCount contains the number of auto-heal attempts</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncOperationResource">SyncOperationResource
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncOperation">SyncOperation</a>)
</p>
<p>
<p>SyncOperationResource contains resources to sync.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>-</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncOperationResult">SyncOperationResult
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.OperationState">OperationState</a>)
</p>
<p>
<p>SyncOperationResult represent result of sync operation</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ResourceResults">
ResourceResults
</a>
</em>
</td>
<td>
<p>Resources contains a list of sync result items for each individual resource in a sync operation</p>
</td>
</tr>
<tr>
<td>
<code>revision</code><br/>
<em>
string
</em>
</td>
<td>
<p>Revision holds the revision this sync operation was performed to</p>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSource">
ApplicationSource
</a>
</em>
</td>
<td>
<p>Source records the application source information of the sync, used for comparing auto-sync</p>
</td>
</tr>
<tr>
<td>
<code>sources</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ApplicationSources">
ApplicationSources
</a>
</em>
</td>
<td>
<p>Source records the application source information of the sync, used for comparing auto-sync</p>
</td>
</tr>
<tr>
<td>
<code>revisions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Revisions holds the revision this sync operation was performed for respective indexed source in sources field</p>
</td>
</tr>
<tr>
<td>
<code>managedNamespaceMetadata</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ManagedNamespaceMetadata">
ManagedNamespaceMetadata
</a>
</em>
</td>
<td>
<p>ManagedNamespaceMetadata contains the current sync state of managed namespace metadata</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncOptions">SyncOptions
(<code>[]string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncOperation">SyncOperation</a>, 
<a href="#argoproj.io/v1alpha1.SyncPolicy">SyncPolicy</a>)
</p>
<p>
</p>
<h3 id="argoproj.io/v1alpha1.SyncPolicy">SyncPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationSpec">ApplicationSpec</a>)
</p>
<p>
<p>SyncPolicy controls when a sync will be performed in response to updates in git</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>automated</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncPolicyAutomated">
SyncPolicyAutomated
</a>
</em>
</td>
<td>
<p>Automated will keep an application synced to the target revision</p>
</td>
</tr>
<tr>
<td>
<code>syncOptions</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncOptions">
SyncOptions
</a>
</em>
</td>
<td>
<p>Options allow you to specify whole app sync-options</p>
</td>
</tr>
<tr>
<td>
<code>retry</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.RetryStrategy">
RetryStrategy
</a>
</em>
</td>
<td>
<p>Retry controls failed sync retry behavior</p>
</td>
</tr>
<tr>
<td>
<code>managedNamespaceMetadata</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ManagedNamespaceMetadata">
ManagedNamespaceMetadata
</a>
</em>
</td>
<td>
<p>ManagedNamespaceMetadata controls metadata in the given namespace (if CreateNamespace=true)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncPolicyAutomated">SyncPolicyAutomated
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncPolicy">SyncPolicy</a>)
</p>
<p>
<p>SyncPolicyAutomated controls the behavior of an automated sync</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>prune</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Prune specifies whether to delete resources from the cluster that are not found in the sources anymore as part of automated sync (default: false)</p>
</td>
</tr>
<tr>
<td>
<code>selfHeal</code><br/>
<em>
bool
</em>
</td>
<td>
<p>SelfHeal specifies whether to revert resources back to their desired state upon modification in the cluster (default: false)</p>
</td>
</tr>
<tr>
<td>
<code>allowEmpty</code><br/>
<em>
bool
</em>
</td>
<td>
<p>AllowEmpty allows apps have zero live resources (default: false)</p>
</td>
</tr>
<tr>
<td>
<code>enabled</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Enable allows apps to explicitly control automated sync</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncSource">SyncSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SourceHydrator">SourceHydrator</a>)
</p>
<p>
<p>SyncSource specifies a location from which hydrated manifests may be synced. If RepoURL is not set, it is assumed
to be the same as the associated DrySource config in the SourceHydrator.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>targetBranch</code><br/>
<em>
string
</em>
</td>
<td>
<p>TargetBranch is the branch from which hydrated manifests will be synced.
If HydrateTo is not set, this is also the branch to which hydrated manifests are committed.</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<p>Path is a directory path within the git repository where hydrated manifests should be committed to and synced
from. The Path should never point to the root of the repo. If hydrateTo is set, this is just the path from which
hydrated manifests will be synced.</p>
</td>
</tr>
<tr>
<td>
<code>repoURL</code><br/>
<em>
string
</em>
</td>
<td>
<p>RepoURL is the URL to the git repository that contains the hydrated manifests. If not set, defaults to
the DrySource.RepoURL.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncStatus">SyncStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ApplicationStatus">ApplicationStatus</a>)
</p>
<p>
<p>SyncStatus contains information about the currently observed live and desired states of an application</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncStatusCode">
SyncStatusCode
</a>
</em>
</td>
<td>
<p>Status is the sync state of the comparison</p>
</td>
</tr>
<tr>
<td>
<code>comparedTo</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.ComparedTo">
ComparedTo
</a>
</em>
</td>
<td>
<p>ComparedTo contains information about what has been compared</p>
</td>
</tr>
<tr>
<td>
<code>revision</code><br/>
<em>
string
</em>
</td>
<td>
<p>Revision contains information about the revision the comparison has been performed to</p>
</td>
</tr>
<tr>
<td>
<code>revisions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Revisions contains information about the revisions of multiple sources the comparison has been performed to</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncStatusCode">SyncStatusCode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ResourceStatus">ResourceStatus</a>, 
<a href="#argoproj.io/v1alpha1.SyncStatus">SyncStatus</a>)
</p>
<p>
<p>SyncStatusCode is a type which represents possible comparison results</p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;OutOfSync&#34;</p></td>
<td><p>SyncStatusCodeOutOfSync indicates that there is a drift between desired and live states</p>
</td>
</tr><tr><td><p>&#34;Synced&#34;</p></td>
<td><p>SyncStatusCodeSynced indicates that desired and live states match</p>
</td>
</tr><tr><td><p>&#34;Unknown&#34;</p></td>
<td><p>SyncStatusCodeUnknown indicates that the status of a sync could not be reliably determined</p>
</td>
</tr></tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncStrategy">SyncStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncOperation">SyncOperation</a>)
</p>
<p>
<p>SyncStrategy controls the manner in which a sync is performed</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apply</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncStrategyApply">
SyncStrategyApply
</a>
</em>
</td>
<td>
<p>Apply will perform a <code>kubectl apply</code> to perform the sync.</p>
</td>
</tr>
<tr>
<td>
<code>hook</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncStrategyHook">
SyncStrategyHook
</a>
</em>
</td>
<td>
<p>Hook will submit any referenced resources to perform the sync. This is the default strategy</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncStrategyApply">SyncStrategyApply
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncStrategy">SyncStrategy</a>, 
<a href="#argoproj.io/v1alpha1.SyncStrategyHook">SyncStrategyHook</a>)
</p>
<p>
<p>SyncStrategyApply uses <code>kubectl apply</code> to perform the apply</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>force</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Force indicates whether or not to supply the &ndash;force flag to <code>kubectl apply</code>.
The &ndash;force flag deletes and re-create the resource, when PATCH encounters conflict and has
retried for 5 times.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncStrategyHook">SyncStrategyHook
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.SyncStrategy">SyncStrategy</a>)
</p>
<p>
<p>SyncStrategyHook will perform a sync using hooks annotations.
If no hook annotation is specified falls back to <code>kubectl apply</code>.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>SyncStrategyApply</code><br/>
<em>
<a href="#argoproj.io/v1alpha1.SyncStrategyApply">
SyncStrategyApply
</a>
</em>
</td>
<td>
<p>
(Members of <code>SyncStrategyApply</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Embed SyncStrategyApply type to inherit any <code>apply</code> options</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncWindow">SyncWindow
</h3>
<p>
<p>SyncWindow contains the kind, time, duration and attributes that are used to assign the syncWindows to apps</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
<p>Kind defines if the window allows or blocks syncs</p>
</td>
</tr>
<tr>
<td>
<code>schedule</code><br/>
<em>
string
</em>
</td>
<td>
<p>Schedule is the time the window will begin, specified in cron format</p>
</td>
</tr>
<tr>
<td>
<code>duration</code><br/>
<em>
string
</em>
</td>
<td>
<p>Duration is the amount of time the sync window will be open</p>
</td>
</tr>
<tr>
<td>
<code>applications</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Applications contains a list of applications that the window will apply to</p>
</td>
</tr>
<tr>
<td>
<code>namespaces</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Namespaces contains a list of namespaces that the window will apply to</p>
</td>
</tr>
<tr>
<td>
<code>clusters</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Clusters contains a list of clusters that the window will apply to</p>
</td>
</tr>
<tr>
<td>
<code>manualSync</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ManualSync enables manual syncs when they would otherwise be blocked</p>
</td>
</tr>
<tr>
<td>
<code>timeZone</code><br/>
<em>
string
</em>
</td>
<td>
<p>TimeZone of the sync that will be applied to the schedule</p>
</td>
</tr>
<tr>
<td>
<code>andOperator</code><br/>
<em>
bool
</em>
</td>
<td>
<p>UseAndOperator use AND operator for matching applications, namespaces and clusters instead of the default OR operator</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<p>Description of the sync that will be applied to the schedule, can be used to add any information such as a ticket number for example</p>
</td>
</tr>
<tr>
<td>
<code>syncOverrun</code><br/>
<em>
bool
</em>
</td>
<td>
<p>SyncOverrun allows ongoing syncs to continue in two scenarios:
For deny windows: allows syncs that started before the deny window became active to continue running
For allow windows: allows syncs that started during the allow window to continue after the window ends</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.SyncWindows">SyncWindows
(<code>[]*github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1.SyncWindow</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.AppProjectSpec">AppProjectSpec</a>)
</p>
<p>
<p>SyncWindows is a collection of sync windows in this project</p>
</p>
<h3 id="argoproj.io/v1alpha1.TLSClientConfig">TLSClientConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ClusterConfig">ClusterConfig</a>)
</p>
<p>
<p>TLSClientConfig contains settings to enable transport layer security</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>insecure</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Insecure specifies that the server should be accessed without verifying the TLS certificate. For testing only.</p>
</td>
</tr>
<tr>
<td>
<code>serverName</code><br/>
<em>
string
</em>
</td>
<td>
<p>ServerName is passed to the server for SNI and is used in the client to check server
certificates against. If ServerName is empty, the hostname used to contact the
server is used.</p>
</td>
</tr>
<tr>
<td>
<code>certData</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>CertData holds PEM-encoded bytes (typically read from a client certificate file).
CertData takes precedence over CertFile</p>
</td>
</tr>
<tr>
<td>
<code>keyData</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>KeyData holds PEM-encoded bytes (typically read from a client certificate key file).
KeyData takes precedence over KeyFile</p>
</td>
</tr>
<tr>
<td>
<code>caData</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
CAData takes precedence over CAFile</p>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.TagFilter">TagFilter
</h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="argoproj.io/v1alpha1.TrackingMethod">TrackingMethod
(<code>string</code> alias)</p></h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;annotation&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;annotation&#43;label&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;label&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>.
</em></p>

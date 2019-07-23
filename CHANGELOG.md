# Changelog

## v1.1.0 (2019-07-23)

* Fix argocd app sync/get cli ([#1959](https://github.com/argoproj/argo-cd/issues/1959))
* Issue [#1935](https://github.com/argoproj/argo-cd/issues/1935) - `argocd app sync` hangs when cluster is not configured [#1935](https://github.com/argoproj/argo-cd/issues/1935) (#1962)
* Remove unnecessary details from sync errors ([#1951](https://github.com/argoproj/argo-cd/issues/1951))
* Issue [#1919](https://github.com/argoproj/argo-cd/issues/1919) - Eliminate unnecessary git interactions for top-level resource changes (#1929)
* Do not allow app-of-app child app's Missing status to affect parent ([#1954](https://github.com/argoproj/argo-cd/issues/1954))
* Improve sync result messages. Closes [#1486](https://github.com/argoproj/argo-cd/issues/1486) (#1768)
* Change git prometheus counter name ([#1949](https://github.com/argoproj/argo-cd/issues/1949))
* Update k8s libraries to v1.14 ([#1806](https://github.com/argoproj/argo-cd/issues/1806))
* Issue [#1912](https://github.com/argoproj/argo-cd/issues/1912) - Add Prometheus metrics for git repo interactions (#1914)
* Issue [#1909](https://github.com/argoproj/argo-cd/issues/1909) - App controller should log additional information during app syncing (#1910)
* Upgrade argo ui version to pull dropdown fix ([#1906](https://github.com/argoproj/argo-cd/issues/1906))
* Upgrade argo ui version to pull dropdown fix ([#1899](https://github.com/argoproj/argo-cd/issues/1899))
* Log more error information. See [#1887](https://github.com/argoproj/argo-cd/issues/1887) (#1891)
* Issue [#1874](https://github.com/argoproj/argo-cd/issues/1874) - validate app spec before verifying app permissions (#1875)
* Redacts Helm username and password. Closes [#1868](https://github.com/argoproj/argo-cd/issues/1868) (#1871)
* Issue [#1867](https://github.com/argoproj/argo-cd/issues/1867) - Fix JS error on project role edit panel (#1869)
* Upgrade argo-ui version to fix dropdown position calculation ([#1847](https://github.com/argoproj/argo-cd/issues/1847))
* Removes logging that appears when using the  CLI ([#1842](https://github.com/argoproj/argo-cd/issues/1842))
* Added local path syncing ([#1578](https://github.com/argoproj/argo-cd/issues/1578))
* Added local sync to docs ([#1771](https://github.com/argoproj/argo-cd/issues/1771))
* Issue [#1820](https://github.com/argoproj/argo-cd/issues/1820) - Make sure api server to repo server grpc calls have timeout (#1832)
* Adds a timeout to all external commands. Closes [#1821](https://github.com/argoproj/argo-cd/issues/1821) (#1823)
* Running application actions should require `override` privileges not `get` ([#1828](https://github.com/argoproj/argo-cd/issues/1828))
* Use correct healthcheck for Rollout with empty steps list ([#1776](https://github.com/argoproj/argo-cd/issues/1776))
* Move remarshaling to happen only during comparison, instead of manifest generation ([#1788](https://github.com/argoproj/argo-cd/issues/1788))
* Server side rotation of cluster bearer tokens ([#1744](https://github.com/argoproj/argo-cd/issues/1744))
* Add health check to the controller deployment ([#1785](https://github.com/argoproj/argo-cd/issues/1785))
* Make status fields as optional fields ([#1779](https://github.com/argoproj/argo-cd/issues/1779))
* Sync status button should be hidden if there is no sync operation ([#1770](https://github.com/argoproj/argo-cd/issues/1770))
* UI should allow editing repo URL ([#1763](https://github.com/argoproj/argo-cd/issues/1763))
* Fixes a bug where cluster objs could leave app is running op state. C… ([#1796](https://github.com/argoproj/argo-cd/issues/1796))
* Adds support for SSH keys with Kustomize remote bases WIP ([#1733](https://github.com/argoproj/argo-cd/issues/1733))
* Added `--async` flag to `argocd app sync` ([#1738](https://github.com/argoproj/argo-cd/issues/1738))
* Support parameterizing argocd base image ([#1741](https://github.com/argoproj/argo-cd/issues/1741))
* Issue [#1677](https://github.com/argoproj/argo-cd/issues/1677) - Allow users to define app specific urls to expose in the UI (#1714)
* Add Optoro to list of users ([#1737](https://github.com/argoproj/argo-cd/issues/1737))
* Adding Volvo Cars as officially using ArgoCD ([#1735](https://github.com/argoproj/argo-cd/issues/1735))
* No longer waits for healthy before completing sync op. Closes [#1715](https://github.com/argoproj/argo-cd/issues/1715) (#1727)
* Issue [#1375](https://github.com/argoproj/argo-cd/issues/1375) - Error view instead of blank page in UI (#1726)
* Helm parameter fix ([#1732](https://github.com/argoproj/argo-cd/issues/1732))
* Fix key generation loop when running server on insecure mode ([#1723](https://github.com/argoproj/argo-cd/issues/1723))
* Fixes non-escaped comma bug on Helm command arguments ([#1720](https://github.com/argoproj/argo-cd/issues/1720))
* Order users alphabetically ([#1721](https://github.com/argoproj/argo-cd/issues/1721))
* Add ui/node_modules to docker ignore ([#1725](https://github.com/argoproj/argo-cd/issues/1725))
* Issue [#1693](https://github.com/argoproj/argo-cd/issues/1693) - Project Editor: Whitelisted Cluster Resources doesn't strip whitespace (#1722)
* Issue [#1711](https://github.com/argoproj/argo-cd/issues/1711) - Upgrade argo ui version to get dropdown fix (#1717)
* Forward git credentials to config management plugins. Closes [#1628](https://github.com/argoproj/argo-cd/issues/1628) (#1716)
* Adds documentation around repo connections ([#1709](https://github.com/argoproj/argo-cd/issues/1709))
* Issue [#1701](https://github.com/argoproj/argo-cd/issues/1701) - UI will crash when create application without destination namespace (#1713)
* Adding Telsa to list of users ([#1712](https://github.com/argoproj/argo-cd/issues/1712))
* Account for missing fields in Rollout HealthStatus ([#1699](https://github.com/argoproj/argo-cd/issues/1699))
* Added logout ability (`argocd logout`) ([#1582](https://github.com/argoproj/argo-cd/issues/1582))
* Adds Prune=false and IgnoreExtraneous options ([#1680](https://github.com/argoproj/argo-cd/issues/1680))
* Restore reposerver in Procfile ([#1708](https://github.com/argoproj/argo-cd/issues/1708))
* Name e2e apps after the test they run for, rather than random ID. ([#1698](https://github.com/argoproj/argo-cd/issues/1698))
* Improve Circle CI builds ([#1691](https://github.com/argoproj/argo-cd/issues/1691))
* Updates generated code ([#1707](https://github.com/argoproj/argo-cd/issues/1707))
* Support to override helm release name ([#1682](https://github.com/argoproj/argo-cd/issues/1682))
* Add Mirantis as an official user ([#1702](https://github.com/argoproj/argo-cd/issues/1702))
* Handle nil obj when processing custom actions ([#1700](https://github.com/argoproj/argo-cd/issues/1700))
* Documents HA/DR ([#1690](https://github.com/argoproj/argo-cd/issues/1690))
* Move generated api code to pkg package ([#1696](https://github.com/argoproj/argo-cd/issues/1696))
* Bump base version to 1.0.1 for cluster-install ([#1695](https://github.com/argoproj/argo-cd/issues/1695))
* Adds custom port repo note ([#1694](https://github.com/argoproj/argo-cd/issues/1694))
* Sync wave ([#1634](https://github.com/argoproj/argo-cd/issues/1634))
* Tidy up [#1684](https://github.com/argoproj/argo-cd/issues/1684) (#1689)
* Update SUPPORT.md ([#1681](https://github.com/argoproj/argo-cd/issues/1681))
* Merge pull request [#1688](https://github.com/argoproj/argo-cd/issues/1688) from argoproj/merge-ui
* add tZERO to organizations using Argo CD list ([#1686](https://github.com/argoproj/argo-cd/issues/1686))
* Added Codility to ArgoCD users ([#1679](https://github.com/argoproj/argo-cd/issues/1679))
* codegen ([#1674](https://github.com/argoproj/argo-cd/issues/1674))
* Add ability to specify system namespace during cluster add operation ([#1661](https://github.com/argoproj/argo-cd/issues/1661))
* Issue [#1668](https://github.com/argoproj/argo-cd/issues/1668) - Replicasets ordering is not stable on app tree view (#1669)
* Fix broken e2e tests ([#1667](https://github.com/argoproj/argo-cd/issues/1667))
* Adds docs about app deletion ([#1664](https://github.com/argoproj/argo-cd/issues/1664))
* Issue [#1665](https://github.com/argoproj/argo-cd/issues/1665) - Stuck processor on App Controller after deleting application with incomplete operation (#1666)
* Update releasing.md ([#1657](https://github.com/argoproj/argo-cd/issues/1657))
* Issue [#1662](https://github.com/argoproj/argo-cd/issues/1662) - Role edit page fails with JS error (#126)
* Terminates op before delete ([#1658](https://github.com/argoproj/argo-cd/issues/1658))
* Issue [#1609](https://github.com/argoproj/argo-cd/issues/1609) - Improve Kustomize 2 parameters UI (#125)
* Make listener and metrics ports configurable ([#1647](https://github.com/argoproj/argo-cd/issues/1647))
* Build ArgoCD on CircleCI ([#1635](https://github.com/argoproj/argo-cd/issues/1635))
* Updated templates ([#1654](https://github.com/argoproj/argo-cd/issues/1654))
* Update README.md ([#1650](https://github.com/argoproj/argo-cd/issues/1650))
* Add END. to adopters in README.md ([#1643](https://github.com/argoproj/argo-cd/issues/1643))
* Make build options in Makefile settable from environment ([#1619](https://github.com/argoproj/argo-cd/issues/1619))
* Codegen ([#1632](https://github.com/argoproj/argo-cd/issues/1632))
* Update v1.0.0 change log ([#1618](https://github.com/argoproj/argo-cd/issues/1618))
* Fixes e2e tests. Closes [#1616](https://github.com/argoproj/argo-cd/issues/1616). (#1617)
* E2e test infra ([#1600](https://github.com/argoproj/argo-cd/issues/1600))
* Issue [#1352](https://github.com/argoproj/argo-cd/issues/1352) - Dedupe live resourced by UID instead of group/kind/namespace/name (#123)
* Updates codegen ([#1601](https://github.com/argoproj/argo-cd/issues/1601))
* Updates issue template and Makefile ([#1598](https://github.com/argoproj/argo-cd/issues/1598))
* Issue [#1592](https://github.com/argoproj/argo-cd/issues/1592) - Fix UI Crash is app never been reconciled
* Documents Kustomize. Closes [#1566](https://github.com/argoproj/argo-cd/issues/1566) (#1572)
* add commonbond to users of argocd ([#1577](https://github.com/argoproj/argo-cd/issues/1577))
* Add GMETRI to organizations using ArgoCD ([#1564](https://github.com/argoproj/argo-cd/issues/1564))
* Issue [#1563](https://github.com/argoproj/argo-cd/issues/1563) - Network view crashes if any filter is set (#122)
* Fix broken applications chart icon ([#121](https://github.com/argoproj/argo-cd/issues/121))
* Issue [#1550](https://github.com/argoproj/argo-cd/issues/1550) - Support ':' character in resource name (#120)
* Updates manifests. Closes [#1520](https://github.com/argoproj/argo-cd/issues/1520) (#1549)
* Adds missing section to docs ([#1537](https://github.com/argoproj/argo-cd/issues/1537))
* Add kustomize ([#1541](https://github.com/argoproj/argo-cd/issues/1541))
* fix typo in best practices ([#1538](https://github.com/argoproj/argo-cd/issues/1538))
* Documents cluster bootstrapping. Close [#1481](https://github.com/argoproj/argo-cd/issues/1481) (#1530)
* Update CONTRIBUTING.md ([#1534](https://github.com/argoproj/argo-cd/issues/1534))
* Fix e2e ([#1526](https://github.com/argoproj/argo-cd/issues/1526))
* codegen ([#1521](https://github.com/argoproj/argo-cd/issues/1521))
* Updated CHANGELOG.md ([#1518](https://github.com/argoproj/argo-cd/issues/1518))
* Add Network View description to changelog ([#1519](https://github.com/argoproj/argo-cd/issues/1519))
* Issue [#1507](https://github.com/argoproj/argo-cd/issues/1507) - Selective sync is broken in UI (#118)
* Issue [#1502](https://github.com/argoproj/argo-cd/issues/1502) - UI fails to load custom actions is resource is not deployed (#117)
* Issue [#1503](https://github.com/argoproj/argo-cd/issues/1503) - Events tab title is not right if resources has no errors (#116)
* Issue [#1505](https://github.com/argoproj/argo-cd/issues/1505) - Fix broken node resource panel (#115)
* Adds event count. Closes argoproj/argo-cd[#1477](https://github.com/argoproj/argo-cd/issues/1477) (#113)
* Issue [#1386](https://github.com/argoproj/argo-cd/issues/1386) - Improve notifications rendering (#112)
* Network view external nodes ([#109](https://github.com/argoproj/argo-cd/issues/109))
* Allows health to be null in the UI ([#104](https://github.com/argoproj/argo-cd/issues/104))
* Issue [#1217](https://github.com/argoproj/argo-cd/issues/1217) - Improve form input usability
* Issue [#1354](https://github.com/argoproj/argo-cd/issues/1354) - [UI] default view should resource view instead of diff view
* Issue [#1368](https://github.com/argoproj/argo-cd/issues/1368) - [UI] applications view blows up when user does not have  permissions
* Issue [#1357](https://github.com/argoproj/argo-cd/issues/1357) - Dropdown menu should not have sync item for unmanaged resources
* Support tab deep linking on app details page ([#102](https://github.com/argoproj/argo-cd/issues/102))
* Changing SSO login URL to be a relative link so it's affected by basehref ([#101](https://github.com/argoproj/argo-cd/issues/101))
* Support overriding image name/tag in for Kustomize 2 apps ([#97](https://github.com/argoproj/argo-cd/issues/97))
* Issue [#1310](https://github.com/argoproj/argo-cd/issues/1310) - application table view needs to be sorted
* Issue [#1282](https://github.com/argoproj/argo-cd/issues/1282) - Prevent filering out application node on Applicatoin details page
* Issue [#1261](https://github.com/argoproj/argo-cd/issues/1261) - UI loads helm parameters without taking into account selected values files
* Issue [#1058](https://github.com/argoproj/argo-cd/issues/1058) - Allows you to set sync-policy when you create an app
* Issue [#1236](https://github.com/argoproj/argo-cd/issues/1236) - project field in 'create application' dialog is confusing
* Issue [#1141](https://github.com/argoproj/argo-cd/issues/1141) - Deprecate ComponentParameterOverrides in favor of source specific config
* Issue [#1122](https://github.com/argoproj/argo-cd/issues/1122) - Autosuggest should expand to the top is there is not enough space to expand bottom
* Issue [#1176](https://github.com/argoproj/argo-cd/issues/1176) - UI should support raw YAML editor when creating/updating an app
* Issue [#1086](https://github.com/argoproj/argo-cd/issues/1086) - Switch to text based YAML diff instead of json diff
* Issue [#1152](https://github.com/argoproj/argo-cd/issues/1152) - Render cluster name in application wizard
* Issue [#1160](https://github.com/argoproj/argo-cd/issues/1160) - Deleting an application child resource from a parent application deletes the parent
* Don't show directory app parameters for kustomize apps ([#92](https://github.com/argoproj/argo-cd/issues/92))
* Issue [#929](https://github.com/argoproj/argo-cd/issues/929) - Add indicator to app resources tree if resources are filtered
* Issue [#1101](https://github.com/argoproj/argo-cd/issues/1101) - Add menu to resource list table (#91)
* Issue [#1055](https://github.com/argoproj/argo-cd/issues/1055) - Render sync/health status filter checkboxes even if there are not apps in that status
* Issue [#279](https://github.com/argoproj/argo-cd/issues/279) - improve empty state design
* Issue [#1061](https://github.com/argoproj/argo-cd/issues/1061) - Implement table view mode on applications list page
* Issue [#1036](https://github.com/argoproj/argo-cd/issues/1036) - Fix rendering resources state without status
* Issue [#1032](https://github.com/argoproj/argo-cd/issues/1032) - fix JS error during editing helm app without value files
* Issue [#1028](https://github.com/argoproj/argo-cd/issues/1028) - Resource details 'blink' when resource changes
* Issue [#1027](https://github.com/argoproj/argo-cd/issues/1027) - UI should render page title to simplify navigation
* Issue [#966](https://github.com/argoproj/argo-cd/issues/966) - UI error with helm charts parameters
* Issue [#969](https://github.com/argoproj/argo-cd/issues/969) - Fix rendering number of application parameter overrides
* Issue [#952](https://github.com/argoproj/argo-cd/issues/952) - Add helm file if user selected file name from autocompletion dropdown
* Issue 914 - Add application force refresh button ([#88](https://github.com/argoproj/argo-cd/issues/88))
* Issue 906 - Support setting different base href in UI ([#87](https://github.com/argoproj/argo-cd/issues/87))
* Issue [#909](https://github.com/argoproj/argo-cd/issues/909) - add sync and health filters
* Issue [#417](https://github.com/argoproj/argo-cd/issues/417) - Add force delete option for deleting resources
* Add sync and health details to app header ([#85](https://github.com/argoproj/argo-cd/issues/85))
* Issue [#741](https://github.com/argoproj/argo-cd/issues/741) - Trim repo URL in app creation wizard
* Issue [#732](https://github.com/argoproj/argo-cd/issues/732) - Cmd+Click should open app in new tab
* Issue [#821](https://github.com/argoproj/argo-cd/issues/821) - Login button when external OIDC provider is configured
* Remove parameters field from ApplicationStatus ([#83](https://github.com/argoproj/argo-cd/issues/83))
* Issue [#740](https://github.com/argoproj/argo-cd/issues/740) - Render synced to revision
* Issue [#822](https://github.com/argoproj/argo-cd/issues/822) - No error indication when insufficient permissions to create tokens
* Remove ability to set helm release name ([#80](https://github.com/argoproj/argo-cd/issues/80))
* Rename 'controlled resources' to 'managed resources' ([#78](https://github.com/argoproj/argo-cd/issues/78))
* Support project whitelists/blacklists rendering and editing ([#77](https://github.com/argoproj/argo-cd/issues/77))
* Present a 'deletion' operation while application is deleting ([#76](https://github.com/argoproj/argo-cd/issues/76))
* Issue [#768](https://github.com/argoproj/argo-cd/issues/768) - Fix application wizard crash (#72)
* Show operation without status.operationStatus existing ([#70](https://github.com/argoproj/argo-cd/issues/70))
* Show confirmation message only if sync is successful ([#66](https://github.com/argoproj/argo-cd/issues/66))
* Issue [#707](https://github.com/argoproj/argo-cd/issues/707) - Application details page don't allow editing parameter if parameter name has '.' (#65)
* Issue [#693](https://github.com/argoproj/argo-cd/issues/693) - Input type text instead of password on Connect repo panel (#63)
* Issue [#655](https://github.com/argoproj/argo-cd/issues/655) - Generate role token click resets policy changes (#62)
* Issue [#685](https://github.com/argoproj/argo-cd/issues/685) - Better update conflict error handing during app editing in UI (#61)
* Issue [#681](https://github.com/argoproj/argo-cd/issues/681) - Display init container logs (#60)
* Issue [#683](https://github.com/argoproj/argo-cd/issues/683) - Resource nodes are 'jumping' on app details page (#59)
* Issue 348 - Redirect to /auth/login instead of /login when SSO token expires ([#58](https://github.com/argoproj/argo-cd/issues/58))
* Issue [#669](https://github.com/argoproj/argo-cd/issues/669) - Sync always suggest using latest revision instead of target (#57)
* Move form-form components to argo-ui; Use autocomplete component ([#54](https://github.com/argoproj/argo-cd/issues/54))
* Move DataLoader and NotificationError components to argo-ui libarary ([#50](https://github.com/argoproj/argo-cd/issues/50))
* Issue [#615](https://github.com/argoproj/argo-cd/issues/615) - Ability to modify application from UI (#49)
* Issue [#566](https://github.com/argoproj/argo-cd/issues/566) - indicate when operation is in progress or has failed (#46)
* Issue [#601](https://github.com/argoproj/argo-cd/issues/601) - Fix NPE in getResourceLabels function (#44)
* Issue [#573](https://github.com/argoproj/argo-cd/issues/573) - Projects filter does not work when application got changed (#42)
* Issue [#562](https://github.com/argoproj/argo-cd/issues/562) - App creation wizard should allow specifying source revision (#41)
* Issue [#396](https://github.com/argoproj/argo-cd/issues/396) - provide a YAML view of resources (#40)
* Merge pull request [#37](https://github.com/argoproj/argo-cd/issues/37) from merenbach/539-indicate-notready-pods
* Add ability edit projects with * sources and destinations ([#33](https://github.com/argoproj/argo-cd/issues/33))
* App create wizard support for kustomize apps ([#31](https://github.com/argoproj/argo-cd/issues/31))
* Merge pull request [#27](https://github.com/argoproj/argo-cd/issues/27) from alexmt/459-app-wizard-improvement
* Issue [#459](https://github.com/argoproj/argo-cd/issues/459) - Improve application creation wizard
* Merge pull request [#26](https://github.com/argoproj/argo-cd/issues/26) from alexmt/474-list-apps
* Merge pull request [#25](https://github.com/argoproj/argo-cd/issues/25) from alexmt/446-loading-error-notification
* Issue [#446](https://github.com/argoproj/argo-cd/issues/446) - Improve data loading errors notification
* Merge pull request [#23](https://github.com/argoproj/argo-cd/issues/23) from merenbach/fix-application-card
* Merge pull request [#22](https://github.com/argoproj/argo-cd/issues/22) from alexmt/443-helm-app
* Merge pull request [#20](https://github.com/argoproj/argo-cd/issues/20) from alexmt/340-app-events-ui
* Issue [#406](https://github.com/argoproj/argo-cd/issues/406) - add button to terminate a operation
* Issue [#402](https://github.com/argoproj/argo-cd/issues/402) - App deployment history don't display parameter overrides
* Issue [#400](https://github.com/argoproj/argo-cd/issues/400) - Provide a link to swagger UI
* Issue [#290](https://github.com/argoproj/argo-cd/issues/290) - Cluster list page
* Issue [#341](https://github.com/argoproj/argo-cd/issues/341) - add refresh button in app view
* Issue [#337](https://github.com/argoproj/argo-cd/issues/337) - remember my resource filtering preferences
* Issue [#306](https://github.com/argoproj/argo-cd/issues/306) - UI should allow redeploying most recent successful deployment from history
* Issue [#352](https://github.com/argoproj/argo-cd/issues/352) -  resource names are almost always truncated
* Support  option for app sync operation on app details page [#289](https://github.com/argoproj/argo-cd/issues/289)
* Issue [#231](https://github.com/argoproj/argo-cd/issues/231) - Display pod status on application details page
* Issue [#286](https://github.com/argoproj/argo-cd/issues/286) - Resource events tab on application details page
* Issue [#241](https://github.com/argoproj/argo-cd/issues/241) - Repositories list page
* Issue [#232](https://github.com/argoproj/argo-cd/issues/232) - Resource filtering on Application Details page
* Issue [#235](https://github.com/argoproj/argo-cd/issues/235) - Allow viewing pod side car container logs
* Issue [#230](https://github.com/argoproj/argo-cd/issues/230) - Display operation state on application details page
* Issue [#184](https://github.com/argoproj/argo-cd/issues/184) - Allow downloading of argocd binaries directly from API server
* Issue [#189](https://github.com/argoproj/argo-cd/issues/189) - switch to Spec.Destination.Server/Namespace fields
* Issue [#191](https://github.com/argoproj/argo-cd/issues/191) - ArgoCD UI s/rollback/history/ and s/deploy/sync/

## v1.0.0 (2019-05-16)

### New Features

#### Network View

A new way to visual application resources had been introduced to the Application Details page. The Network View visualizes connections between Ingresses, Services and Pods
based on ingress reference service, service's label selectors and labels. The new view is useful to understand the application traffic flow and troubleshot connectivity issues.

#### Custom Actions

Argo CD introduces Custom Resource Actions to allow users to provide their own Lua scripts to modify existing Kubernetes resources in their applications. These actions are exposed in the UI to allow easy, safe, and reliable changes to their resources.  This functionality can be used to introduce functionality such as suspending and enabling a Kubernetes cronjob, continue a BlueGreen deployment with Argo Rollouts, or scaling a deployment. 

#### UI Enhancements & Usability Enhancements

* New color palette intended to highlight unhealthily and out-of-sync resources more clearly.
* The health of more resources is displayed, so it easier to quickly zoom to unhealthy pods, replica-sets, etc.
* Resources that do not have health no longer appear to be healthy. 
* Support for configuring Git repo credentials at a domain/org level
* Support for configuring requested OIDC provider scopes and enforced RBAC scopes
* Support for configuring monitored resources whitelist in addition to excluded resources

### Breaking Changes

* Remove deprecated componentParameterOverrides field #1372

### Changes since v0.12.2

#### Enhancements

* `argocd app wait` should have `--resource` flag like sync #1206
* Adds support for `kustomize edit set image`. Closes #1275 (#1324)
* Allow wait to return on health or suspended (#1392)
* Application warning when a manifest is defined twice #1070
* Create new documentation website #1390
* Default view should resource view instead of diff view #1354
* Display number of errors on resource tab #1477
* Displays resources that are being deleted as "Progressing". Closes #1410 (#1426)
* Generate random name for grpc proxy unix socket file instead of time stamp (#1455)
* Issue #357 - Expose application nodes networking information (#1333)
* Issue #1404 - App controller unnecessary set namespace to cluster level resources (#1405)
* Nils health if the resource does not provide it. Closes #1383 (#1408)
* Perform health assessments on all resource nodes in the tree. Closes #1382 (#1422)
* Remove deprecated componentParameterOverrides field #1372
* Shows the health of the application. Closes #1433 (#1434)
* Surface Service/Ingress external IPs, hostname to application #908
* Surface pod status to tree view #1358
* Support for customizable resource actions as Lua scripts #86
* UI / API Errors Truncated, Time Out #1386
* UI Enhancement Proposals Quick Wins #1274
* Update argocd-util import/export to support proper backup and restore (#1328)
* Whitelisting repos/clusters in projects should consider repo/cluster permissions #1432
* Adds support for configuring repo creds at a domain/org level. (#1332)
* Implement whitelist option analogous to `resource.exclusions` (#1490)
* Added ability to sync specific labels from the command line (#1241)
* Improve rendering app image information (#1552)
* Add liveness probe to repo server/api servers (#1546)
* Support configuring requested OIDC provider scopes and enforced RBAC scopes (#1471)

#### Bug Fixes

- Don't compare secrets in the CLI, since argo-cd doesn't have access to their data (#1459)
- Dropdown menu should not have sync item for unmanaged resources #1357
- Fixes goroutine leak. Closes #1381 (#1457)
- Improve input style #1217
- Issue #908 - Surface Service/Ingress external IPs, hostname to application (#1347)
- kustomization fields are all mandatory #1504
- Resource node details is crashing if live resource is missing $1505
- Rollback UI is not showing correct ksonnet parameters in preview #1326
- See details of applications fails with "r.nodes is undefined" #1371
- UI fails to load custom actions is resource is not deployed #1502
- Unable to create app from private repo: x509: certificate signed by unknown authority (#1171)
- Fix hardcoded 'git' user in `util/git.NewClient` (#1555)
- Application controller becomes unresponsive (#1476)
- Load target resource using K8S if conversion fails (#1414)
- Can't ignore a non-existent pointer anymore (#1586)
- Impossible to sync to HEAD from UI if auto-sync is enabled (#1579)
- Application controller is unable to delete self-referenced app (#1570)
- Prevent reconciliation loop for self-managed apps (#1533)
- Controller incorrectly report health state of self managed application (#1557)
- Fix kustomize manifest generation crash is manifest has image without version (#1540)
- Supply resourceVersion to watch request to prevent reading of stale cache (#1605)

## v0.12.2 (2019-04-22)

### Changes since v0.12.1

- Fix racing condition in controller cache (#1498)
- "bind: address already in use" after switching to gRPC-Web (#1451)
- Annoying warning while using --grpc-web flag (#1420)
- Delete helm temp directories (#1446)
- Fix null pointer exception in secret normalization function (#1389)
- Argo CD should not delete CRDs(#1425)
- UI is unable to load cluster level resource manifest (#1429)

## v0.12.1 (2019-04-09)

### Changes since v0.12.0

- [UI] applications view blows up when user does not have  permissions (#1368)
- Add k8s objects circular dependency protection to getApp method (#1374)
- App controller unnecessary set namespace to cluster level resources (#1404)
- Changing SSO login URL to be a relative link so it's affected by basehref (#101) (@arnarg)
- CLI diff should take into account resource customizations (#1294)
- Don't try deleting application resource if it already has `deletionTimestamp` (#1406)
- Fix invalid group filtering in 'patch-resource' command (#1319)
- Fix null pointer dereference error in 'argocd app wait' (#1366)
- kubectl v1.13 fails to convert extensions/NetworkPolicy (#1012)
- Patch APIs are not audited (#1397)

+ 'argocd app wait' should fail sooner if app transitioned to Degraded state (#733)
+ Add mapping to new canonical Ingress API group - kubernetes 1.14 support (#1348) (@twz123)
+ Adds support for `kustomize edit set image`. (#1275)
+ Allow using any name for secrets which store cluster credentials (#1218)
+ Update argocd-util import/export to support proper backup and restore (#1048)

## v0.12.0 (2019-03-20)

### New Features

#### Improved UI

Many improvements to the UI were made, including:

* Table view when viewing applications
* Filters on applications
* Table view when viewing application resources
* YAML editor in UI
* Switch to text-based diff instead of json diff
* Ability to edit application specs

#### Custom Health Assessments (CRD Health)

Argo CD has long been able to perform health assessments on resources, however this could only
assess the health for a few native kubernetes types (deployments, statefulsets, daemonsets, etc...).
Now, Argo CD can be extended to gain understanding of any CRD health, in the form of Lua scripts.
For example, using this feature, Argo CD now understands the CertManager Certificate CRD and will
report a Degraded status when there are issues with the cert.

#### Configuration Management Plugins

Argo CD introduces Config Management Plugins to support custom configuration management tools other
than the set that Argo CD provides out-of-the-box (Helm, Kustomize, Ksonnet, Jsonnet). Using config
management plugins, Argo CD can be configured to run specified commands to render manifests. This
makes it possible for Argo CD to support other config management tools (kubecfg, kapitan, shell
scripts, etc...).

#### High Availability

Argo CD is now fully HA. A set HA of manifests are provided for users who wish to run Argo CD in
a highly available manner. NOTE: The HA installation will require at least three different nodes due
to pod anti-affinity roles in the specs.

#### Improved Application Source

* Support for Kustomize 2
* YAML/JSON/Jsonnet Directories can now be recursed
* Support for Jsonnet external variables and top-level arguments

#### Additional Prometheus Metrics

Argo CD provides the following additional prometheus metrics:
* Sync counter to track sync activity and results over time
* Application reconciliation (refresh) performance to track Argo CD performance and controller activity
* Argo CD API Server metrics for monitoring HTTP/gRPC requests

#### Fuzzy Diff Logic

Argo CD can now be configured to ignore known differences for resource types by specifying a json
pointer to the field path to ignore. This helps prevent OutOfSync conditions when a user has no
control over the manifests. Ignored differences can be configured either at an application level, 
or a system level, based on a group/kind.

#### Resource Exclusions

Argo CD can now be configured to completely ignore entire classes of resources group/kinds.
Excluding high-volume resources improves performance and memory usage, and reduces load and
bandwidth to the Kubernetes API server. It also allows users to fine-tune the permissions that
Argo CD needs to a cluster by preventing Argo CD from attempting to watch resources of that
group/kind.

#### gRPC-Web Support

The argocd CLI can be now configured to communicate to the Argo CD API server using gRPC-Web
(HTTP1.1) using a new CLI flag `--grpc-web`. This resolves some compatibility issues users were
experiencing with ingresses and gRPC (HTTP2), and should enable argocd CLI to work with virtually
any load balancer, ingress controller, or API gateway.

#### CLI features

Argo CD introduces some additional CLI commands:

* `argocd app edit APPNAME` - to edit an application spec using preferred EDITOR
* `argocd proj edit PROJNAME` - to edit an project spec using preferred EDITOR
* `argocd app patch APPNAME` - to patch an application spec
* `argocd app patch-resource APPNAME` - to patch a specific resource which is part of an application


### Breaking Changes

#### Label selector changes, dex-server rename

The label selectors for deployments were been renamed to use kubernetes common labels
(`app.kuberentes.io/name=NAME` instead of `app=NAME`). Since K8s deployment label selectors are
immutable, during an upgrade from v0.11 to v0.12, the old deployments should be deleted using
`--cascade=false` which allows the new deployments to be created without introducing downtime.
Once the new deployments are ready, the older replicasets can be deleted. Use the following
instructions to upgrade from v0.11 to v0.12 without introducing downtime:

```
# delete the deployments with cascade=false. this orphan the replicasets, but leaves the pods running
kubectl delete deploy --cascade=false argocd-server argocd-repo-server argocd-application-controller

# apply the new manifests and wait for them to finish rolling out
kubectl apply <new install manifests>
kubectl rollout status deploy/argocd-application-controller
kubectl rollout status deploy/argocd-repo-server
kubectl rollout status deploy/argocd-application-controller

# delete old replicasets which are using the legacy label
kubectl delete rs -l app=argocd-server
kubectl delete rs -l app=argocd-repo-server
kubectl delete rs -l app=argocd-application-controller

# delete the legacy dex-server which was renamed
kubectl delete deploy dex-server
```

#### Deprecation of spec.source.componentParameterOverrides

For declarative application specs, the `spec.source.componentParameterOverrides` field is now
deprecated in favor of application source specific config. They are replaced with new fields
specific to their respective config management. For example, a Helm application spec using the
legacy field:

```yaml
spec:
  source:
    componentParameterOverrides:
    - name: image.tag
      value: v1.2
```

should move to:

```yaml
spec:
  source:
    helm:
      parameters:
      - name: image.tag
        value: v1.2
```

Argo CD will automatically duplicate the legacy field values to the new locations (and vice versa)
as part of automatic migration. The legacy `spec.source.componentParameterOverrides` field will be
kept around for the v0.12 release (for migration purposes) and will be removed in the next Argo CD
release.

#### Removal of spec.source.environment and spec.source.valuesFiles

The `spec.source.environment` and `spec.source.valuesFiles` fields, which were deprecated in v0.11,
are now completely removed from the Application spec.


#### API/CLI compatibility

Due to API spec changes related to the deprecation of componentParameterOverrides, Argo CD v0.12
has a minimum client version of v0.12.0. Older CLI clients will be rejected.


### Changes since v0.11:
+ Improved UI
+ Custom Health Assessments (CRD Health)
+ Configuration Management Plugins
+ High Availability
+ Fuzzy Diff Logic
+ Resource Exclusions
+ gRPC-Web Support
+ CLI features
+ Additional prometheus metrics
+ Sample Grafana dashboard (#1277) (@hartman17)
+ Support for Kustomize 2
+ YAML/JSON/Jsonnet Directories can now be recursed
+ Support for Jsonnet external variables and top-level arguments
+ Optimized reconciliation performance for applications with very active resources (#1267)
+ Support a separate OAuth2 CLI clientID different from server (#1307)
+ argocd diff: only print to stdout, if there is a diff + exit code (#1288) (@marcb1)
+ Detection and handling of duplicated resource definitions (#1284)
+ Support kustomize apps with remote bases in private repos in the same host (#1264)
+ Support patching resource using REST API (#1186)
* Deprecate componentParameterOverrides in favor of source specific config (#1207)
* Support talking to Dex using local cluster address instead of public address (#1211)
* Use Recreate deployment strategy for controller (#1315)
* Honor os environment variables for helm commands (#1306) (@1337andre)
* Disable CGO_ENABLED for server/controller binaries (#1286)
* Documentation fixes and improvements (@twz123, @yann-soubeyrand, @OmerKahani, @dulltz)
- Fix CRD creation/deletion handling (#1249)
- Git cloning via SSH was not verifying host public key (#1276)
- Fixed multiple goroutine leaks in controller and api-server
- Fix isssue where `argocd app set -p` required repo privileges. (#1280)
- Fix local diff of non-namespaced resources. Also handle duplicates in local diff (#1289)
- Deprecated resource kinds from 'extensions' groups are not reconciled correctly (#1232)
- Fix issue where CLI would panic after timeout when cli did not have get permissions (#1209)
- invalidate repo cache on delete (#1182) (@narg95)

## v0.11.2 (2019-02-19)
+ Adds client retry. Fixes #959 (#1119)
- Prevent deletion hotloop (#1115)
- Fix EncodeX509KeyPair function so it takes in account chained certificates (#1137) (@amarruedo)
- Exclude metrics.k8s.io from watch (#1128)
- Fix issue where dex restart could cause login failures (#1114)
- Relax ingress/service health check to accept non-empty ingress list (#1053)
- [UI] Correctly handle empty response from repository/<repo>/apps API

## v0.11.1 (2019-01-18)
+ Allow using redis as a cache in repo-server (#1020)
- Fix controller deadlock when checking for stale cache (#1044)
- Namespaces are not being sorted during apply (#1038)
- Controller cache was susceptible to clock skew in managed cluster
- Fix ability to unset ApplicationSource specific parameters
- Fix force resource delete API (#1033)
- Incorrect PermissionDenied error during app creation when using project roles + user-defined RBAC (#1019)
- Fix `kubctl convert` issue preventing deployment of extensions/NetworkPolicy (#1012)
- Do not allow metadata.creationTimestamp to affect sync status (#1021)
- Graceful handling of clusters where API resource discovery is partially successful (#1018)
- Handle k8s resources circular dependency (#1016)
- Fix `app diff --local` command (#1008)

## v0.11.0 (2019-01-10)
This is Argo CD's biggest release ever and introduces a completely redesigned controller architecture.

### New Features

#### Performance & Scalability
The application controller has a completely redesigned architecture which improved performance and
scalability during application reconciliation.

This was achieved by introducing an in-memory, live state cache of lightweight Kubernetes object 
metadata. During reconciliation, the controller no longer performs expensive, in-line queries of app
related resources in K8s API server, instead relying on the metadata available in the live state 
cache. This dramatically improves performance and responsiveness, and is less burdensome to the K8s
API server.

#### Object relationship visualization for CRDs
With the new controller design, Argo CD is now able to understand ownership relationship between
*all* Kubernetes objects, not just the built-in types. This enables Argo CD to visualize
parent/child relationships between all kubernetes objects, including CRDs.

#### Multi-namespaced applications
During sync, Argo CD will now honor any explicitly set namespace in a manifest. Manifests without a
namespace will continue deploy to the "preferred" namespace, as specified in app's
`spec.destination.namespace`. This enables support for a class of applications which install to
multiple namespaces. For example, Argo CD can now install the
[prometheus-operator](https://github.com/helm/charts/tree/master/stable/prometheus-operator)
helm chart, which deploys some resources into `kube-system`, and others into the
`prometheus-operator` namespace.

#### Large application support
Full resource objects are no longer stored in the Application CRD object status. Instead, only
lightweight metadata is stored in the status, such as a resource's sync and health status.
This change enabled Argo CD to support applications with a very large number of resources 
(e.g. istio), and reduces the bandwidth requirements when listing applications in the UI.

#### Resource lifecycle hook improvements
Resource lifecycle hooks (e.g. PreSync, PostSync) are now visible/manageable from the UI.
Additionally, bare Pods with a restart policy of Never can now be used as a resource hook, as an
alternative to Jobs, Workflows.

#### K8s recommended application labels
The tracking label for resources has been changed to use `app.kubernetes.io/instance`, as
recommended in [Kubernetes recommended labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/),
(changed from `applications.argoproj.io/app-name`). This will enable applications managed by Argo CD
to interoperate with other tooling which are also converging on this labeling, such as the
Kubernetes dashboard. Additionally, Argo CD no longer injects any tracking labels at the
`spec.template.metadata` level.

#### External OIDC provider support
Argo CD now supports auth delegation to an existing, external OIDC providers without the need for
running Dex (e.g. Okta, OneLogin, Auth0, Microsoft, etc...)

The optional, [Dex IDP OIDC provider](https://github.com/dexidp/dex) is still bundled as part of the
default installation, in order to provide a seamless out-of-box experience, enabling Argo CD to
integrate with non-OIDC providers, and to benefit from Dex's full range of
[connectors](https://github.com/dexidp/dex/tree/master/Documentation/connectors).

#### OIDC group bindings to Project Roles
OIDC group claims from an OAuth2 provider can now be bound to a Argo CD project roles. Previously,
group claims could only be managed in the centralized ConfigMap, `argocd-rbac-cm`. They can now be
managed at a project level. This enables project admins to self service access to applications
within a project.

#### Declarative Argo CD configuration
Argo CD settings can be now be configured either declaratively, or imperatively. The `argocd-cm`
ConfigMap now has a `repositories` field, which can reference credentials in a normal Kubernetes
secret which you can create declaratively, outside of Argo CD.

#### Helm repository support
Helm repositories can be configured at the system level, enabling the deployment of helm charts
which have a dependency to external helm repositories.

### Breaking changes:

* Argo CD's resource names were renamed for consistency. For example, the application-controller
  deployment was renamed to argocd-application-controller. When upgrading from v0.10 to v0.11,
  the older resources should be pruned to avoid inconsistent state and controller in-fighting.

* As a consequence to moving to recommended kubernetes labels, when upgrading from v0.10 to v0.11,
  all applications will immediately be OutOfSync due to the change in tracking labels. This will
  correct itself with another sync of the application. However, since Pods will be recreated, please
  take this into consideration, especially if your applications are configured with auto-sync.

* There was significant reworking of the `app.status` fields to reduce the payload size, simplify
  the datastructure and remove fields which were no longer used by the controller. No breaking
  changes were made in `app.spec`.

* An older Argo CD CLI (v0.10 and below) will not be compatible with Argo CD v0.11. To keep
  CI pipelines in sync with the API server, it is recommended to have pipelines download the CLI
  directly from the API server https://${ARGOCD_SERVER}/download/argocd-linux-amd64 during the CI
  pipeline.

### Changes since v0.10:
* Improve Application state reconciliation performance (#806)
* Refactor, consolidate and rename resource type data structures
+ Declarative setup and configuration of ArgoCD (#536)
+ Declaratively add helm repositories (#747)
+ Switch to k8s recommended app.kubernetes.io/instance label (#857)
+ Ability for a single application to deploy into multiple namespaces (#696)
+ Self service group access to project applications (#742)
+ Support for Pods as a sync hook (#801)
+ Support 'crd-install' helm hook (#355)
+ Use external 'diff' utility to render actual vs target state difference
+ Show sync policy in app list view
* Remove resources state from application CRD (#758)
* API server & UI should serve argocd binaries instead of linking to GitHub (#716)
* Update versions for kubectl (v1.13.1), helm (v2.12.1), ksonnet (v0.13.1)
* Update version of aws-iam-authenticator (0.4.0-alpha.1)
* Ability to force refresh of application manifests from git
* Improve diff assessment for Secrets, ClusterRoles, Roles
- Failed to deploy helm chart with local dependencies and no internet access (#786)
- Out of sync reported if Secrets with stringData are used (#763)
- Unable to delete application in K8s v1.12 (#718)

## v0.10.6 (2018-11-14)
- Fix issue preventing in-cluster app sync due to go-client changes (issue #774)

## v0.10.5 (2018-11-13)
+ Increase concurrency of application controller
* Update dependencies to k8s v1.12 and client-go v9.0 (#729)
- add argo cluster permission to view logs (#766) (@conorfennell)
- Fix issue where applications could not be deleted on k8s v1.12
- Allow 'syncApplication' action to reference target revision rather then hard-coding to 'HEAD' (#69) (@chrisgarland)
- Issue #768 - Fix application wizard crash

## v0.10.4 (2018-11-07)
* Upgrade to Helm v0.11.0 (@amarrella)
- Health check is not discerning apiVersion when assessing CRDs (issue #753)
- Fix nil pointer dereference in util/health (@mduarte)

## v0.10.3 (2018-10-28)
* Fix applying TLS version settings
* Update to kustomize 1.0.10 (@twz123)

## v0.10.2 (2018-10-25)
* Update to kustomize 1.0.9 (@twz123)
- Fix app refresh err when k8s patch is too slow

## v0.10.1 (2018-10-24)

- Handle case where OIDC settings become invalid after dex server restart (issue #710)
- git clean also needs to clean files under gitignore (issue #711)

## v0.10.0 (2018-10-19)

### Changes since v0.9:

+ Allow more fine-grained sync (issue #508)
+ Display init container logs (issue #681)
+ Redirect to /auth/login instead of /login when SSO token is used for authenticaion (issue #348)
+ Support ability to use a helm values files from a URL (issue #624)
+ Support public not-connected repo in app creation UI (issue #426)
+ Use ksonnet CLI instead of ksonnet libs (issue #626)
+ We should be able to select the order of the `yaml` files while creating a Helm App (#664)
* Remove default params from app history (issue #556)
* Update to ksonnet v0.13.0
* Update to kustomize 1.0.8
- API Server fails to return apps due to grpc max message size limit  (issue #690)
- App Creation UI for Helm Apps shows only files prefixed with `values-` (issue #663)
- App creation UI should allow specifying values files outside of helm app directory bug (issue #658)
- argocd-server logs credentials in plain text when adding git repositories (issue #653)
- Azure Repos do not work as a repository (issue #643)
- Better update conflict error handing during app editing (issue #685)
- Cluster watch needs to be restarted when CRDs get created (issue #627)
- Credentials not being accepted for Google Source Repositories (issue #651)
- Default project is created without permission to deploy cluster level resources (issue #679)
- Generate role token click resets policy changes (issue #655)
- Input type text instead of password on Connect repo panel (issue #693)
- Metrics endpoint not reachable through the metrics kubernetes service (issue #672)
- Operation stuck in 'in progress' state if application has no resources (issue #682)
- Project should influence options for cluster and namespace during app creation (issue #592)
- Repo server unable to execute ls-remote for private repos (issue #639)
- Resource is always out of sync if it has only 'ksonnet.io/component' label (issue #686)
- Resource nodes are 'jumping' on app details page (issue #683)
- Sync always suggest using latest revision instead of target UI bug (issue #669)
- Temporary ignore service catalog resources (issue #650)

## v0.9.2 (2018-09-28)

* Update to kustomize 1.0.8
- Fix issue where argocd-server logged credentials in plain text during repo add (issue #653)
- Credentials not being accepted for Google Source Repositories (issue #651)
- Azure Repos do not work as a repository (issue #643)
- Temporary ignore service catalog resources (issue #650)
- Normalize policies by always adding space after comma

## v0.9.1 (2018-09-24)

- Repo server unable to execute ls-remote for private repos (issue #639)

## v0.9.0 (2018-09-24)

### Notes about upgrading from v0.8
* Cluster wide resources should be allowed in default project (due to issue #330):

```
argocd project allow-cluster-resource default '*' '*'
```

* Projects now provide the ability to allow or deny deployments of cluster-scoped resources
(e.g. Namespaces, ClusterRoles, CustomResourceDefinitions). When upgrading from v0.8 to v0.9, to
match the behavior of v0.8 (which did not have restrictions on deploying resources) and continue to
allow deployment of cluster-scoped resources, an additional command should be run:

```bash
argocd proj allow-cluster-resource default '*' '*'
```

The above command allows the `default` project to deploy any cluster-scoped resources which matches
the behavior of v0.8.

* The secret keys in the argocd-secret containing the TLS certificate and key, has been renamed from
  `server.crt` and `server.key` to the standard `tls.crt` and `tls.key` keys. This enables Argo CD
  to integrate better with Ingress and cert-manager. When upgrading to v0.9, the `server.crt` and
  `server.key` keys in argocd-secret should be renamed to the new keys.

### Changes since v0.8:
+ Auto-sync option in application CRD instance (issue #79)
+ Support raw jsonnet as an application source (issue #540)
+ Reorder K8s resources to correct creation order (issue #102)
+ Redact K8s secrets from API server payloads (issue #470)
+ Support --in-cluster authentication without providing a kubeconfig (issue #527)
+ Special handling of CustomResourceDefinitions (issue #613)
+ Argo CD should download helm chart dependencies (issue #582)
+ Export Argo CD stats as prometheus style metrics (issue #513)
+ Support restricting TLS version (issue #609)
+ Use 'kubectl auth reconcile' before 'kubectl apply' (issue #523)
+ Projects need controls on cluster-scoped resources (issue #330)
+ Support IAM Authentication for managing external K8s clusters (issue #482)
+ Compatibility with cert manager (issue #617)
* Enable TLS for repo server (issue #553)
* Split out dex into it's own deployment (instead of sidecar) (issue #555)
+ [UI] Support selection of helm values files in App creation wizard (issue #499)
+ [UI] Support specifying source revision in App creation wizard allow (issue #503)
+ [UI] Improve resource diff rendering (issue #457)
+ [UI] Indicate number of ready containers in pod (issue #539)
+ [UI] Indicate when app is overriding parameters (issue #503)
+ [UI] Provide a YAML view of resources (issue #396)
+ [UI] Project Role/Token management from UI (issue #548)
+ [UI] App creation wizard should allow specifying source revision (issue #562)
+ [UI] Ability to modify application from UI (issue #615)
+ [UI] indicate when operation is in progress or has failed (issue #566)
- Fix issue where changes were not pulled when tracking a branch (issue #567)
- Lazy enforcement of unknown cluster/namespace restricted resources (issue #599)
- Fix controller hot loop when app source contains bad manifests (issue #568)
- Fix issue where Argo CD fails to deploy when resources are in a K8s list format (issue #584)
- Fix comparison failure when app contains unregistered custom resource (issue #583)
- Fix issue where helm hooks were being deployed as part of sync (issue #605)
- Fix race conditions in kube.GetResourcesWithLabel and DeleteResourceWithLabel (issue #587)
- [UI] Fix issue where projects filter does not work when application got changed
- [UI] Creating apps from directories is not obvious (issue #565)
- Helm hooks are being deployed as resources (issue #605)
- Disagreement in three way diff calculation (issue #597)
- SIGSEGV in kube.GetResourcesWithLabel (issue #587)
- Argo CD fails to deploy resources list (issue #584)
- Branch tracking not working properly (issue #567)
- Controller hot loop when application source has bad manifests (issue #568)

## v0.8.2 (2018-09-12)
- Downgrade ksonnet from v0.12.0 to v0.11.0 due to quote unescape regression
- Fix CLI panic when performing an initial `argocd sync/wait`

## v0.8.1 (2018-09-10)
+ [UI] Support selection of helm values files in App creation wizard (issue #499)
+ [UI] Support specifying source revision in App creation wizard allow (issue #503)
+ [UI] Improve resource diff rendering (issue #457)
+ [UI] Indicate number of ready containers in pod (issue #539)
+ [UI] Indicate when app is overriding parameters (issue #503)
+ [UI] Provide a YAML view of resources (issue #396)
- Fix issue where changes were not pulled when tracking a branch (issue #567)
- Fix controller hot loop when app source contains bad manifests (issue #568)
- [UI] Fix issue where projects filter does not work when application got changed

## v0.8.0 (2018-09-04)

### Notes about upgrading from v0.7
* The RBAC model has been improved to support explicit denies. What this means is that any previous
RBAC policy rules, need to be rewritten to include one extra column with the effect:
`allow` or `deny`. For example, if a rule was written like this:
    ```
    p, my-org:my-team, applications, get, */*
    ```
    It should be rewritten to look like this:
    ```
    p, my-org:my-team, applications, get, */*, allow
    ```

### Changes since v0.7:
+ Support kustomize as an application source (issue #510)
+ Introduce project tokens for automation access (issue #498)
+ Add ability to delete a single application resource to support immutable updates (issue #262)
+ Update RBAC model to support explicit denies (issue #497)
+ Ability to view Kubernetes events related to application projects for auditing
+ Add PVC healthcheck to controller (issue #501)
+ Run all containers as an unprivileged user (issue #528)
* Upgrade ksonnet to v0.12.0
* Add readiness probes to API server (issue #522)
* Use gRPC error codes instead of fmt.Errorf (#532)
- API discovery becomes best effort when partial resource list is returned (issue #524)
- Fix `argocd app wait` printing incorrect Sync output (issue #542)
- Fix issue where argocd could not sync to a tag (#541)
- Fix issue where static assets were browser cached between upgrades (issue #489)

## v0.7.2 (2018-08-21)
- API discovery becomes best effort when partial resource list is returned (issue #524)

## v0.7.1 (2018-08-03)
+ Surface helm parameters to the application level (#485)
+ [UI] Improve application creation wizard (#459)
+ [UI] Show indicator when refresh is still in progress (#493)
* [UI] Improve data loading error notification (#446)
* Infer username from claims during an `argocd relogin` (#475)
* Expand RBAC role to be able to create application events. Fix username claims extraction
- Fix scalability issues with the ListApps API (#494)
- Fix issue where application server was retrieving events from incorrect cluster (#478)
- Fix failure in identifying app source type when path was '.'
- AppProjectSpec SourceRepos mislabeled (#490)
- Failed e2e test was not failing CI workflow
* Fix linux download link in getting_started.md (#487) (@chocopowwwa)

## v0.7.0 (2018-07-27)
+ Support helm charts and yaml directories as an application source
+ Audit trails in the form of API call logs
+ Generate kubernetes events for application state changes
+ Add ksonnet version to version endpoint (#433)
+ Show CLI progress for sync and rollback
+ Make use of dex refresh tokens and store them into local config
+ Expire local superuser tokens when their password changes
+ Add `argocd relogin` command as a convenience around login to current context
- Fix saving default connection status for repos and clusters
- Fix undesired fail-fast behavior of health check
- Fix memory leak in the cluster resource watch
- Health check for StatefulSets, DaemonSet, and ReplicaSets were failing due to use of wrong converters

## v0.6.2 (2018-07-23)
- Health check for StatefulSets, DaemonSet, and ReplicaSets were failing due to use of wrong converters

## v0.6.1 (2018-07-18)
- Fix regression where deployment health check incorrectly reported Healthy
+ Intercept dex SSO errors and present them in Argo login page

## v0.6.0 (2018-07-16)
+ Support PreSync, Sync, PostSync resource hooks
+ Introduce Application Projects for finer grain RBAC controls
+ Swagger Docs & UI
+ Support in-cluster deployments internal kubernetes service name
+ Refactoring & Improvements
* Improved error handling, status and condition reporting
* Remove installer in favor of kubectl apply instructions
* Add validation when setting application parameters
* Cascade deletion is decided during app deletion, instead of app creation
- Fix git authentication implementation when using using SSH key
- app-name label was inadvertently injected into spec.selector if selector was omitted from v1beta1 specs

## v0.5.4 (2018-06-27)
- Refresh flag to sync should be optional, not required

## v0.5.3 (2018-06-20)
+ Support cluster management using the internal k8s API address https://kubernetes.default.svc (#307)
+ Support diffing a local ksonnet app to the live application state (resolves #239) (#298)
+ Add ability to show last operation result in app get. Show path in app list -o wide (#297)
+ Update dependencies: ksonnet v0.11, golang v1.10, debian v9.4 (#296)
+ Add ability to force a refresh of an app during get (resolves #269) (#293)
+ Automatically restart API server upon certificate changes (#292)

## v0.5.2 (2018-06-14)
+ Resource events tab on application details page (#286)
+ Display pod status on application details page (#231)

## v0.5.1 (2018-06-13)
- API server incorrectly compose application fully qualified name for RBAC check (#283)
- UI crash while rendering application operation info if operation failed

## v0.5.0 (2018-06-12)
+ RBAC access control
+ Repository/Cluster state monitoring
+ Argo CD settings import/export
+ Application creation UI wizard
+ argocd app manifests for printing the application manifests
+ argocd app unset command to unset parameter overrides
+ Fail app sync if prune flag is required (#276)
+ Take into account number of unavailable replicas to decided if deployment is healthy or not #270
+ Add ability to show parameters and overrides in CLI (resolves #240)
- Repo names containing underscores were not being accepted (#258)
- Cookie token was not parsed properly when mixed with other site cookies

## v0.4.7 (2018-06-07)
- Fix argocd app wait health checking logic

## v0.4.6 (2018-06-06)
- Retry argocd app wait connection errors from EOF watch. Show detailed state changes

## v0.4.5 (2018-05-31)
+ Add argocd app unset command to unset parameter overrides
- Cookie token was not parsed properly when mixed with other site cookies

## v0.4.4 (2018-05-30)
+ Add ability to show parameters and overrides in CLI (resolves #240)
+ Add Events API endpoint
+ Issue #238 - add upsert flag to 'argocd app create' command
+ Add repo browsing endpoint (#229)
+ Support subscribing to settings updates and auto-restart of dex and API server
- Issue #233 - Controller does not persist rollback operation result
- App sync frequently fails due to concurrent app modification

## v0.4.3 (2018-05-21)
- Move local branch deletion as part of git Reset() (resolves #185) (#222)
- Fix exit code for app wait (#219)

## v0.4.2 (2018-05-21)
+ Show URL in argocd app get
- Remove interactive context name prompt during login which broke login automation
* Rename force flag to cascade in argocd app delete

## v0.4.1 (2018-05-18)
+ Implemented argocd app wait command

## v0.4.0 (2018-05-17)
+ SSO Integration
+ GitHub Webhook
+ Add application health status
+ Sync/Rollback/Delete is asynchronously handled by controller
* Refactor CRUD operation on clusters and repos
* Sync will always perform kubectl apply
* Synced Status considers last-applied-configuration annotatoin
* Server & namespace are mandatory fields (still inferred from app.yaml)
* Manifests are memoized in repo server
- Fix connection timeouts to SSH repos

## v0.3.2 (2018-05-03)
+ Application sync should delete 'unexpected' resources #139
+ Update ksonnet to v0.10.1
+ Detect unexpected resources
- Fix: App sync frequently fails due to concurrent app modification #147
- Fix: improve app state comparator: #136, #132

## v0.3.1 (2018-04-24)
+ Add new rollback RPC with numeric identifiers
+ New argo app history and argo app rollback command
+ Switch to gogo/protobuf for golang code generation
- Fix: create .argocd directory during argo login (issue #123)
- Fix: Allow overriding server or namespace separately (issue #110)

## v0.3.0 (2018-04-23)
+ Auth support
+ TLS support
+ DAG-based application view
+ Bulk watch
+ ksonnet v0.10.0-alpha.3
+ kubectl apply deployment strategy
+ CLI improvements for app management

## v0.2.0 (2018-04-03)
+ Rollback UI
+ Override parameters

## v0.1.0 (2018-03-12)
+ Define app in Github with dev and preprod environment using KSonnet
+ Add cluster Diff App with a cluster Deploy app in a cluster
+ Deploy a new version of the app in the cluster
+ App sync based on Github app config change - polling only
+ Basic UI: App diff between Git and k8s cluster for all environments Basic GUI

# Changelog

## v1.3.2 (2019-12-03)

#### Pull Requests

- #2696 Revert "Use Kustomize 3 to generate manifetsts. Closes #2487 (#2510)"
- #2797 Fix directory traversal edge case and enhance tests

#### Contributors

* Alex Collins <!-- num=3 -->
* Simon Behar <!-- num=1 -->

See also [milestone v1.3](https://github.com/argoproj/argo-cd/milestone/15?closed=1)

## v1.3.1 (2019-12-02)

#### Bug Fixes

- #2664 update account password from API resulted 404
- #2724 Can't use `DNS-1123` compliant app name when creating project role
- #2726 App list does not show chart for Helm app
- #2741 argocd local sync cannot parse kubernetes version
- #2754 BeforeHookCreation should be the default hook
- #2767 Fix bug whereby retry does not work for CLI
- #2770 Always cache miss for manifests

#### Other

- #1345 argocd-application-controller: can not retrieve list of objects using index : Index with name namespace does not exist

#### Pull Requests

- #2701 Adds support for /api/v1/account* via HTTP. Fixes #2664
- #2716 Make directory enforcer more lenient and add flag
- #2728 Shows chart name in apps tiles and apps table pages. Closes #2726
- #2747 Allow you to sync local Helm apps.  Fixes #2741
- #2755 Allow dot in project policy. Closes #2724
- #2759 Make BeforeHookCreation the default. Fixes #2754
- #2761 Removes log warning regarding indexer and may improve perf. Closes #1345
- #2768 Fixes bug whereby retry does not work for CLI. Fixes #2767
- #2771 Fix bug where manifests are not cached. Fixes #2770

#### Contributors

* Alex Collins <!-- num=11 -->
* Simon Behar <!-- num=1 -->

See also [milestone v1.3](https://github.com/argoproj/argo-cd/milestone/15?closed=1)

## v1.3.0 (2019-11-13)

#### New Features

##### Helm 1st-Class Support

We know that for many of our users, they want to deploy existing Helm charts using Argo CD. Up until now that has required you to create an Argo CD app in a Git repo that does nothing but point to that chart. Now you can use a Helm chart repository is the same way as a Git repository.

On top of that, we've improved support for Helm apps. The most common types of Helm hooks such as `pre-install` and `post-install` are supported as well as a the delete policy `before-hook-creation` which makes it easier to work with hooks.

https://youtu.be/GP7xtrnNznw

##### Orphan Resources

Some users would like to make sure that resources in a namespace are managed only by Argo CD. So we've introduced the concept of an "orphan resource" - any resource that is in namespace associated with an app, but not managed by Argo CD. This is enabled in the project settings. Once enabled, Argo CD will show in the app view any resources in the app's namepspace that is not mananged by Argo CD. 

https://youtu.be/9ZoTevVQf5I

##### Sync Windows

There may be instances when you want to control the times during which an Argo CD app can sync. Sync Windows now gives you the capability to create windows of time in which apps are either allowed or denied the ability to sync. This can apply to both manual and auto-sync, or just auto-sync. The windows are configured at the project level and assigned to apps using app name, namespace or cluster. Wildcards are supported for all fields.

#### Enhancements

* #1099 [UI] Add application labels to Applications list and Applications details page
* #1145 Helm repository as first class Argo CD Application source
* #1167 Ability to generate a warn/alert when a namespace deviates from the expected state
* #1615 Improve diff support for resource requests/limits
* #1642 HTTP API should allow JWT to be passed via Authorization header
* #1852 Ability to create & upsert projects from spec
* #1930 Support for in-line block from helm chart values
* #1956 Request OIDC groups claim if groups scope is not supported
* #1995 Add a maintenance window for Applications with automated syncing
* #2036 Support `argocd.argoproj.io/hook-delete-policy: BeforeHookCreation`
* #2078 Support setting Helm string parameters using CLI/UI
* #2203 Config management plugin environment variable UI/CLI support
* #2260 Helm: auto-detect URLs
* #2261 Helm: UI improvements
* #2275 Support `helm template --kube-version `
* #2277 Use community icons for resources
* #2298 Make `group` optional for `ignoreDifferences` config
* #2315 Update Helm docs
* #2354 Add cluster information into Splunk
* #2396 argocd list command should have filter options like by project
* #2445 Add target/current revision to status badge
* #2487 Update tooling to use Kustomize v3
* #2488 Update root `Dockerfile` to use the `hack/install.sh`
* #2559 Support and document using HPA for repo-server
* #2587 Upgrade Helm 
* #2604 UI fixes for "Sync Apps" panel.
* #2609 Upgrade kustomize from v3.1.0 to v3.2.1
* #355 Map helm lifecycle hooks to ArgoCD pre/post/sync hooks

#### Bug Fixes

- #1660 failed parsing on parameters with comma
- #1881 Statefuleset with OnDelete Update Strategy stuck progressing
- #1923 Warning during secret diffing
- #1944 Error message "Unable to load data: key is missing" is confusing
- #2006 OIDC group bindings are truncated
- #2022 Multiple parallel app syncs causes OOM
- #2046 Unknown error when setting params with argocd app set on helm app
- #2060 Endpoint is no longer shown as a child of services
- #2099 SSH known hosts entry cannot be deleted if contains shell pattern in name
- #2114 Application 404s on names with periods
- #2116 Adding certs for hostnames ending with a dot (.) is not possible
- #2141 Fix `TestHookDeleteBeforeCreation`
- #2146 v1.2.0-rc1 nil pointer dereference when syncing
- #2150 Replacing services failure
- #2152 1.2.0-rc1 - Authentication Required error in Repo Server
- #2174 v1.2.0-rc1 Applications List View doesn't work
- #2185 Manual sync does not trigger Presync hooks
- #2192 SyncError app condition disappears during app reconciliation
- #2198 argocd app wait\sync prints 'Unknown' for resources without health
- #2206 1.2.0-rc2 Warning during secret diffing
- #2212 SSO redirect url is incorrect if configured Argo CD URL has trailing slash
- #2215 Application summary diff page shows hooks
- #2216 An app with a single resource and Sync hook remains progressing
- #2231 CONTRIBUTING documentation outdated
- #2243 v1.2.0-rc2 does not retrieve http(s) based git repository behind the proxy
- #2245 Intermittent "git ls-remote" request failures should not fail app reconciliation
- #2263 Result of ListApps operation for Git repo is cached incorrectly
- #2287 ListApps does not utilize cache
- #2290 Controller panics due to nil pointer error
- #2303 The Helm --kube-version support does not work on GKE:
- #2308 Fixes bug that prevents you creating repos via UI/CLI. 
- #2316 The 'helm.repositories' settings is dropped without migration path
- #2317 Badge response does not contain cache control header
- #2321 Inconsistent sync result from UI and CLI
- #2330 Failed edit application with plugin type requiring environment
- #2339 AutoSync doesn't work anymore
- #2371 End-to-End tests not working with Kubernetes v1.16
- #2378 Creating an application from Helm repository should select "Helm" as source type
- #2386 The parameters of ValidateAccess GRPC method should not be logged 
- #2398 Maintenance window meaning is confusing
- #2407 UI bug when targetRevision is ommited
- #2425 Too many vulnerabilities in Docker image
- #2443 proj windows commands not consistent with other commands
- #2448 Custom resource actions cannot be executed from the UI
- #2453 Application controller sometimes accidentally removes duplicated/excluded resource warning condition
- #2455 Logic that checks sync windows state in the cli is incorrect
- #2475 UI don't allow to create window with `* * * * *` schedule
- #2480 Helm Hook is executed twice if annotated with both pre-install and pre-upgrade annotations
- #2484 Impossible to edit chart name using App details page
- #2496 ArgoCD does not provide CSRF protection
- #2497 ArgoCD failing to install CRDs in master from Helm Charts
- #2549 Timestamp in Helm package file name causes error in Application with Helm source
- #2567 Attempting to create a repo with password but not username panics
- #2577 UI incorrectly mark resources as `Required Pruning`
- #2616 argocd app diff prints only first difference
- #2619 Bump min client cache version
- #2620 Cluster list page fails if any cluster is not reachable
- #2622 Repository type should be mandatory for repo add command in CLI
- #2626 Repo server executes unnecessary ls-remotes
- #2633 Application list page incorrectly filter apps by label selector
- #2635 Custom actions are disabled in Argo CD UI
- #2645 Failure of `argocd version` in the self-building container image
- #2655 Application list page is not updated automatically anymore
- #2659 Login regression issues
- #2662 Regression: Cannot return Kustomize version for 3.1.0
- #2670 API server does not allow creating role with action `action/*`
- #2673 Application controller `kubectl-parallelism-limit` flag is broken
- #2691 Annoying toolbar flickering

#### Other

- #1059 [UI] Enhance app creation page with Helm parameters overrides
- #1103 Deal with 4KB cookie limit for JWT
- #2086 Fix `TestAutoSyncSelfHealEnabled`
- #2124 Add configurable help link to every page
- #2272 e2e tests timing out after 10m
- Fix lint and merge issues
- Fixes merge issue
- Merge test from master
- Update manifests to v1.3.0
- Update manifests to v1.3.0-rc1
- Update manifests to v1.3.0-rc2
- Update manifests to v1.3.0-rc3
- Update manifests to v1.3.0-rc4
- Update manifests to v1.3.0-rc5
- codegen

#### Pull Requests

- #1805 Allow list actions to return yaml or json
- #2101 Fix and enhance end-to-end testing for SSH repositories
- #2110 RBAC Support for Actions
- #2111 Correct some broken links in yaml
- #2122 Add FuturePLC to List of companies using ArgoCD
- #2127 Updates hook delete policy docs
- #2130 Update rbac.md
- #2131 Update faq.md
- #2132 Minor CLI bug fixes
- #2133 Adds checks around valid paths for apps
- #2134 Enhances cookie warning with actual length to help users fix their co…
- #2135 Ignore generated code coverage
- #2145 Update helm.md
- #2149 Indicate that `SyncFail` hooks are on v1.2+
- #2153 Added 'SyncFail' to possible HookTypes in UI
- #2158 Update broken link
- #2159 Updates app-of-apps docs
- #2160 Added more health filters in UI
- #2161 Create "argocd" ns in `make start`
- #2162 Fixed routing issue for periods
- #2164 Better detection for authorization_code OIDC response type
- #2166 Determine the manifest version from the VERSION file when on release …
- #2172 Temporary disable Git LFS test to unblock release
- #2187 Add missing labels to configmap/secret in documentation
- #2189 Add missing labels to argocd-cm yaml in kustomize.md
- #2190 FAQ: Simplify admin password snippet a bit
- #2195 Codegen
- #2196 Remove duplicated DoNotIgnoreErrors method
- #2208 Add path to externalURLs
- #2210 Fix flaky TestOrphanedResource test
- #2213 Fix TestImmutableChange for running locally in microk8s
- #2222 Fix JS crash in EditablePanel component
- #2229 Fix/grafana datasources
- #2230 Alter wording in Ingress docs to be more natural
- #2232 Grammar fixes.
- #2233 docs/user-guide/projects.md: fix example policy
- #2240 Fix typo.
- #2242 Update bug_report.md
- #2244 codegen
- #2247 Improve build stability
- #2252 Add v1.2 Changelog
- #2254 codegen
- #2265 Fix typo
- #2266 Change Helm repo URLs to argoproj/argo-cd/master
- #2279 Adding information to make local execution more accessible
- #2282 Fix TestAutoSyncSelfHealEnabled test flakiness
- #2283 Update OWNERS
- #2293 Fix TestAutoSyncSelfHealEnabled test flakiness
- #2296 Add --self-heal flag to argocd cli
- #2300 Add Deployment action for `kubectl rollout restart`
- #2307 Fixes bug in `argocd repo list` and tidy up UI
- #2312 Document flags/env variables useful for performance tuning
- #2313 Fix broken links
- #2314 Add helm.repositories back to documentation
- #2319 Fix docker image for dev
- #2324 Who uses ArgoCD: Add Lytt to the list
- #2326 util/localconfig: prefer HOME env var over os/user
- #2329 Added Kustomize, Helm, and Kubectl to `argocd version`
- #2332 Detypo architecture doc
- #2336 chore(dashboard.json): adding argocd project as variable to grafana d…
- #2341 docs: improve sso oidc documentation regarding client secret
- #2342 Don't fix imports in auto-generated files
- #2343 Codegen
- #2344 Adds support for Github Enterprise URLs
- #2345 Fixes display of path in UI
- #2362 Make argo-cd docker images openshift friendly
- #2365 Update api-docs.md
- #2375 Add custom action example to argocd-cm.yaml
- #2377 Refactor Helm client and unit test repo server
- #2385 Use configured certificate to access helm repository
- #2390 Add missing externalURL for networking.k8s.io Ingress type
- #2392 Adds Traefik v2 documentation to ingress options
- #2393 Fix broken helm test
- #2394 Update faq.md
- #2395 Add release 1.2.1~1.2.3 changelog
- #2400 Polish maintenance windows
- #2411 Stop unnecessary re-loading clusters on every app list page re-render
- #2414 Add a hook example for sending a Slack message
- #2419 App status panel shows metadata of current revision in git instead ofmost recent reconciled revision
- #2426 Fixes flakey test
- #2428 Detach ArgoCD from specific workflow API
- #2432 Fix JS error on application creation page if no plugins configured
- #2434 Error with new  `actions run` suggestion
- #2440 Fix flaky TestExcludedResource
- #2446 Fix formatting issues in docs
- #2452 Fix possible path traversal attack when supporting Helm `values.yaml`
- #2459 Hide sync windows if there are none
- #2467 add support for --additional-headers cli flag
- #2469 Allow collapse/expand helm values text
- #2470 Change "available" to "disabled" in actions, make them available by default
- #2474 Optimizes e2e tests
- #2478 Update issue and PR templates
- #2479 Optimize linting
- #2482 Optimize codegen
- #2486 Final optimisations
- #2489 Speedup codegen on mac
- #2490 Fix UI crash on application list page
- #2494 Update CONTRIBUTING.md
- #2499 Work-around golang cilint error
- #2538 Redact secrets in dex logs
- #2544 Unknown child app should not affect app health
- #2570 Adds timeout to Helm commands.
- #2579 Add AnalysisRun and Experiment HealthCheck
- #2605 Executes app label filtering on client side
- #2700 Restore 'argocd app run action' backward compatibility

#### Contributors

* Aalok Ahluwalia <!-- num=1 -->
* Aananth K <!-- num=1 -->
* Adam Johnson <!-- num=4 -->
* Alex Collins <!-- num=78 -->
* Alexander Matyushentsev <!-- num=80 -->
* Andrew Waters <!-- num=2 -->
* Ben Doyle <!-- num=1 -->
* Chris Jones <!-- num=1 -->
* David Hong <!-- num=1 -->
* Fred Dubois <!-- num=1 -->
* Gregor Krmelj <!-- num=2 -->
* Gustav Paul <!-- num=2 -->
* Isaac Gaskin <!-- num=1 -->
* Jesse Suen <!-- num=1 -->
* John Reese <!-- num=1 -->
* Luiz Fonseca <!-- num=1 -->
* Michael Bridgen <!-- num=1 -->
* Mitz Amano <!-- num=1 -->
* Olivier Boukili <!-- num=1 -->
* Olivier Lemasle <!-- num=1 -->
* Rayyis <!-- num=1 -->
* Rodolphe Prin <!-- num=1 -->
* Ryota <!-- num=2 -->
* Seiya Muramatsu <!-- num=1 -->
* Simon Behar <!-- num=12 -->
* Sverre Boschman <!-- num=1 -->
* Toby Jackson <!-- num=1 -->
* Tom Wieczorek <!-- num=3 -->
* Yujun Zhang <!-- num=4 -->
* Zoltán Reegn <!-- num=1 -->
* agabet <!-- num=1 -->
* dthomson25 <!-- num=3 -->
* jannfis <!-- num=10 -->
* ssbtn <!-- num=2 -->

See also [milestone v1.3](https://github.com/argoproj/argo-cd/milestone/15?closed=1)

## v1.2.5 (2019-10-29)

### Changes since v1.2.4

- Issue #2339 - Don't update 'status.reconciledAt' unless compared with latest git version

## v1.2.4 (2019-10-23)

### Changes since v1.2.3

- Issue #2185 - Manual sync don't trigger hooks (#2477)
- Issue #2339 - Controller should compare with latest git revision if app has changed (#2543)
- Unknown child app should not affect app health (#2544)
- Redact secrets in dex logs (#2538)

## v1.2.3 (2019-10-01)

### Changes since v1.2.2
* Make argo-cd docker images openshift friendly (#2362) (@duboisf)
* Add dest-server and dest-namespace field to reconciliation logs (#2354)

- Stop loggin /repository.RepositoryService/ValidateAccess parameters (#2386)

## v1.2.2 (2019-09-24)

### Changes since v1.2.1
+ Resource action equivalent to `kubectl rollout restart` (#2177)

- Badge response does not contain cache-control header (#2317) (@greenstatic)
- Make sure the controller uses the latest git version if app reconciliation result expired (#2339)

## v1.2.1 (2019-09-12)

### Changes since v1.2.0

+ Support limiting number of concurrent kubectl fork/execs (#2022)
+ Add --self-heal flag to argocd cli (#2296)

- Fix degraded proxy support for http(s) git repository (#2243)
- Fix nil pointer dereference in application controller (#2290)

## v1.2.0 (2019-09-04)

### New Features

#### Server Certificate And Known Hosts Management

The Server Certificate And Known Hosts Management feature makes it really easy to connect private Git repositories to Argo CD. Now Argo CD provides UI and CLI which
enables managing certificates and known hosts which are used to access Git repositories. It is also possible to configure both hosts and certificates in a declarative manner using
[argocd-ssh-known-hosts-cm](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/argocd-ssh-known-hosts-cm.yaml) and 
[argocd-tls-certs-cm.yaml](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/argocd-tls-certs-cm.yaml) config maps.

#### Self-Healing

The existing Automatic Sync feature allows to automatically apply any new changes in Git to the target Kubernetes cluster. However, Automatic Sync does not cover the case when the
application is out of sync due to the unexpected change in the target cluster. The Self-Healing feature fills this gap. With Self-Healing enabled Argo CD automatically pushes the desired state from Git into the cluster every time when state deviation is detected.

**Anonymous access** - enable read-only access without authentication to anyone in your organization.

Support for Git LFS enabled repositories - now you can store Helm charts as tar files and enable Git LFS in your repository.

**Compact diff view** - compact diff summary of the whole application in a single view.

**Badge for application status** - add badge with the health and sync status of your application into README.md of your deployment repo.

**Allow configuring google analytics tracking** - use Google Analytics to check how many users are visiting UI or your Argo CD instance.

#### Backward Incompatible Changes
- Kustomize v1 support is removed. All kustomize charts are built using the same Kustomize version
- Kustomize v2.0.3 upgraded to v3.1.0 . We've noticed one backward incompatible change: https://github.com/kubernetes-sigs/kustomize/issues/42 . Starting v2.1.0 namespace prefix feature works with CRD ( which might cause renaming of generated resource definitions)
- Argo CD config maps must be annotated with `app.kubernetes.io/part-of: argocd` label. Make sure to apply updated `install.yaml` manifest in addition to changing image version.


#### Enhancements
+ Adds a floating action button with help and chat links to every page.… (#2124)
+ Enhances cookie warning with actual length to help users fix their co… (#2134)
+ Added 'SyncFail' to possible HookTypes in UI (#2147)
+ Support for Git LFS enabled repositories (#1853)
+ Server certificate and known hosts management (#1514)
+ Client HTTPS certifcates for private git repositories (#1945)
+ Badge for application status (#1435)
+ Make the health check for APIService a built in (#1841)
+ Bitbucket Server and Gogs webhook providers (#1269)
+ Jsonnet TLA arguments in ArgoCD CLI (#1626)
+ Self Healing (#1736)
+ Compact diff view (#1831)
+ Allow Helm parameters to force ambiguously-typed values to be strings (#1846)
+ Support anonymous argocd access (#1620)
+ Allow configuring google analytics tracking (#738)
+ Bash autocompletion for argocd (#1798)
+ Additional commit metadata (#1219)
+ Displays targetRevision in app dashboards. (#1239)
+ Local path syncing (#839)
+ System level `kustomize build` options (#1789)
+ Adds support for `argocd app set` for Kustomize. (#1843)
+ Allow users to create tokens for projects where they have any role. (#1977)
+ Add Refresh button to applications table and card view (#1606)
+ Adds CLI support for adding and removing groups from project roles. (#1851)
+ Support dry run and hook vs. apply strategy during sync (#798)
+ UI should remember most recent selected tab on resource info panel (#2007)
+ Adds link to the project from the app summary page. (#1911)
+ Different icon for resources which require pruning (#1159)

#### Bug Fixes

- Do not panic if the type is not api.Status (an error scenario) (#2105)
- Make sure endpoint is shown as a child of service (#2060)
- Word-wraps app info in the table and list views. (#2004)
- Project source/destination removal should consider wildcards (#1780)
- Repo whitelisting in UI does not support wildcards (#2000)
- Wait for CRD creation during sync process (#1940)
- Added a button to select out of sync items in the sync panel (#1902)
- Proper handling of an excluded resource in an application (#1621)
- Stop repeating logs on stoped container (#1614)
- Fix git repo url parsing on application list view (#2174)
- Fix nil pointer dereference error during app reconciliation (#2146)
- Fix history api fallback implementation to support app names with dots (#2114)
- Fixes some code issues related to Kustomize build options. (#2146)
- Adds checks around valid paths for apps (#2133)
- Enpoint incorrectly considered top level managed resource (#2060)
- Allow adding certs for hostnames ending on a dot (#2116)

#### Other
* Upgrade kustomize to v3.1.0 (#2068)
* Remove support for Kustomize 1. (#1573)

#### Contributors

* [alexec](https://github.com/alexec)
* [alexmt](https://github.com/alexmt)
* [dmizelle](https://github.com/dmizelle)
* [lcostea](https://github.com/lcostea)
* [jutley](https://github.com/jutley)
* [masa213f](https://github.com/masa213f)
* [Rayyis](https://github.com/Rayyis)
* [simster7](https://github.com/simster7)
* [dthomson25](https://github.com/dthomson25)
* [jannfis](https://github.com/jannfis)
* [naynasiddharth](https://github.com/naynasiddharth)
* [stgarf](https://github.com/stgarf)

See also [milestone v1.2](https://github.com/argoproj/argo-cd/issues?q=is%3Aissue+milestone%3Av1.2+is%3Aclosed)

## v1.1.2 (2019-07-30)

### Changes since v1.1.1

-  'argocd app wait' should print correct sync status (#2049)
- Check that TLS is enabled when registering DEX Handlers (#2047)
- Do not ignore Argo hooks when there is a Helm hook. (#1952)

## v1.1.1 (2019-07-24)

### Changes since v1.1.0

- Support 'override' action in UI/API (#1984)
- Fix argocd app wait message (#1982)

## v1.1.0 (2019-07-24)

### New Features

#### Sync Waves

Sync waves feature allows executing a sync operation in a number of steps or waves. Within each synchronization phase (pre-sync, sync, post-sync) you can have one or more waves,
than allows you to ensure certain resources are healthy before subsequent resources are synced.

#### Optimized interaction with Git

Argo CD needs to execute `git fetch` operation to access application manifests and `git ls-remote` to resolve ambiguous git revision. The `git ls-remote` is executed very frequently
and although the operation is very lightweight it adds unnecessary load on Git server and might cause performance issues. In v1.1 release, the application reconciliation process was
optimized which significantly reduced the number of Git requests. With v1.1 release, Argo CD should send 3x ~ 5x fewer Git requests.

#### User defined Application metadata

User-defined Application metadata enables the user to define a list of useful URLs for their specific application and expose those links on the UI
(e.g. reference tp a CI pipeline or an application-specific management tool). These links should provide helpful shortcuts that make easier to integrate Argo CD into existing
systems by making it easier to find other components inside and outside Argo CD.

### Deprecation Notice

* Kustomize v1.0 is deprecated and support will be removed in the Argo CD v1.2 release.

#### Enhancements

- Sync waves [#1544](https://github.com/argoproj/argo-cd/issues/1544)
- Adds Prune=false and IgnoreExtraneous options [#1629](https://github.com/argoproj/argo-cd/issues/1629)
- Forward Git credentials to config management plugins [#1628](https://github.com/argoproj/argo-cd/issues/1628)
- Improve Kustomize 2 parameters UI [#1609](https://github.com/argoproj/argo-cd/issues/1609)
- Adds `argocd logout` [#1210](https://github.com/argoproj/argo-cd/issues/1210)
- Make it possible to set Helm release name different from Argo CD app name.  [#1066](https://github.com/argoproj/argo-cd/issues/1066)
- Add ability to specify system namespace during cluster add operation [#1661](https://github.com/argoproj/argo-cd/pull/1661) 
- Make listener and metrics ports configurable [#1647](https://github.com/argoproj/argo-cd/pull/1647) 
- Using SSH keys to authenticate kustomize bases from git [#827](https://github.com/argoproj/argo-cd/issues/827)
- Adds `argocd app sync APPNAME --async` [#1728](https://github.com/argoproj/argo-cd/issues/1728)
- Allow users to define app specific urls to expose in the UI [#1677](https://github.com/argoproj/argo-cd/issues/1677)
- Error view instead of blank page in UI [#1375](https://github.com/argoproj/argo-cd/issues/1375)
- Project Editor: Whitelisted Cluster Resources doesn't strip whitespace [#1693](https://github.com/argoproj/argo-cd/issues/1693)
- Eliminate unnecessary git interactions for top-level resource changes (#1919)
- Ability to rotate the bearer token used to manage external clusters (#1084)

#### Bug Fixes

- Project Editor: Whitelisted Cluster Resources doesn't strip whitespace [#1693](https://github.com/argoproj/argo-cd/issues/1693)
- \[ui small bug\] menu position outside block [#1711](https://github.com/argoproj/argo-cd/issues/1711)
- UI will crash when create application without destination namespace [#1701](https://github.com/argoproj/argo-cd/issues/1701)
- ArgoCD synchronization failed due to internal error [#1697](https://github.com/argoproj/argo-cd/issues/1697)
- Replicasets ordering is not stable on app tree view [#1668](https://github.com/argoproj/argo-cd/issues/1668)
- Stuck processor on App Controller after deleting application with incomplete operation [#1665](https://github.com/argoproj/argo-cd/issues/1665)
- Role edit page fails with JS error [#1662](https://github.com/argoproj/argo-cd/issues/1662)
- failed parsing on parameters with comma [#1660](https://github.com/argoproj/argo-cd/issues/1660)
- Handle nil obj when processing custom actions [#1700](https://github.com/argoproj/argo-cd/pull/1700)
- Account for missing fields in Rollout HealthStatus [#1699](https://github.com/argoproj/argo-cd/pull/1699) 
- Sync operation unnecessary waits for a healthy state of all resources [#1715](https://github.com/argoproj/argo-cd/issues/1715)
- failed parsing on parameters with comma [#1660](https://github.com/argoproj/argo-cd/issues/1660)
- argocd app sync hangs when cluster is not configured (#1935)
- Do not allow app-of-app child app's Missing status to affect parent (#1954)
- Argo CD don't handle well k8s objects which size exceeds 1mb (#1685)
- Secret data not redacted in last-applied-configuration (#897)
- Running app actions requires only read privileges (#1827)
- UI should allow editing repo URL (#1763)
- Make status fields as optional fields (#1779)
- Use correct healthcheck for Rollout with empty steps list (#1776)

### Other
- Add Prometheus metrics for git repo interactions (#1912)
- App controller should log additional information during app syncing (#1909)
- Make sure api server to repo server grpc calls have timeout (#1820)
- Forked tool processes should timeout (#1821)
- Add health check to the controller deployment (#1785)

#### Contributors

* [Aditya Gupta](https://github.com/AdityaGupta1)
* [Alex Collins](https://github.com/alexec)
* [Alex Matyushentsev](https://github.com/alexmt)
* [Danny Thomson](https://github.com/dthomson25)
* [jannfis](https://github.com/jannfis)
* [Jesse Suen](https://github.com/jessesuen)
* [Liviu Costea](https://github.com/lcostea)
* [narg95](https://github.com/narg95)
* [Simon Behar](https://github.com/simster7)

See also [milestone v1.1](https://github.com/argoproj/argo-cd/milestone/13)

## v1.0.2 (2019-06-14)

### Changes since v1.0.1

* Cluster registration was unintentionally persisting client-cert auth credentials (#1742)

## Checking if Argo CD is storing unneccesry credentials.

The following instructions explain if any action is needed to remove certificate-authentication data from Argo CD:

1. Run the following command and see if you see key data. If there is non-null output (e.g. output like the following), then Argo CD is storing unnecessary credentials and the cluster credentials should be re-added.

```shell
kubectl get secret cluster-192.168.64.66-892108214 -o json | jq -r '.data.config' | base64 --decode | jq -r '.tlsClientConfig.keyData'
LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBeU54R1U1R2lPdEsydHpqWjVTMlVJUHJ6c0VyYktTdFlBakc2V3RqQUhjSHFQWmVzClkyTy96ekNNK3c5SFdSOEVCdkE3NjROdHdVVHFCVkFiQU05a3kvbEZLVjdmSUFkZVVDWmQ2QzZ6OXpDc242eTcKTnlBcWJETDR6b0xjZHRsUEJmL3JuSENKRXgwTEhiRWJscHNVSUpoNHZVWUNDSkRnbGh1NVNzM3ZGRmxNdkZBbQpZdis3QlZ3aE5YU2RwdEU1amg4WU1VTUJMTHFHQzAwUXpsKzVUZmdEVk9qd3U5Rzdub1pvanhZSlZScnJvaDQ2CjY4Z05hdVJkUzNaY2dxVkRUMWVrTmlsQjFKZW83Q0ZiaWdlRW5FUGVUenF5T3BjWG1lbkh5dDZ6L1BNZGN3LzgKbFpLWWkzaHpJcTBKY1lGWTBranBZME05bE95d2dveUd6YTcxRXdJREFRQUJBb0lCQVFDVHNwNG9EMXZxdzAxRwpONURLWEJTamw4VWZxanV6NzBKZEFySVU0WE9Mcmk4UHNYczY3bnQ1NENxYTVtWkJtM1A3b2lWOWpmeGo5TWZjCnRrWFU5NndYN1NrMVBhVDJ5VlJKdlp5cUFjV21DKzJ6MEhFdUhRSDA1QnBleUkxUys0S0hWK09wK25waFNxY0UKNDFuMUNmM241aFpLbjdNWkYyZCtHYzdMdWRpRzdjSG8walZmdXQ1eVNONitHeC91VTlraGV3ZERMQXRzOFVMeQp6NnVlYm9uTUhkQlVZNHNoNEtWZktGRmRrV2FRNlB5YU1DNktUOGNFbWJBY1RPT01yNVc5cjRXVHNxSXZ0ckNhClpzSWQ2YUhmUFRaWTZuMjdqNXNBd2tGZ0tSZXFmZ0NwZGdtbUM1bGxCRS9sTkhUbXFhNkRJc25MbXhzQUEvYjgKYWcvZVRaM2hBb0dCQU9NVGJaaDM5SlJtdkNTK2NWcnN2eHZ1V3FMNEdnbnRQUUhEUzhPZXBNcnR2V0dvV2hDSwpaWVRwU3dxZHh6dFArWU5iRFNzUCsyZ09TRnV2bEFSWFE3dzRqbWdNZHZVSDFqb0FSSDhuN3M0MFFKUkU1STJkCnROSU9pd0FLeTNpdDJxNy81UXlnemtQNVpTWktwSkhZWWk3QTRhaDFZZ3lQcmpmaXB0V3dYTnpqQW9HQkFPSnkKQVFvK3ZjQlJDZVV4T2lWajkzRFZJMkNvb1NhcmlBa1RQRnJhcTZXUm02S2FSTlpBYjZlWk0zc2JiMDNYSm9mbgp6ZC92UmJBWWYzeVV1K1BsL0s4Y0VnQUVYSHRxOVhBZ2NPQ0xYTzExRHlTV3RrMEowYnRYSlFrM01DQXBHVnc2CkFUbXpGMGNuYjBQdXZhZXNRTStUWVdwMzd2cU56S3hmdmRsUXRxNFJBb0dCQU5pNExoMGFQMTl6UFpXRC9RUGUKZC9iY1lieXdOWW5MMWpIY2htN0k5bGFHMS94Z2hMVE1vVjljbUxZbEo0VEFLMDdtazRiSjFoUFZyZEZ6blQwWApYQnBEa0FaVi95S1V2Q3pYSElpUFFDZWxUdzB6UXo2MWlXSUJaMEEvRFRxOEVyNTZrOHlkbkw3YlEySnNVdXl2CksrV2JTTU5TWktYQWEzSUM2MTkrMXVJcEFvR0FZSDlrb2hFS202SHRMWlpFeVJwSW4vUzBGc1RGcDgwQk01elcKNDRDOEZOcHdFR0xkWXRBaXhMRXNseEdoNVBJQ29YZk82OWJ6UTQrdEJGSDluNmlxZlpUZ3R0RWsrQk1rZEp2ZQpmbEhsVCt2S2dEVVppc3JjYlpFOVh5ZjlnamNCYjZQb1VjWlg3U0tJNzlJVlVCYS9wN1dPbGVoMkZwL0cwTTRjCkFUZThJWUVDZ1lCSWoweVZ3aGlmNm1FcjVrUmIveFJRb2V4aGdrSW1CME9LVnlFZUU3VTFBR1RjYXRQSWlSZFIKdWpJSFpOdzVNRWZ1b24vWFZkdXl1dytqakFVcUZ5c05aNTdQT2tXdjUweXMyTkVSM0RXUFJReS9MN0M2ZWNOcQpFNDhiWGt3V0ExVXR2QUlMckRDa0s5ZVd3cEpLNTdFQms4d2NvdkU4YlRRV0hUTEJndHEzWnc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
```

If the secret does not contain key data (i.e. output like the following), then no action is necessary:
```shell
$ kubectl get secret cluster-192.168.64.66-892108214 -o json | jq -r '.data.config' | base64 --decode | jq -r '.tlsClientConfig.keyData'
null
```

2. If key data is stored in the secret, run the following commands to re-add the cluster to Argo CD. **_IMPORTANT_**: ensure you are using argocd v1.0.2 CLI or greater.

```shell
$ argocd cluster rm CLUSTERURL
$ argocd cluster add CONTEXTNAME
```

## v1.0.1 (2019-05-28)

### Changes since v1.0.1

* Public git creds (#1625)

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

## v0.12.3 (2019-04-30)

## Changes since v0.12.2

- Application controller becomes unresponsive (#1476)

## v0.12.2 (2019-04-22)

## Changes since v0.12.1

- Fix racing condition in controller cache (#1498)
- "bind: address already in use" after switching to gRPC-Web (#1451)
- Annoying warning while using --grpc-web flag (#1420)
-  Delete helm temp directories (#1446)
- Fix null pointer exception in secret normalization function (#1389)
- Argo CD should not delete CRDs(#1425)
- UI is unable to load cluster level resource manifest (#1429)

## v0.12.1 (2019-04-09)

## Changes since v0.12.0

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

## v0.12.0 (2019-03-23)

## New Features

### Improved UI

Many improvements to the UI were made, including:

* Table view when viewing applications
* Filters on applications
* Table view when viewing application resources
* YAML editor in UI
* Switch to text-based diff instead of json diff
* Ability to edit application specs

### Custom Health Assessments (CRD Health)

Argo CD has long been able to perform health assessments on resources, however this could only
assess the health for a few native kubernetes types (deployments, statefulsets, daemonsets, etc...).
Now, Argo CD can be extended to gain understanding of any CRD health, in the form of Lua scripts.
For example, using this feature, Argo CD now understands the CertManager Certificate CRD and will
report a Degraded status when there are issues with the cert.

### Configuration Management Plugins

Argo CD introduces Config Management Plugins to support custom configuration management tools other
than the set that Argo CD provides out-of-the-box (Helm, Kustomize, Ksonnet, Jsonnet). Using config
management plugins, Argo CD can be configured to run specified commands to render manifests. This
makes it possible for Argo CD to support other config management tools (kubecfg, kapitan, shell
scripts, etc...).

### High Availability

Argo CD is now fully HA. A set HA of manifests are provided for users who wish to run Argo CD in
a highly available manner. NOTE: The HA installation will require at least three different nodes due
to pod anti-affinity roles in the specs.

### Improved Application Source

* Support for Kustomize 2
* YAML/JSON/Jsonnet Directories can now be recursed
* Support for Jsonnet external variables and top-level arguments

### Additional Prometheus Metrics

Argo CD provides the following additional prometheus metrics:
* Sync counter to track sync activity and results over time
* Application reconciliation (refresh) performance to track Argo CD performance and controller activity
* Argo CD API Server metrics for monitoring HTTP/gRPC requests

### Fuzzy Diff Logic

Argo CD can now be configured to ignore known differences for resource types by specifying a json
pointer to the field path to ignore. This helps prevent OutOfSync conditions when a user has no
control over the manifests. Ignored differences can be configured either at an application level, 
or a system level, based on a group/kind.

### Resource Exclusions

Argo CD can now be configured to completely ignore entire classes of resources group/kinds.
Excluding high-volume resources improves performance and memory usage, and reduces load and
bandwidth to the Kubernetes API server. It also allows users to fine-tune the permissions that
Argo CD needs to a cluster by preventing Argo CD from attempting to watch resources of that
group/kind.

### gRPC-Web Support

The argocd CLI can be now configured to communicate to the Argo CD API server using gRPC-Web
(HTTP1.1) using a new CLI flag `--grpc-web`. This resolves some compatibility issues users were
experiencing with ingresses and gRPC (HTTP2), and should enable argocd CLI to work with virtually
any load balancer, ingress controller, or API gateway.

### CLI features

Argo CD introduces some additional CLI commands:

* `argocd app edit APPNAME` - to edit an application spec using preferred EDITOR
* `argocd proj edit PROJNAME` - to edit an project spec using preferred EDITOR
* `argocd app patch APPNAME` - to patch an application spec
* `argocd app patch-resource APPNAME` - to patch a specific resource which is part of an application


## Breaking Changes

### Label selector changes, dex-server rename

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

### Deprecation of spec.source.componentParameterOverrides

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

### Removal of spec.source.environment and spec.source.valuesFiles

The `spec.source.environment` and `spec.source.valuesFiles` fields, which were deprecated in v0.11,
are now completely removed from the Application spec.


### API/CLI compatibility

Due to API spec changes related to the deprecation of componentParameterOverrides, Argo CD v0.12
has a minimum client version of v0.12.0. Older CLI clients will be rejected.

## Changes since v0.11
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

# Changes since v0.11.1:
+ Adds client retry. Fixes #959 (#1119)
- Prevent deletion hotloop (#1115)
- Fix EncodeX509KeyPair function so it takes in account chained certificates (#1137) (@amarruedo)
- Exclude metrics.k8s.io from watch (#1128)
- Fix issue where dex restart could cause login failures (#1114)
- Relax ingress/service health check to accept non-empty ingress list (#1053)
- [UI] Correctly handle empty response from repository/<repo>/apps API

## v0.11.1 (2019-01-18)

## Changes since v0.11.0:
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

## v0.11.0 (2019-01-11)

This is Argo CD's biggest release ever and introduces a completely redesigned controller architecture.

# New Features

## Performance & Scalability
The application controller has a completely redesigned architecture which improved performance and scalability during application reconciliation.

This was achieved by introducing an in-memory, live state cache of lightweight Kubernetes object metadata. During reconciliation, the controller no longer performs expensive, in-line queries of app related resources in K8s API server, instead relying on the metadata available in the live state cache. This dramatically improves performance and responsiveness, and is less burdensome to the K8s API server.

## Object relationship visualization for CRDs
With the new controller design, Argo CD is now able to understand ownership relationship between *all* Kubernetes objects, not just the built-in types. This enables Argo CD to visualize parent/child relationships between all kubernetes objects, including CRDs.

## Multi-namespaced applications
During sync, Argo CD will now honor any explicitly set namespace in a manifest. Manifests without a namespace will continue deploy to the "preferred" namespace, as specified in app's `spec.destination.namespace`. This enables support for a class of applications which install to multiple namespaces. For example, Argo CD can now install the [prometheus-operator](https://github.com/helm/charts/tree/master/stable/prometheus-operator) helm chart, which deploys some resources into `kube-system`, and others into the `prometheus-operator` namespace.

## Large application support
Full resource objects are no longer stored in the Application CRD object status. Instead, only lightweight metadata is stored in the status, such as a resource's sync and health status. This change enabled Argo CD to support applications with a very large number of resources (e.g. istio), and reduces the bandwidth requirements when listing applications in the UI.

## Resource lifecycle hook improvements
Resource lifecycle hooks (e.g. PreSync, PostSync) are now visible/manageable from the UI. Additionally, bare Pods with a restart policy of Never can now be used as a resource hook, as an alternative to Jobs, Workflows.

## K8s recommended application labels
The tracking label for resources has been changed to use `app.kubernetes.io/instance`, as recommended in [Kubernetes recommended labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/), (changed from `applications.argoproj.io/app-name`). This will enable applications managed by Argo CD to interoperate with other tooling which are also converging on this labeling, such as the Kubernetes dashboard. Additionally, Argo CD no longer injects any tracking labels at the `spec.template.metadata` level.

## External OIDC provider support
Argo CD now supports auth delegation to an existing, external OIDC providers without the need for running Dex (e.g. Okta, OneLogin, Auth0, Microsoft, etc...)

The optional, [Dex IDP OIDC provider](https://github.com/dexidp/dex) is still bundled as part of the default installation, in order to provide a seamless out-of-box experience, enabling Argo CD to integrate with non-OIDC providers, and to benefit from Dex's full range of [connectors](https://github.com/dexidp/dex/tree/master/Documentation/connectors). 

## OIDC group bindings to Project Roles
OIDC group claims from an OAuth2 provider can now be bound to Argo CD project roles. Previously, group claims could only be managed in the centralized ConfigMap, `argocd-rbac-cm`. They can now be managed at a project level. This enables project admins to self service access to applications within a project.

## Declarative Argo CD configuration
Argo CD settings can be now be configured either declaratively, or imperatively. The `argocd-cm` ConfigMap now has a `repositories` field, which can reference credentials in a normal Kubernetes secret which you can create declaratively, outside of Argo CD. 

## Helm repository support
Helm repositories can be configured at the system level, enabling the deployment of helm charts which have a dependency to external helm repositories.

# Breaking changes:

* Argo CD's resource names were renamed for consistency. For example, the application-controller deployment was renamed to argocd-application-controller. When upgrading from v0.10 to v0.11, the older resources should be pruned to avoid inconsistent state and controller in-fighting. 
* As a consequence to moving to recommended kubernetes labels, when upgrading from v0.10 to v0.11, all applications will immediately be OutOfSync due to the change in tracking labels. This will correct itself with another sync of the application. However, since Pods will be recreated, please take this into consideration, especially if your applications are configured with auto-sync.
* There was significant reworking of the `app.status` fields to reduce the payload size, simplify the datastructure and remove fields which were no longer used by the controller. No breaking changes were made in `app.spec`.
* An older Argo CD CLI (v0.10 and below) will not be compatible with Argo CD v0.11. To keep CI pipelines in sync with the API server, it is recommended to have pipelines download the CLI directly from the API server https://${ARGOCD_SERVER}/download/argocd-linux-amd64 during the CI pipeline.

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

## v0.10.6 (2018-11-15)

- Fix issue preventing in-cluster app sync due to go-client changes (issue #774)

## v0.10.5 (2018-11-14)

+ Increase concurrency of application controller
* Update dependencies to k8s v1.12 and client-go v9.0 (#729)
- Fix issue where applications could not be deleted on k8s v1.12
- add argo cluster permission to view logs (@conorfennell)
- [UI] Allow 'syncApplication' action to reference target revision rather then hard-coding to 'HEAD'
- [UI] Issue #768 - Fix application wizard crash

## v0.10.4 (2018-11-08)

* Upgrade to Helm v0.11.0 (@amarrella)
- Health check is not discerning apiVersion when assessing CRDs (issue #753)
- Fix nil pointer dereference in util/health (@mduarte)

## v0.10.3 (2018-10-29)

- Fix applying TLS version settings
* Update to kustomize 1.0.10

## v0.10.2 (2018-10-25)

- Fix app refresh err when k8s patch is too slow

* Update to kustomize 1.0.9

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

* The `server.crt` and `server.key` fields of `argocd-secret` had been renamed to `tls.crt` and `tls.key` for
better integration with cert manager(issue #617). Existing `argocd-secret` should be updated accordingly to
preserve existing TLS certificate.
* Cluster wide resources should be allowed in default project (due to issue #330):

```
argocd project allow-cluster-resource default '*' '*'
```

### Changes since v0.8:
+ Auto-sync option in application CRD instance (issue #79)
+ Support raw jsonnet as an application source (issue #540)
+ Reorder K8s resources to correct creation order (issue #102)
+ Redact K8s secrets from API server payloads (issue #470)
+ Support --in-cluster authentication without providing a kubeconfig (issue #527)
+ Special handling of CustomResourceDefinitions (issue #613)
+ ArgoCD should download helm chart dependencies (issue #582)
+ Export ArgoCD stats as prometheus style metrics (issue #513)
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
- [UI] Fix issue where projects filter does not work when application got changed
- [UI] Creating apps from directories is not obvious (issue #565)
- Helm hooks are being deployed as resources (issue #605)
- Disagreement in three way diff calculation (issue #597)
- SIGSEGV in kube.GetResourcesWithLabel (issue #587)
- ArgoCD fails to deploy resources list (issue #584)
- Branch tracking not working properly (issue #567)
- Controller hot loop when application source has bad manifests (issue #568)

## v0.8.2 (2018-09-12)

## v0.8.2 (2018-09-12)
- Downgrade ksonnet from v0.12.0 to v0.11.0 due to quote unescape regression
- Fix CLI panic when performing an initial `argocd sync/wait`

## v0.8.1 (2018-09-11)

# v0.8.1
## Changes since v0.8.0
* [UI] Support selection of helm values files in App creation wizard (issue #499)
* [UI] Support specifying source revision in App creation wizard (issue #503)
* [UI] Improve resource diff rendering (issue #457)
* [UI] Indicate number of ready containers in pod (issue #539)
* [UI] Indicate when app is overriding parameters (issue #503)
* [UI] Provide a YAML view of resources (issue #396)
* Fix issue where changes were not pulled when tracking a branch (issue #567)
* Fix controller hot loop when app source contains bad manifests (issue #568)

## v0.8.0 (2018-09-05)

# Notes about upgrading from v0.7
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

# Changes since v0.7:
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

Fixed:
- API discovery becomes best effort when partial resource list is returned (issue #524)

## v0.7.1 (2018-08-03)

# Changes from v0.7.0

## Features:
+ Surface helm parameters to the application level (#485)
+ [UI] Improve application creation wizard (#459)
+ [UI] Show indicator when refresh is still in progress (#493)

## Refactoring and improvements:
* [UI] Improve data loading error notification (#446)
* Infer username from claims during an `argocd relogin` (#475)
* Expand RBAC role to be able to create application events. Fix username claims extraction
* Fix linux download link in getting_started.md (#487) (@chocopowwwa)

## Bug Fixes
- Fix scalability issues with the ListApps API (#494)
- Fix issue where application server was retrieving events from incorrect cluster (#478)
- Fix failure in identifying app source type when path was '.'
- AppProjectSpec SourceRepos mislabeled (#490)
- Failed e2e test was not failing CI workflow

## v0.7.0 (2018-07-28)

New Features:
+ Support helm charts and yaml directories as an application source
+ Audit trails in the form of API call logs
+ Generate kubernetes events for application state changes
+ Add ksonnet version to version endpoint (#433)
+ Show CLI progress for sync and rollback
+ Make use of dex refresh tokens and store them into local config
+ Add `argocd relogin` command as a convenience around login to current context

Refactoring & Improvements
* Expire local superuser tokens when their password changes

Bug Fixes:
- Fix saving default connection status for repos and clusters
- Fix undesired fail-fast behavior of health check
- Fix memory leak in the cluster resource watch
- Health check for StatefulSets, DaemonSet, and ReplicaSets were failing due to use of wrong converters

## v0.6.2 (2018-07-24)

Bug fixes:
* Health check for StatefulSets, DaemonSet, and ReplicaSets were failing due to use of wrong converters

## v0.6.1 (2018-07-18)

Bug Fixes:
- Fix regression where deployment health check incorrectly reported Healthy
- Intercept dex SSO errors and present them in Argo login page

## v0.6.0 (2018-07-17)

Features:
+ Support PreSync, Sync, PostSync resource hooks
+ Introduce Application Projects for finer grain RBAC controls
+ Swagger Docs & UI
+ Support in-cluster deployments internal kubernetes service name

Refactoring & Improvements
* Improved error handling, status and condition reporting
* Remove installer in favor of `kubectl apply` instructions
* Add validation when setting application parameters
* Cascade deletion is decided during app deletion, instead of app creation

Bug Fixes:
- Fix git authentication implementation when using using SSH key
- app-name label was inadvertently injected into spec.selector if selector was omitted from v1beta1 specs

## v0.5.4 (2018-06-27)

Refresh flag to sync should be optional, not required

## v0.5.3 (2018-06-21)

+ Support cluster management using the internal k8s API address https://kubernetes.default.svc (#307)
+ Support diffing a local ksonnet app to the live application state (resolves #239) (#298)
+ Add ability to show last operation result in `app get`. Show path in `app list -o wide` (#297)
+ Update dependencies: ksonnet v0.11, golang v1.10, debian v9.4 (#296)
+ Add ability to force a refresh of an app during get (resolves #269) (#293)
+ Automatically restart API server upon certificate changes (#292)

## v0.5.2 (2018-06-14)

+ Resource events tab on application details page (#286)
+ Display pod status on application details page (#231)

## v0.5.1 (2018-06-13)

* API server incorrectly compose application fully qualified name for RBAC check (#283)
* UI crash while rendering application operation info if operation failed

## v0.5.0 (2018-06-12)

+ RBAC access control
+ Repository/Cluster state monitoring
+ ArgoCD settings import/export
+ Application creation UI wizard
+ `argocd app manifests` for printing the application manifests
+ `argocd app unset` command to unset parameter overrides

* Fail app sync if `prune` flag is required (#276)
* Take into account number of unavailable replicas to decided if deployment is healthy or not #270
* Repo names containing underscores were not being accepted (#258)
* Cookie token was not parsed properly when mixed with other site cookies
* Add ability to show parameters and overrides in CLI (resolves #240)

## v0.4.7 (2018-06-07)

* Fix `argocd app wait` health checking logic

## v0.4.6 (2018-06-06)

- Retry `argocd app wait` connection errors from EOF watch. Show detailed state changes

## v0.4.5 (2018-05-31)

+ Add `argocd app unset` command to unset parameter overrides
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

* Remove interactive context name prompt during login which broke login automation
* Show URL in argocd app get
* Rename force flag to cascade in argocd app delete

## v0.4.1 (2018-05-18)

Implemented `argocd app wait` command

## v0.4.0 (2018-05-17)

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

* Application sync should delete 'unexpected' resources https://github.com/argoproj/argo-cd/issues/139
* Update ksonnet to v0.10.1
* Detect `unexpected` resources 
* Fix: App sync frequently fails due to concurrent app modification https://github.com/argoproj/argo-cd/issues/147
*  Fix: improve app state comparator: https://github.com/argoproj/argo-cd/issues/136, https://github.com/argoproj/argo-cd/issues/132

## v0.3.1 (2018-04-24)

- Add new rollback RPC with numeric identifiers
- New `argo app history` and `argo app rollback` command
- Switch to gogo/protobuf for golang code generation
- Fix: create .argocd directory during `argo login` (issue #123)
- Fix: Allow overriding server or namespace separately (issue #110)

## v0.3.0 (2018-04-23)

* Auth support
* TLS support
* DAG-based application view
* Bulk watch
* ksonnet v0.10.0-alpha.3
* kubectl apply deployment strategy
* CLI improvements for app management

## v0.2.0 (2018-04-03)

- Bug fixes
- Rollback UI
- Override parameters

## v0.1.0 (2018-03-13)

- Define app in Github with dev and preprod environment using KSonnet
- Add cluster Diff App with a cluster Deploy app in a cluster
-  Deploy a new version of the app in the cluster
- App sync based on Github app config change - polling only
- Basic UI: App diff between Git and k8s cluster for all environments Basic GUI


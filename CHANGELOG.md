# Changelog

## v0.9.0 (Unreleased)

### Changes since v0.8:
+ Support raw jsonnet as an application source (#issue #540)
+ Reorder K8s resources to correct creation order (issue #102)
+ Redact K8s secrets from API server payloads (issue #470)
+ [UI] Support selection of helm values files in App creation wizard (issue #499)
+ [UI] Support specifying source revision in App creation wizard allow (issue #503)
+ [UI] Improve resource diff rendering (issue #457)
+ [UI] Indicate number of ready containers in pod (issue #539)
+ [UI] Indicate when app is overriding parameters (issue #503)
+ [UI] Provide a YAML view of resources (issue #396)
* Enable TLS for repo server (issue #553)
- Fix issue where changes were not pulled when tracking a branch (issue #567)
- Fix controller hot loop when app source contains bad manifests (issue #568)
- [UI] Fix issue where projects filter does not work when application got changed

## v0.8.2 (2018-09-11)
- Downgrade ksonnet from v0.12.0 to v0.11.0 due to quote unescape regression

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
+ ArgoCD settings import/export
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

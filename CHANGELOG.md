# Changelog

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


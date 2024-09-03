## Resolving conflicts during upstream changes merge 

This docs include info about places where codefresh made it's customizations:

#### General notes:
1. All files that're deleted in our branches - we can keep deleted (accept ours).
2. all `xxx.pb.go` - apply theirs and after resolving conflicts re-generate.

#### Paths and actions on them
1. `.github/workflows` - accept ours (yours).
2. `applicationset` - accept theirs
3. `assets / swagger` - accept ours. Later run codegen and commit new version
4. `cmd / argocd` - accept ours if files deleted.
5. `cmd / argocd-application-controller` - no custom thing from our side, so just resolve conflicts.
6. `cmd / notifications` - no custom thing from our side, so just accept theirs.
7. `cmd / argocd-repo-server` - includes our changes with codefresh related parameters.
8. `cmd / common` - includes our changes with codefresh related constants (event-reporter)
9. `cmd / controller / application.go` - includes our changes to resource node (to return labels and annotations getResourceTree method)
10. `cmd / controller / state.go` - includes our changes (GetRepoObjs method)
11. `cmd / controller / state_test.go - includes our changes. Replace manifest values with our struct `apiclient.Manifest`
12. `docs` - apply theirs
13. `examples` - apply theirs
14. `hack` - apply theirs
15. `manifests` - merge (it might include codefresh image repositories). Includes v2 event-reporter manifests (often by replacing notifications)
16. `notification_controller` - apply theirs
17. `pkg/apis/application/v1alpha` - generatedXXX - apply theirs (than re-generate). types.go  - merge (includes our changes with ForceNamespace).
18. `server / application.go` - merge (includes our v1 event-reporter.)
19. `ui` - accept theirs.
20. `util / kustomize` - merge, as it includes ours changes.
21. `mkdocs.yaml` - apply theirs.
22. `go.mod` - merge direct dependencies. go.sum accept theirs. Run go mod tidy. Check `replace` section, perform cleanup if needed.
23. `reposerver / sepository.go` - merge, includes: cf appVersion logic; type manifest struct (with path to file, rawManifest);


#### Post actions:
1. run `go mod tidy`
2. run `go mod download`
3. run `go mod vendor`
4. run `make lint-local`
5. run `make protogen-fast` - because sometimes gogen won't work if types from protogen used
6. run `make codegen`
7. run `make test-local`

### Thoughts

1. Revert cherry picks before merges - as they cause issues later if in upstream decided to slightly move some parts of such changes. In this case no conflicts will occur during merge as they on different lines but then you need cleanup them manually.
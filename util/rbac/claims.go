package rbac

// ClaimsSubjectKeys is used to access claims in context objects
const ClaimsSubjectKey = "claims"

// RBAC enforcement claim objects
const (
	ClaimsResourceApplications = "applications"
	ClaimsResourceClusters     = "clusters"
	ClaimsResourceProjects     = "projects"
	ClaimsResourceRepositories = "repositories"
)

// RBAC enforcement claim actions
const (
	ClaimsActionCreate = "create"
	ClaimsActionDelete = "delete"
	ClaimsActionGet    = "get"
	ClaimsActionSync   = "sync"
	ClaimsActionUpdate = "update"
)

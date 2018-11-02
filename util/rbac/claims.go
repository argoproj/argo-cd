package rbac

// ClaimsSubjectKeys is used to access claims in context objects
const ClaimsSubjectKey = "claims"

// RBAC enforcement claim objects
const (
	ClaimsObjectApplications = "applications"
	ClaimsObjectClusters     = "clusters"
	ClaimsObjectProjects     = "projects"
	ClaimsObjectRepositories = "repositories"
)

// RBAC enforcement claim actions
const (
	ClaimsActionCreate = "create"
	ClaimsActionDelete = "delete"
	ClaimsActionGet    = "get"
	ClaimsActionSync   = "sync"
	ClaimsActionUpdate = "update"
)

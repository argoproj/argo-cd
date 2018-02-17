package application

const (
	// API Group
	Group string = "argoproj.io"

	// Application constants
	ApplicationKind      string = "Application"
	ApplicationSingular  string = "application"
	ApplicationPlural    string = "applications"
	ApplicationShortName string = "app"
	ApplicationFullName  string = ApplicationPlural + "." + Group

	// Cluster constants
	ClusterKind      string = "Cluster"
	ClusterSingular  string = "cluster"
	ClusterPlural    string = "clusters"
	ClusterShortName string = "cluster"
	ClusterFullName  string = ClusterPlural + "." + Group
)

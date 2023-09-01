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

	// AppProject constants
	AppProjectKind      string = "AppProject"
	AppProjectSingular  string = "appproject"
	AppProjectPlural    string = "appprojects"
	AppProjectShortName string = "appproject"
	AppProjectFullName  string = AppProjectPlural + "." + Group

	// ApplicationSet constants
	ApplicationSetKind      string = "ApplicationSet"
	ApplicationSetSingular  string = "applicationset"
	ApplicationSetShortName string = "appset"
	ApplicationSetPlural    string = "applicationsets"
	ApplicationSetFullName  string = ApplicationSetPlural + "." + Group

	// SyncStrategy constants
	SyncStrategyKind      string = "SyncStrategy"
	SyncStrategySingular  string = "syncstrategy"
	SyncStrategyShortName string = "syncstrategy"
	SyncStrategyPlural    string = "syncstrategies"
	SyncStrategyFullName  string = SyncStrategyPlural + "." + Group

	// ClusterSyncStrategy constants
	ClusterSyncStrategyKind      string = "ClusterSyncStrategy"
	ClusterSyncStrategySingular  string = "clustersyncstrategy"
	ClusterSyncStrategyShortName string = "clustersyncstrategy"
	ClusterSyncStrategyPlural    string = "clustersyncstrategies"
	ClusterSyncStrategyFullName  string = ClusterSyncStrategyPlural + "." + Group
)

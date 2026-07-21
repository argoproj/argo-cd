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

	// ArgoCDConfiguration constants
	ArgoCDConfigurationKind      string = "ArgoCDConfiguration"
	ArgoCDConfigurationSingular  string = "argocdconfiguration"
	ArgoCDConfigurationPlural    string = "argocdconfigurations"
	ArgoCDConfigurationShortName string = "argocdconfig"
	ArgoCDConfigurationFullName  string = ArgoCDConfigurationPlural + "." + Group
	// ArgoCDConfigurationName is the required singleton object name.
	ArgoCDConfigurationName string = "argocd-config"
)

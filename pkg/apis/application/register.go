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
)

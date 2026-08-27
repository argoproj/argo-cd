package commitserver

// Config holds durable commit-server settings resolved from flags/env/defaults
// before NewServer. InitConfigProvider captures these into StaticFields.
type Config struct {
	ListenHost                 string
	ListenPort                 int
	MetricsHost                string
	MetricsPort                int
	LogFormat                  string
	LogLevel                   string
	GrpcEnableTxtServiceConfig bool
}

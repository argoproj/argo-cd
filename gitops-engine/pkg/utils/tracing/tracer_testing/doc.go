package tracer_testing

//go:generate go run go.uber.org/mock/mockgen -destination "logger.go" -package "tracer_testing" "github.com/go-logr/logr" "LogSink"

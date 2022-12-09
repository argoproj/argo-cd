package tracer_testing

//go:generate go run github.com/golang/mock/mockgen -destination "logger.go" -package "tracer_testing" "github.com/go-logr/logr" "LogSink"

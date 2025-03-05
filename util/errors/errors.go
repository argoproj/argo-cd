package errors

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
)

const (
	// ErrorGeneric is returned for generic error
	ErrorGeneric = 20
)

type Handler struct {
	t *testing.T
}

func NewHandler(t *testing.T) *Handler {
	t.Helper()
	return &Handler{t: t}
}

// CheckForErr fails the test if there is an error.
func (h *Handler) CheckForErr(err error) {
	h.t.Helper()
	if err != nil {
		h.t.Fatal(err)
	}
}

// FailOnErr fails the test if there is an error. It returns the first value so you can use it if you cast it:
// text := FailOrErr(Foo).(string)
func (h *Handler) FailOnErr(v any, err error) any {
	h.t.Helper()
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

// CheckError logs a fatal message and exits with ErrorGeneric if err is not nil
func CheckError(err error) {
	if err != nil {
		Fatal(ErrorGeneric, err)
	}
}

// Fatal is a wrapper for logrus.Fatal() to exit with custom code
func Fatal(exitcode int, args ...any) {
	exitfunc := func() {
		os.Exit(exitcode)
	}
	log.RegisterExitHandler(exitfunc)
	log.Fatal(args...)
}

// Fatalf is a wrapper for logrus.Fatalf() to exit with custom code
func Fatalf(exitcode int, format string, args ...any) {
	exitfunc := func() {
		os.Exit(exitcode)
	}
	log.RegisterExitHandler(exitfunc)
	log.Fatalf(format, args...)
}

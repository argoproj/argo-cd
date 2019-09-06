package metrics

type Reporter interface {
	Event(repoURL, event string)
}

var NopReporter = nopReporter{}

type nopReporter struct {
}

func (n nopReporter) Event(repoURL, event string) {
}

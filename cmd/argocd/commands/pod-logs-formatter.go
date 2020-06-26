package commands

import (
	"bytes"
	"fmt"

	"github.com/sirupsen/logrus"
)

// used to format the output of when tailing pod logs
type podLogFormatter struct{}

// takes a log entry from PodLogs(), and makes it readable
func (plf *podLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer

	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	b.WriteByte('[')
	b.WriteString(fmt.Sprintf("%v", entry.Data["podName"]))
	b.WriteByte(']')
	b.WriteByte(' ')
	if entry.Message != "" {
		b.WriteString(" - ")
		b.WriteString(entry.Message)
	}
	b.WriteByte('\n')

	return b.Bytes(), nil
}

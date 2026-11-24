package fixture

import (
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
)

// Custom formatter that truncates excessively long messages
type truncatingFormatter struct {
	inner log.Formatter
	// maximum length (bytes)
	maxLen int
}

func (f truncatingFormatter) Format(e *log.Entry) ([]byte, error) {
	b, err := f.inner.Format(e)
	numBytes := len(b)
	if numBytes <= f.maxLen {
		return b, err
	}
	for i := 0; i < numBytes; {
		// parse characters to avoid producing invalid non-UTF8 compliant text
		_, size := utf8.DecodeRune(b[i:])
		newLen := i + size
		// truncate until current byte if next character exceeds the limit
		if newLen > f.maxLen {
			b = b[:i]
			b = append(b, []byte("...[truncated]...\n")...)
			break
		}
		i = newLen
	}
	return b, err
}

func MakeTruncatingFormatter(maxLineLen int) log.Formatter {
	formatter := truncatingFormatter{
		log.StandardLogger().Formatter,
		maxLineLen,
	}
	return formatter
}

package shared

import (
	"time"
)

type CommitMetadata struct {
	// Commit message
	Message string
	// Commit author
	Author string
	// Commit creation date
	Date time.Time
	// Associated tags
	Tags []string
}

package buffered_context

import (
	"context"
	"time"
)

// WithEarlierDeadline creates a new context with a deadline before the given context's deadline. The buffer parameter
// determines how much earlier the new deadline is. Returns the new context (with the original context as its parent)
// and a pointer to a cancel function for the new context.
//
// If the given context doesn't have a deadline, return the original context unchanged and a do-nothing cancel function.
func WithEarlierDeadline(originalCtx context.Context, buffer time.Duration) (context.Context, context.CancelFunc) {
	var cancelFunc context.CancelFunc = func() {}
	bufferedCtx := originalCtx
	if deadline, ok := originalCtx.Deadline(); ok {
		bufferedCtx, cancelFunc = context.WithDeadline(originalCtx, deadline.Add(-1*buffer))
	}
	return bufferedCtx, cancelFunc
}

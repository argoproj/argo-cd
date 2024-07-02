package buffered_context_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/util/buffered_context"
)

func TestWithEarlierDeadline_NoDeadline(t *testing.T) {
	ctx := context.Background()

	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(ctx, 100*time.Millisecond)
	defer cancel()

	assert.Equal(t, ctx, bufferedCtx)

	_, hasDeadline := bufferedCtx.Deadline()
	assert.False(t, hasDeadline)
}

func TestWithEarlierDeadline_WithDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()

	buffer := 100 * time.Millisecond
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(ctx, buffer)
	defer cancel()

	assert.NotEqual(t, ctx, bufferedCtx)
	originalDeadline, _ := ctx.Deadline()
	newDeadline, _ := bufferedCtx.Deadline()
	assert.Equal(t, originalDeadline.Add(-1*buffer), newDeadline)
}

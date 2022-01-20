package buffered_context_test

import (
	"context"
	"github.com/argoproj/argo-cd/v2/util/buffered_context"
	"gopkg.in/go-playground/assert.v1"
	"testing"
	"time"
)

func TestWithEarlierDeadline_NoDeadline(t *testing.T) {
	ctx := context.Background()

	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(ctx, 100*time.Millisecond)
	defer cancel()

	assert.Equal(t, ctx, bufferedCtx)

	_, hasDeadline := bufferedCtx.Deadline()
	assert.IsEqual(hasDeadline, false)
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
	assert.Equal(t, newDeadline, originalDeadline.Add(-1 * buffer))
}

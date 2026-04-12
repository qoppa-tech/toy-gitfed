package e2e

import (
	"context"
	"testing"
	"time"
)

func TestNewSuiteRootContextHasNoDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := newSuiteRootContext()
	t.Cleanup(cancel)

	if _, ok := ctx.Deadline(); ok {
		t.Fatal("expected suite root context to be non-expiring")
	}
}

func TestWithOperationTimeoutAddsDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := withOperationTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected operation context to have a deadline")
	}
}

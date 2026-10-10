package main

import (
	"context"
	"testing"
	"time"
)

func TestUntilShutdown_CancelsCallWhenServerShutsDown(t *testing.T) {
	shutdown, stop := context.WithCancel(context.Background())
	ctx, cancel := untilShutdown(context.Background(), shutdown)
	defer cancel()

	stop()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("tool call context outlived server shutdown")
	}
}

func TestUntilShutdown_CallCancellationStillPropagates(t *testing.T) {
	call, cancelCall := context.WithCancel(context.Background())
	ctx, cancel := untilShutdown(call, context.Background())
	defer cancel()

	cancelCall()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("tool call context ignored the client's cancellation")
	}
}

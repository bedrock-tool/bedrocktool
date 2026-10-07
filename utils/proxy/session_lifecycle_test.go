package proxy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConnectServerWaitHonoursAttemptCancellation(t *testing.T) {
	s := NewSession(context.Background(), ProxySettings{}, nil, nil, true)
	defer s.cancelCtx(nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.connectServer(ctx, nil) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation", err)
		}
	case <-time.After(200 * time.Millisecond):
		s.cancelCtx(nil)
		<-done
		t.Fatal("server wait retained client slot after attempt cancellation")
	}
}

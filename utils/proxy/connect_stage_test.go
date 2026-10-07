package proxy

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

type contextOnlyConn struct {
	minecraft.IConn
	ctx context.Context
}

func (c contextOnlyConn) Context() context.Context { return c.ctx }

func TestClientClosureCancelsSessionAttempt(t *testing.T) {
	s := NewSession(context.Background(), ProxySettings{}, nil, nil, true)
	defer s.cancelCtx(nil)
	ctx, closeClient := context.WithCancel(context.Background())
	s.watchClient(context.Background(), contextOnlyConn{ctx: ctx})
	closeClient()
	select {
	case <-s.ctx.Done():
		if !strings.Contains(context.Cause(s.ctx).Error(), "local client disconnected") {
			t.Fatal(context.Cause(s.ctx))
		}
	case <-time.After(time.Second):
		t.Fatal("disconnected client retained active session")
	}
}

func TestConnectStageDeadlineAndNonsecretDiagnostics(t *testing.T) {
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	_, err := connectStage(context.Background(), logrus.NewEntry(logger), "test-stage", 20*time.Millisecond, func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, errors.Join(ctx.Err(), errors.New("SECRET_TOKEN_DO_NOT_LOG"))
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(output.String(), "SECRET_TOKEN") {
		t.Fatal("diagnostic leaked raw error")
	}
	if !strings.Contains(output.String(), "test-stage") || !strings.Contains(output.String(), "timeout") {
		t.Fatal(output.String())
	}
}

func TestConnectStageSuccessDoesNotCancelParent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	value, err := connectStage(ctx, logrus.NewEntry(logrus.New()), "success", time.Second, func(context.Context) (int, error) { return 42, nil })
	if err != nil || value != 42 || ctx.Err() != nil {
		t.Fatalf("value=%d err=%v parent=%v", value, err, ctx.Err())
	}
}

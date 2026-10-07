package proxy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

const connectServiceTimeout = 90 * time.Second

// Includes authentication, ICE negotiation and resource-pack exchange. Device
// login happens before Session.Run, and is deliberately not subject to this limit.
const connectLoginTimeout = 5 * time.Minute

// connectStage logs only static stage names, duration and outcome: errors may
// contain service response bodies or credentials and must not be logged here.
// Operations must honour ctx; do not abandon them in background goroutines.
func connectStage[T any](ctx context.Context, log *logrus.Entry, stage string, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	log.WithFields(logrus.Fields{"stage": stage, "limit": timeout}).Info("Connection stage started")
	// Even an upstream API that neglects cancellation must not remain silent.
	deadlineLogDone := make(chan struct{})
	stopDeadlineLog := context.AfterFunc(ctx, func() {
		defer close(deadlineLogDone)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			log.WithField("stage", stage).Warn("Connection stage deadline reached; cancellation requested")
		}
	})
	defer func() {
		if !stopDeadlineLog() {
			<-deadlineLogDone
		}
	}()
	value, err := fn(ctx)
	if err == nil && ctx.Err() != nil {
		err = context.Cause(ctx)
	}
	outcome := "complete"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "timeout"
	case errors.Is(err, context.Canceled):
		outcome = "cancelled"
	case err != nil:
		outcome = "failed"
	}
	log.WithFields(logrus.Fields{"stage": stage, "elapsed": time.Since(start).Round(time.Millisecond), "outcome": outcome}).Info("Connection stage finished")
	if err != nil {
		return value, fmt.Errorf("connection stage %s: %w", stage, err)
	}
	return value, nil
}

// A session has one client and one-shot login/resource-pack gates. Reusing its
// slot after an early disconnect would mix old and new login data. End the
// attempt instead, which closes the listener and unblocks the upstream work.
func (s *Session) waitClientData(ctx context.Context) error {
	if !s.withClient {
		return nil
	}
	_, err := connectStage(ctx, s.log, "local-client-login-data", connectServiceTimeout, func(ctx context.Context) (struct{}, error) {
		select {
		case <-s.haveClientData:
			return struct{}{}, nil
		case <-ctx.Done():
			return struct{}{}, context.Cause(ctx)
		}
	})
	return err
}

func (s *Session) watchClient(ctx context.Context, c minecraft.IConn) {
	go func() {
		select {
		case <-ctx.Done():
		case <-c.Context().Done():
			s.log.Info("Local client disconnected; ending connection attempt")
			s.cancelCtx(errors.New("local client disconnected"))
		}
	}()
}

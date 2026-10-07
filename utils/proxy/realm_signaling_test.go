package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/service/signaling"
	"github.com/sandertv/gophertunnel/minecraft/service/signaling/messaging"
)

func TestRealmSignalingDoesNotFallbackForOtherFailures(t *testing.T) {
	host := "signal-francecentral.franchise.minecraft-services.net"
	for _, tc := range []struct {
		name, region string
		err          error
	}{
		{"unauthorized", "FranceCentral", errors.New("HTTP 401 unauthorized")},
		{"deadline", "FranceCentral", context.DeadlineExceeded},
		{"DNS timeout", "FranceCentral", &net.DNSError{Name: host, IsTimeout: true}},
		{"DNS temporary", "FranceCentral", &net.DNSError{Name: host, IsTemporary: true}},
		{"DNS unknown", "FranceCentral", &net.DNSError{Name: host}},
		{"token-service NXDOMAIN", "FranceCentral", &net.DNSError{Name: "authorization.franchise.minecraft-services.net", IsNotFound: true}},
		{"default NXDOMAIN", "", &net.DNSError{Name: host, IsNotFound: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			_, err := dialRealmSignaling(context.Background(), tc.region, func(context.Context, *signaling.Environment) (*messaging.Conn, error) { calls++; return nil, tc.err })
			if err != tc.err || calls != 1 {
				t.Fatalf("error %v, calls %d; want original error and no fallback", err, calls)
			}
		})
	}
}

func TestRealmSignalingPropagatesDiscoveryFailureWithoutRetry(t *testing.T) {
	want := errors.New("discovery unauthorized")
	calls := 0
	_, err := dialRealmSignaling(context.Background(), "FranceCentral", func(_ context.Context, env *signaling.Environment) (*messaging.Conn, error) {
		calls++
		if calls == 1 {
			return nil, &net.DNSError{Name: env.ServiceURI.Hostname(), IsNotFound: true}
		}
		if env != nil {
			t.Fatal("fallback environment must be nil")
		}
		return nil, want
	})
	if err != want || calls != 2 {
		t.Fatalf("error %v, calls %d; want fallback error and exactly one retry", err, calls)
	}
}

func TestRealmSignalingSuccessDoesNotFallback(t *testing.T) {
	want := new(messaging.Conn)
	calls := 0
	got, err := dialRealmSignaling(context.Background(), "FranceCentral", func(context.Context, *signaling.Environment) (*messaging.Conn, error) { calls++; return want, nil })
	if err != nil || got != want || calls != 1 {
		t.Fatalf("got %p, error %v, calls %d", got, err, calls)
	}
}

func TestRealmSignalingInvalidRegionNeverDials(t *testing.T) {
	_, err := dialRealmSignaling(context.Background(), "evil.example/path", func(context.Context, *signaling.Environment) (*messaging.Conn, error) {
		t.Fatal("unsafe region dialed")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected invalid-region error")
	}
}

func TestRealmSignalingCanceledContextDoesNotFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	want := &net.DNSError{Name: "signal-francecentral.franchise.minecraft-services.net", IsNotFound: true}
	_, err := dialRealmSignaling(ctx, "FranceCentral", func(context.Context, *signaling.Environment) (*messaging.Conn, error) {
		calls++
		cancel()
		return nil, want
	})
	if err != want || calls != 1 {
		t.Fatalf("error %v, calls %d", err, calls)
	}
}

func TestRealmSignalingRegionalNXDOMAINUsesDiscoveryOnce(t *testing.T) {
	ctx := context.Background()
	want := new(messaging.Conn)
	calls := 0
	got, err := dialRealmSignaling(ctx, "FranceCentral", func(gotCtx context.Context, env *signaling.Environment) (*messaging.Conn, error) {
		calls++
		if gotCtx != ctx {
			t.Fatal("context changed")
		}
		switch calls {
		case 1:
			if env == nil || env.ServiceURI.Hostname() != "signal-francecentral.franchise.minecraft-services.net" {
				t.Fatalf("wrong regional environment: %+v", env)
			}
			return nil, fmt.Errorf("websocket dial: %w", &net.DNSError{Name: env.ServiceURI.Hostname(), Err: "no such host", IsNotFound: true})
		case 2:
			if env != nil {
				t.Fatal("fallback must use service discovery, not a hardcoded host")
			}
			return want, nil
		default:
			t.Fatal("more than one fallback")
			return nil, nil
		}
	})
	if err != nil || got != want || calls != 2 {
		t.Fatalf("got %p, error %v, calls %d; want discovery success after two calls", got, err, calls)
	}
}

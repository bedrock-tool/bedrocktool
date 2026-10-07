package resourcepacks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type stackTestConn struct{ minecraft.IConn }

func (stackTestConn) Expect(...uint32)                {}
func (stackTestConn) WritePacket(packet.Packet) error { return nil }

func TestRemoteStackWaitHonoursAttemptCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewResourcePackHandler(ctx, nil)
	r.Server = stackTestConn{}
	r.clientDone = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- r.OnResourcePackStack(&packet.ResourcePackStack{}) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		close(r.clientDone)
		<-done
		t.Fatal("resource-pack stack callback blocked after attempt cancelled")
	}
}

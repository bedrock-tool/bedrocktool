package proxy

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// closedConn is a client connection that closed with the transport's own
// reason, as a NetherNet connection does when the game quits.
type closedConn struct {
	minecraft.IConn
	ctx context.Context
	err error
}

func (c closedConn) Context() context.Context        { return c.ctx }
func (c closedConn) WritePacket(packet.Packet) error { return c.err }

func TestClientLeft(t *testing.T) {
	sctp := errors.New("write packet: nethernet: SCTP transport closed")
	ctx, cancel := context.WithCancel(context.Background())

	s := &Session{Client: closedConn{ctx: ctx, err: sctp}}
	if s.clientLeft(sctp) {
		t.Error("an error on an open connection taken for the player leaving")
	}
	if err := s.ClientWritePacket(&packet.Text{}); errors.Is(err, net.ErrClosed) {
		t.Errorf("open connection: %v reported as closed", err)
	}

	cancel()
	if !s.clientLeft(sctp) {
		t.Error("closed connection's transport error not taken for the player leaving")
	}
	err := s.ClientWritePacket(&packet.Text{})
	if !errors.Is(err, net.ErrClosed) || !errors.Is(err, sctp) {
		t.Errorf("closed connection: %v, want net.ErrClosed wrapping the transport error", err)
	}
	if !s.clientLeft(net.ErrClosed) {
		t.Error("net.ErrClosed not taken for the player leaving")
	}
}

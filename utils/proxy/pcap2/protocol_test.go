package pcap2

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func protoPayload(p int32) []byte {
	return binary.BigEndian.AppendUint32(nil, uint32(p))
}

func TestCheckProtocol(t *testing.T) {
	if err := checkProtocol(packet.IDRequestNetworkSettings, protoPayload(protocol.CurrentProtocol)); err != nil {
		t.Fatalf("current protocol rejected: %v", err)
	}
	err := checkProtocol(packet.IDRequestNetworkSettings, protoPayload(786))
	var wrong ErrWrongProtocol
	if !errors.As(err, &wrong) || wrong.Protocol != 786 {
		t.Fatalf("old protocol: got %v", err)
	}
	// other packets are never checked
	if err := checkProtocol(packet.IDLogin, protoPayload(786)); err != nil {
		t.Fatalf("login checked: %v", err)
	}
}

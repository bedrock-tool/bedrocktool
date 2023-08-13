package proxy

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type rpHandler struct {
	resourcePacksInfo *packet.ResourcePacksInfo
	Client            minecraft.IConn
	Server            minecraft.IConn
}

func (r *rpHandler) OnClientResponse(conn *minecraft.Conn, pk *packet.ResourcePackClientResponse) error {
	return nil
}

func (r *rpHandler) OnChunkRequest(conn *minecraft.Conn, pk *packet.ResourcePackChunkRequest) error {
	return nil
}

func (r *rpHandler) OnResourcePacksInfo(conn *minecraft.Conn, pk *packet.ResourcePacksInfo) error {
	return r.Client.WritePacket(pk)
}

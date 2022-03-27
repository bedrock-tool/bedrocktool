package main

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func send_popup(conn *minecraft.Conn, text string) {
	conn.WritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text,
	})
}

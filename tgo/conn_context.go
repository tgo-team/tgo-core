package tgo

import (
	"github.com/tgo-team/tgo-core/tgo/packets"
)

type PacketContext struct {
	Packet packets.Packet
	Conn Conn
}

func NewPacketContext(packet packets.Packet,conn Conn) *PacketContext {
	return &PacketContext{
		Packet: packet,
		Conn:conn,
	}
}
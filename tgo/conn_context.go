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

// AuthenticatedContext 已认证的连接的上下文
type AuthenticatedContext struct {
	ClientID uint64
	Conn StatefulConn // 有状态连接 无状态连接不存在连接认证这回事。
}

func NewAuthenticatedContext(clientID uint64,conn StatefulConn) *AuthenticatedContext {
	return &AuthenticatedContext{
		ClientID: clientID,
		Conn:conn,
	}
}
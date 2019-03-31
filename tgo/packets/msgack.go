package packets

import "fmt"

type MsgackPacket struct {
	FixedHeader
	MessageIDs []uint64
}

func NewMsgackPacketWithHeader(fh FixedHeader) *MsgackPacket  {
	m := &MsgackPacket{}
	m.FixedHeader = fh
	return  m
}
func NewMsgackPacket(messageIDs []uint64) *MsgackPacket {
	m := &MsgackPacket{}
	m.PacketType = Msgack
	m.MessageIDs = messageIDs
	return m
}

func (m *MsgackPacket) GetFixedHeader() FixedHeader  {

	return m.FixedHeader
}

func (m *MsgackPacket) String() string {
	str := fmt.Sprintf("%s", m.FixedHeader)
	str += " "
	str += fmt.Sprintf("MessageIDs: %v", m.MessageIDs)
	return str
}
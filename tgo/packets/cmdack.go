package packets

type CmdackPacket struct {
	FixedHeader
	CMD     string  // 命令
	Status  uint16 // 状态
	Payload []byte // 消息内容
}

func NewCmdackPacketWithHeader(fh FixedHeader) *CmdackPacket {
	c := &CmdackPacket{}
	c.FixedHeader = fh
	return c
}

func NewCmdackPacket(cmd string,status uint16, payload []byte) *CmdackPacket {

	return &CmdackPacket{CMD: cmd,Status:status, Payload: payload, FixedHeader: FixedHeader{PacketType: Cmd}}
}
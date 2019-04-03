package packets

type CmdackPacket struct {
	FixedHeader
	CMD     string  // 命令
	Status  uint16 // 状态
	Payload []byte // 消息内容
}

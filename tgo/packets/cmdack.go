package packets

type CmdackPacket struct {
	FixedHeader
	CMD       string
	TokenFlag bool // token标识 true表示存在token false为不存在
	Token     string // token字符串
	Payload   []byte // 消息内容
}

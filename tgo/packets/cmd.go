package packets

import "fmt"

type CMDPacket struct {
	FixedHeader
	CMD       string
	TokenFlag bool // token标识 true表示存在token false为不存在
	Token     string // token字符串
	Payload   []byte // 消息内容
}

func NewCMDPacketWithHeader(fh FixedHeader) *CMDPacket {
	c := &CMDPacket{}
	c.FixedHeader = fh
	return c
}

func NewCMDPacket(cmd string, payload []byte) *CMDPacket {

	return &CMDPacket{CMD: cmd, Payload: payload, FixedHeader: FixedHeader{PacketType: CMD}}
}

func (c *CMDPacket) GetFixedHeader() FixedHeader {

	return c.FixedHeader
}

func (c *CMDPacket) String() string {
	str := fmt.Sprintf("%s", c.FixedHeader)
	str += " "
	str += fmt.Sprintf("CMD: %d TokenFlag: %v Token: %v Payload:  %s", c.CMD,c.TokenFlag,c.Token, string(c.Payload))
	return str
}

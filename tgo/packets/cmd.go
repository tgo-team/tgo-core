package packets

import "fmt"

type CmdPacket struct {
	FixedHeader
	CMD       string
	TokenFlag bool // token标识 true表示存在token false为不存在
	Token     string // token字符串
	Payload   []byte // 消息内容
}

func NewCmdPacketWithHeader(fh FixedHeader) *CmdPacket {
	c := &CmdPacket{}
	c.FixedHeader = fh
	return c
}

func NewCmdPacket(cmd string, payload []byte) *CmdPacket {

	return &CmdPacket{CMD: cmd, Payload: payload, FixedHeader: FixedHeader{PacketType: Cmd}}
}

func (c *CmdPacket) GetFixedHeader() FixedHeader {

	return c.FixedHeader
}

func (c *CmdPacket) String() string {
	str := fmt.Sprintf("%s", c.FixedHeader)
	str += " "
	str += fmt.Sprintf("CMD: %s TokenFlag: %v Token: %v Payload:  %s", c.CMD,c.TokenFlag,c.Token, string(c.Payload))
	return str
}

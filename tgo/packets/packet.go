package packets

import (
	"encoding/binary"
	"fmt"
	"io"
)

type PacketCodec interface {
	//Decode 解码
	Decode(reader io.Reader) (Packet, error)
	//Encode 编码
	Encode(msg Packet) ([]byte, error)
}

type Packet interface {
	GetFixedHeader() FixedHeader
	String() string
}

type PacketType int
type FixedHeader struct {
	PacketType      PacketType
	Dup             bool
	Qos             byte
	Retain          bool
	RemainingLength int
	From            uint64 // 发送方ID （如果是TCP可不用参与编码和解码）
}

func (fh FixedHeader) String() string {
	return fmt.Sprintf("%s: From: %d dup: %t qos: %d retain: %t rLength: %d", PacketNames[uint8(fh.PacketType)], fh.From, fh.Dup, fh.Qos, fh.Retain, fh.RemainingLength)
}

const (
	Connect     PacketType = 1
	Connack     PacketType = 2
	Message     PacketType = 3
	Msgack      PacketType = 4
	Pingreq     PacketType = 5 // 心跳请求
	Pingresp    PacketType = 6 // 心跳返回
	Cmd         PacketType = 7 // 命令
	Cmdack       PacketType = 8 // 命令回执
)

var PacketNames = map[uint8]string{
	1:  "CONNECT",
	2:  "CONNACK",
	3:  "MESSAGE",
	4:  "MSGACK",
	5:  "PINGREQ",
	6: "PINGRESP",
	7: "CMD",
	8: "CMDACK",
}

func BoolToByte(b bool) byte {
	switch b {
	case true:
		return 1
	default:
		return 0
	}
}

func DecodeByte(b io.Reader) byte {
	num := make([]byte, 1)
	b.Read(num)
	return num[0]
}

func DecodeUint16(b io.Reader) uint16 {
	num := make([]byte, 2)
	b.Read(num)
	return binary.BigEndian.Uint16(num)
}

func DecodeUint32(b io.Reader) uint32 {
	num := make([]byte, 4)
	b.Read(num)
	return binary.BigEndian.Uint32(num)
}

func DecodeUint64(b io.Reader) uint64 {
	num := make([]byte, 8)
	b.Read(num)
	return binary.BigEndian.Uint64(num)
}

func EncodeUint16(num uint16) []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, num)
	return bytes
}

func EncodeUint32(num uint32) []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, num)
	return bytes
}

func EncodeUint64(num uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, num)
	return bytes
}

func EncodeString(field string) []byte {

	return EncodeBytes([]byte(field))
}

func DecodeString(b io.Reader) string {
	return string(DecodeBytes(b))
}

func DecodeBytes(b io.Reader) []byte {
	fieldLength := DecodeUint16(b)
	field := make([]byte, fieldLength)
	b.Read(field)
	return field
}

func EncodeBytes(field []byte) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, field...)
}

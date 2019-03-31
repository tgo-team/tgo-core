package tgo

import (
	"bytes"
	"encoding/binary"
	"github.com/tgo-team/tgo-core/tgo/packets"
)

type Client struct {
	ClientID uint64
	Password string
}

func NewClient(clientID uint64, password string) *Client {
	return &Client{ClientID: clientID, Password: password}
}
func (c *Client) MarshalBinary() (data []byte, err error) {
	var body bytes.Buffer
	body.Write(packets.EncodeUint64(c.ClientID))
	body.Write(packets.EncodeString(c.Password))
	return body.Bytes(), nil
}

func (c *Client) UnmarshalBinary(data []byte) error {
	c.ClientID = binary.BigEndian.Uint64(data[:8])
	c.Password = packets.DecodeString(bytes.NewBuffer(data[8:]))
	return nil
}

type Storage interface {
	// ------ 消息操作 -----
	StorageMsgChan() chan *MsgContext                                                  // 读取消息
	AddMsgInChannel(msg *Msg, channelID uint64) error                                  // 保存消息
	RemoveMsgInChannel(messageIDs []uint64, channelID uint64) error                    // 移除管道里的消息
	GetMsgInChannel(channelID uint64, pageIndex int64, pageSize int64) ([]*Msg, error) // 获取管道内的消息集合(分页查询)
	// ------ 管道操作 -----
	AddChannel(c *ChannelModel) error                   // 保存管道
	GetChannel(channelID uint64) (*ChannelModel, error) // 获取管道
	Bind(clientID uint64, channelID uint64) error       // 绑定消费者和通道的关系
	GetClientIDs(channelID uint64) ([]uint64, error)    // 获取所属管道所有的客户端
	// ------ 客户端相关 -----
	AddClient(c *Client) error                  // 添加客户端
	GetClient(clientID uint64) (*Client, error) // 获取客户端
}

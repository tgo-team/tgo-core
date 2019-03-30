package tgo

import (
	"fmt"
	"github.com/tgo-team/tgo-core/tgo/packets"
	"github.com/tgo-team/tgo-core/tgo/pqueue"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ------------ channel ----------------
type ChannelType int

const (
	ChannelTypePerson ChannelType = iota // 个人管道
	ChannelTypeGroup                     // 群组管道
)


type Channel struct {
	ChannelID    uint64
	ChannelType  ChannelType
	MessageCount uint64
	sync.RWMutex
	Ctx *Context

	inFlightMessages map[uint64]*Msg
	inFlightPQ       pqueue.PriorityQueue
	inFlightMutex    sync.Mutex

	connMap map[uint64]*Conn

	deliveryMsgChan chan *Msg
	waitGroup               WaitGroupWrapper
}

func NewChannel(channelID uint64, channelType ChannelType, ctx *Context) *Channel {
	c := &Channel{
		connMap:     map[uint64]*Conn{},
		ChannelID:   channelID,
		ChannelType: channelType,
		Ctx:         ctx,
		deliveryMsgChan: make(chan *Msg,1024),
	}
	c.waitGroup.Wrap(func() {
		c.startDeliveryMsg()
	})
	//c.initPQ()
	return c
}
func (c *Channel) initPQ() {
	pqSize := int(math.Max(1, float64(c.Ctx.TGO.GetOpts().MemQueueSize)/10))

	c.inFlightMutex.Lock()
	c.inFlightMessages = make(map[uint64]*Msg)
	c.inFlightPQ = pqueue.New(pqSize)
	c.inFlightMutex.Unlock()
}

func (c *Channel) PutMsg(msg *Msg) error {
	msgContext := NewMsgContext(msg, c.ChannelID)
	select {
	case c.Ctx.TGO.memoryMsgChan <- msgContext:
	default:
		c.Warn("内存消息已满，进入持久化存储！")
		err := c.Ctx.TGO.Storage.AddMsg(msgContext)
		if err != nil {
			return err
		}
	}
	atomic.AddUint64(&c.MessageCount, 1)
	return nil
}

func (c *Channel) DeliveryMsgChan() chan *Msg  {
	return c.deliveryMsgChan
}

// DeliveryMsg 投递消息
func (c *Channel) startDeliveryMsg()  {
	for {
		select {
		case msg :=<- c.deliveryMsgChan:
			c.deliveryMsg(msg)
		}
	}
}

func (c *Channel) deliveryMsg(msg *Msg)  {
	c.Debug("开始投递消息[%d]！",msg.MessageID)
	clientIDs,err := c.Ctx.TGO.Storage.GetClientIDs(c.ChannelID)
	if err!=nil {
		c.Error("获取管道[%d]的客户端ID集合失败！ -> %v",c.ChannelID,err)
		return
	}
	if clientIDs==nil || len(clientIDs)<=0 {
		c.Warn("Channel[%d]里没有客户端！",c.ChannelID)
		return
	}
	for _,clientID :=range clientIDs {
		if clientID == msg.From { // 不发送给自己
			continue
		}
		if clientID == c.ChannelID && c.ChannelType == ChannelTypePerson { // 如果Channel是当前用户的将直接发送消息给用户
			online := IsOnline(clientID)
			if online {
				conn := c.Ctx.TGO.ConnManager.GetConn(clientID)
				if conn!=nil {
					msgPacket := packets.NewMessagePacket(msg.MessageID,c.ChannelID,msg.Payload)
					msgPacket.From = msg.From
					msgPacketData,err := c.Ctx.TGO.GetOpts().Pro.EncodePacket(msgPacket)
					if err!=nil {
						c.Error("编码消息[%d]数据失败！-> %v",msg.MessageID,err)
						continue
					}
					_,err = conn.Write(msgPacketData)
					if err!=nil {
						c.Error("写入消息[%d]数据失败！-> %v",msg.MessageID,err)
						continue
					}
				}else{
					c.Warn("客户端[%d]已是上线状态，但是没有找到客户端的连接！",clientID)
				}
			}
		}else{ // 如果Channel不是当前用户的，将消息存到用户对应的管道内
			personChannel,err := c.Ctx.TGO.GetChannel(clientID)
			if err!=nil {
				c.Error("获取Channel[%d]失败！-> %v",clientID,err)
				continue
			}
			if personChannel==nil {
				c.Warn("没有查询到通道Channel[%d]！",clientID)
				continue
			}
			err = personChannel.PutMsg(msg)
			//TODO PutMsg 发生错误 怎么处理 (暂时不考虑，后面可以进行重新同步之类的反正消息是收到了存储到了管道内，只是没下发到用户的管道内)
			// TODO 出现这种情况会出现客户端丢消息情况
			if err!=nil {
				c.Warn("将消息[%d]存放到用户[%d]的管道[%d]失败！-> %v",msg.MessageID,clientID,c.ChannelID,err)
			}
		}
	}
}

func (c *Channel) StartInFlightTimeout(msg *Msg, clientID int64, timeout time.Duration) error {
	now := time.Now()
	item := &pqueue.Item{Value: msg, Priority: now.Add(timeout).UnixNano()}
	c.addToInFlightPQ(item)
	return nil
}

func (c *Channel) addToInFlightPQ(item *pqueue.Item) {
	c.inFlightMutex.Lock()
	c.inFlightPQ.Push(item)
	c.inFlightMutex.Unlock()
}

func (c *Channel) String() string {
	return fmt.Sprintf("ChannelID: %d ChannelType: %d MessageCount: %d", c.ChannelID, c.ChannelType, c.MessageCount)
}

// ---------- log --------------

func (c *Channel) Info(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Info(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.ChannelID)+f, args...)
	return
}

func (c *Channel) Error(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Error(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.ChannelID)+f, args...)
	return
}

func (c *Channel) Debug(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Debug(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.ChannelID)+f, args...)
	return
}

func (c *Channel) Warn(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Warn(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.ChannelID)+f, args...)
	return
}

func (c *Channel) Fatal(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Fatal(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.ChannelID)+f, args...)
	return
}

func (c *Channel) getLogPrefix() string {
	return "【Chanel】"
}

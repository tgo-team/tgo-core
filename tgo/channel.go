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

const (
	ChannelTypePerson int = iota // 个人管道
	ChannelTypeGroup                     // 群组管道
)

type ChannelModel struct {
	ChannelID uint64
	ChannelType int
}

func NewChannelModel(channelID uint64,channelType int) *ChannelModel  {

	return &ChannelModel{
		ChannelID: channelID,
		ChannelType:channelType,
	}
}

func (cm *ChannelModel) NewChannel(ctx *Context) Channel  {
	switch cm.ChannelType {
	case ChannelTypePerson:
		return NewPersonChannel(cm.ChannelID,cm,ctx)
	case ChannelTypeGroup:
		return NewGroupChannel(cm.ChannelID,cm,ctx)
	default:
		ctx.TGO.GetOpts().Log.Warn("不支持的通道类型[%d]",cm.ChannelType)
	}
	return nil
}

type Channel interface {
	Model() *ChannelModel
	// PutMsg 将消息放入管道 （存储消息和投递消息）
	PutMsg(msg *Msg) error
	// DeliveryMsgChan 投递消息的chan （只投递不存储消息）
	DeliveryMsgChan() chan *Msg
}

// 群组管道
type GroupChannel struct {
	channelID    uint64
	MessageCount uint64
	sync.RWMutex
	Ctx *Context
	model *ChannelModel

	inFlightMessages map[uint64]*Msg
	inFlightPQ       pqueue.PriorityQueue
	inFlightMutex    sync.Mutex

	connMap map[uint64]*Conn

	deliveryMsgChan chan *Msg
	waitGroup       WaitGroupWrapper
	started         bool // 是否已开始
}

func NewGroupChannel(channelID uint64,model *ChannelModel,ctx *Context) *GroupChannel {
	c := &GroupChannel{
		connMap:         map[uint64]*Conn{},
		channelID:       channelID,
		deliveryMsgChan: make(chan *Msg, 1024),
		Ctx:ctx,
		model:model,
	}
	c.waitGroup.Wrap(func() {
		c.startDeliveryMsg()
	})
	//c.initPQ()
	return c
}
func (c *GroupChannel) initPQ() {
	pqSize := int(math.Max(1, float64(c.Ctx.TGO.GetOpts().MemQueueSize)/10))

	c.inFlightMutex.Lock()
	c.inFlightMessages = make(map[uint64]*Msg)
	c.inFlightPQ = pqueue.New(pqSize)
	c.inFlightMutex.Unlock()
}

func (c *GroupChannel) SetContext(ctx *Context) {
	c.Ctx = ctx
}

func (c *GroupChannel) PutMsg(msg *Msg) error {
	err := c.Ctx.TGO.Storage.AddMsgInChannel(msg,c.channelID)
	if err != nil {
		return err
	}
	atomic.AddUint64(&c.MessageCount, 1)
	return nil
}

func (c *GroupChannel) DeliveryMsgChan() chan *Msg {
	return c.deliveryMsgChan
}

func (c *GroupChannel) Model() *ChannelModel {
	return c.model
}

// DeliveryMsg 投递消息
func (c *GroupChannel) startDeliveryMsg() {
	for {
		select {
		case msg := <-c.deliveryMsgChan:
			c.deliveryMsg(msg)
		}
	}
}

func (c *GroupChannel) deliveryMsg(msg *Msg) {
	c.Debug("开始投递消息[%d]！", msg.MessageID)
	clientIDs, err := c.Ctx.TGO.Storage.GetClientIDs(c.channelID)
	if err != nil {
		c.Error("获取管道[%d]的客户端ID集合失败！ -> %v", c.channelID, err)
		return
	}
	if clientIDs == nil || len(clientIDs) <= 0 {
		c.Warn("Channel[%d]里没有客户端！", c.channelID)
		return
	}
	for _, clientID := range clientIDs {
		personChannel, err := c.Ctx.TGO.GetChannel(clientID)
		if err != nil {
			c.Error("获取Channel[%d]失败！-> %v", clientID, err)
			continue
		}
		if personChannel == nil {
			c.Warn("没有查询到通道Channel[%d]！", clientID)
			continue
		}
		err = personChannel.PutMsg(msg)
		//TODO PutMsg 发生错误 怎么处理 (暂时不考虑，后面可以进行重新同步之类的反正消息是收到了存储到了管道内，只是没下发到用户的管道内)
		// TODO 出现这种情况会出现客户端丢消息情况
		if err != nil {
			c.Warn("将消息[%d]存放到用户[%d]的管道[%d]失败！-> %v", msg.MessageID, clientID, c.channelID, err)
		}
	}
}

func (c *GroupChannel) StartInFlightTimeout(msg *Msg, clientID int64, timeout time.Duration) error {
	now := time.Now()
	item := &pqueue.Item{Value: msg, Priority: now.Add(timeout).UnixNano()}
	c.addToInFlightPQ(item)
	return nil
}

func (c *GroupChannel) addToInFlightPQ(item *pqueue.Item) {
	c.inFlightMutex.Lock()
	c.inFlightPQ.Push(item)
	c.inFlightMutex.Unlock()
}

func (c *GroupChannel) String() string {
	return fmt.Sprintf("ChannelID: %d MessageCount: %d", c.channelID, c.MessageCount)
}



type PersonChannel struct {
	channelID    uint64
	MessageCount uint64
	sync.RWMutex
	Ctx *Context
	model *ChannelModel

	inFlightMessages map[uint64]*Msg
	inFlightPQ       pqueue.PriorityQueue
	inFlightMutex    sync.Mutex

	connMap map[uint64]*Conn

	deliveryMsgChan chan *Msg
	waitGroup       WaitGroupWrapper
}

func NewPersonChannel(channelID uint64,model *ChannelModel,ctx *Context) *PersonChannel {
	c := &PersonChannel{
		connMap:         map[uint64]*Conn{},
		channelID:       channelID,
		deliveryMsgChan: make(chan *Msg, 1024),
		Ctx:ctx,
		model:model,
	}
	c.waitGroup.Wrap(func() {
		c.startDeliveryMsg()
	})
	//c.initPQ()
	return c
}
func (c *PersonChannel) initPQ() {
	pqSize := int(math.Max(1, float64(c.Ctx.TGO.GetOpts().MemQueueSize)/10))

	c.inFlightMutex.Lock()
	c.inFlightMessages = make(map[uint64]*Msg)
	c.inFlightPQ = pqueue.New(pqSize)
	c.inFlightMutex.Unlock()
}

func (c *PersonChannel) PutMsg(msg *Msg) error {
	err := c.Ctx.TGO.Storage.AddMsgInChannel(msg,c.channelID)
	if err != nil {
		return err
	}
	atomic.AddUint64(&c.MessageCount, 1)
	return nil
}

func (c *PersonChannel) DeliveryMsgChan() chan *Msg {
	return c.deliveryMsgChan
}

func (c *PersonChannel) Model() *ChannelModel {
	return c.model
}

// DeliveryMsg 投递消息
func (c *PersonChannel) startDeliveryMsg() {
	for {
		select {
		case msg := <-c.deliveryMsgChan:
			c.deliveryMsg(msg)
		}
	}
}

func (c *PersonChannel) deliveryMsg(msg *Msg) {
	c.Debug("开始投递消息[%d]！", msg.MessageID)
	clientIDs, err := c.Ctx.TGO.Storage.GetClientIDs(c.channelID)
	if err != nil {
		c.Error("获取管道[%d]的客户端ID集合失败！ -> %v", c.channelID, err)
		return
	}
	if clientIDs == nil || len(clientIDs) <= 0 {
		c.Warn("Channel[%d]里没有客户端！", c.channelID)
		return
	}
	for _, clientID := range clientIDs {
		if clientID == msg.From { // 不发送给自己
			continue
		}
		online := IsOnline(clientID)
		if online {
			conn := c.Ctx.TGO.ConnManager.GetConn(clientID)
			if conn != nil {
				msgPacket := packets.NewMessagePacket(msg.MessageID, c.channelID, msg.Payload)
				msgPacket.From = msg.From
				msgPacketData, err := c.Ctx.TGO.GetOpts().Pro.EncodePacket(msgPacket)
				if err != nil {
					c.Error("编码消息[%d]数据失败！-> %v", msg.MessageID, err)
					continue
				}
				_, err = conn.Write(msgPacketData)
				if err != nil {
					c.Error("写入消息[%d]数据失败！-> %v", msg.MessageID, err)
					continue
				}
			} else {
				c.Warn("客户端[%d]已是上线状态，但是没有找到客户端的连接！", clientID)
			}
		}else {
			c.Debug("客户端[%d]没在线！",clientID)
		}
	}
}

func (c *PersonChannel) StartInFlightTimeout(msg *Msg, clientID int64, timeout time.Duration) error {
	now := time.Now()
	item := &pqueue.Item{Value: msg, Priority: now.Add(timeout).UnixNano()}
	c.addToInFlightPQ(item)
	return nil
}

func (c *PersonChannel) addToInFlightPQ(item *pqueue.Item) {
	c.inFlightMutex.Lock()
	c.inFlightPQ.Push(item)
	c.inFlightMutex.Unlock()
}

func (c *PersonChannel) String() string {
	return fmt.Sprintf("ChannelID: %d MessageCount: %d", c.channelID, c.MessageCount)
}

// ---------- log --------------

func (c *GroupChannel) Info(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Info(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *GroupChannel) Error(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Error(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *GroupChannel) Debug(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Debug(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *GroupChannel) Warn(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Warn(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *GroupChannel) Fatal(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Fatal(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *GroupChannel) getLogPrefix() string {
	return "【Chanel-Group】"
}

func (c *PersonChannel) Info(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Info(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *PersonChannel) Error(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Error(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *PersonChannel) Debug(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Debug(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *PersonChannel) Warn(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Warn(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *PersonChannel) Fatal(f string, args ...interface{}) {
	c.Ctx.TGO.GetOpts().Log.Fatal(fmt.Sprintf("%s[%d] -> ", c.getLogPrefix(), c.channelID)+f, args...)
	return
}

func (c *PersonChannel) getLogPrefix() string {
	return "【Chanel-Person】"
}

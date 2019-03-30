package tgo

import (
	"fmt"
	"github.com/tgo-team/tgo-core/tgo/packets"
	"math"
	"reflect"
	"runtime"
	"sync"
)

type HandlerFunc func(*MContext)
type AuthHandlerFunc func(MContext) error
type HandlersChain []HandlerFunc

type Route struct {
	pool            sync.Pool
	handlers        HandlersChain
	ctx             *Context
	matchHandlerMap map[string]HandlerFunc
}

func NewRoute(ctx *Context) *Route {
	r := &Route{
		handlers:        HandlersChain{},
		ctx:             ctx,
		matchHandlerMap: make(map[string]HandlerFunc, 0),
	}
	return r
}

func (r *Route) handle(context *MContext) {
	context.Next()
}

func (r *Route) Serve(context *MContext) {
	context.Ctx = r.ctx
	context.handlers = r.handlers
	r.handle(context)

	if context.Packet()!=nil && !context.IsAborted(){
		// 包类型匹配
		packetType := context.Packet().GetFixedHeader().PacketType
		typePath := fmt.Sprintf("type:%d",packetType)
		matchFunc := r.matchHandlerMap[typePath]
		if matchFunc!=nil {
			matchFunc(context)
		}
		// cmd类型匹配
		if packetType == packets.CMD {
			cmd := context.Packet().(*packets.CMDPacket).CMD
			cmdPath := fmt.Sprintf("cmd:%d",cmd)
			matchFunc := r.matchHandlerMap[cmdPath]
			if matchFunc!=nil {
				matchFunc(context)
			}
		}
	}

}

func (r *Route) Use(handles ...HandlerFunc) *Route {
	r.handlers = append(r.handlers, handles...)
	return r
}

func (r *Route) Match(match string, handler HandlerFunc) {
	r.matchHandlerMap[match] = handler
}

const abortIndex int8 = math.MaxInt8 / 2

type MContext struct {
	packetContext *PacketContext
	index       int8
	handlers    HandlersChain
	sync.RWMutex
	Ctx *Context
}

var pool = sync.Pool{
	New: func() interface{} {
		return allocateContext()
	},
}

func GetMContext(packetContext *PacketContext) *MContext {
	mContext := pool.Get().(*MContext)
	mContext.reset()
	mContext.packetContext = packetContext
	return mContext
}

func allocateContext() *MContext {
	return &MContext{index: -1, handlers: nil, packetContext: nil, RWMutex: sync.RWMutex{}}
}

func (m *MContext) Next() {
	m.index++
	for ; m.index < int8(len(m.handlers)); m.index++ {
		m.handlers[m.index](m)
	}
}

func (m *MContext) Packet() packets.Packet {
	return m.packetContext.Packet
}

// CMDPacket 获取CMD包
func (m *MContext) CMDPacket() *packets.CMDPacket {
	return m.packetContext.Packet.(*packets.CMDPacket)
}

func (m *MContext) PacketType() packets.PacketType {

	return m.Packet().GetFixedHeader().PacketType
}

func (m *MContext) Conn() Conn {
	return m.packetContext.Conn
}

func (m *MContext) Storage() Storage {
	return m.Ctx.TGO.Storage
}


func (m *MContext) Msg() *Msg {
	messagePacket, ok := m.packetContext.Packet.(*packets.MessagePacket)
	if ok {
		msg := NewMsg(messagePacket.MessageID,messagePacket.From,messagePacket.Payload)
		msg.MessageID = messagePacket.MessageID
		msg.Payload = messagePacket.Payload
		return msg
	}
	return nil
}

func (m *MContext) Abort() {
	m.index = abortIndex
}

func (m *MContext) IsAborted() bool {
	return m.index >= abortIndex
}

func (m *MContext) ReplyPacket(packet packets.Packet)  {
	data, err := m.Ctx.TGO.GetOpts().Pro.EncodePacket(packet)
	if err != nil {
		m.Error("编码出错！-> %v",err)
		return
	}
	_, err = m.Conn().Write(data)
	if err != nil {
		m.Error("写入数据出错！-> %v",err)
	}
	return
}

func (m *MContext) GetChannel(channelID uint64) (Channel,error) {

	return m.Ctx.TGO.GetChannel(channelID)
}

func (m *MContext) reset() {
	m.Lock()
	defer m.Unlock()
	m.index = -1
	m.packetContext = nil
	m.handlers = nil
}

func (m *MContext) current() HandlerFunc {
	if m.index < int8(len(m.handlers)) && m.index != -1 {
		return m.handlers[m.index]
	}
	return nil
}

// ---------- log --------------
func (m *MContext) Info(f string, args ...interface{}) {
	funcName := m.currentHandleName()
	m.Ctx.TGO.GetOpts().Log.Info(fmt.Sprintf("%s[%s] -> ", m.getLogPrefix(), funcName)+f, args...)
	return
}

func (m *MContext) Error(f string, args ...interface{}) {
	funcName := m.currentHandleName()
	m.Ctx.TGO.GetOpts().Log.Error(fmt.Sprintf("%s[%s] -> ", m.getLogPrefix(), funcName)+f, args...)
	return
}

func (m *MContext) Debug(f string, args ...interface{}) {
	funcName := m.currentHandleName()
	m.Ctx.TGO.GetOpts().Log.Debug(fmt.Sprintf("%s[%s] -> ", m.getLogPrefix(), funcName)+f, args...)
	return
}

func (m *MContext) Warn(f string, args ...interface{}) {
	funcName := m.currentHandleName()
	m.Ctx.TGO.GetOpts().Log.Warn(fmt.Sprintf("%s[%s] -> ", m.getLogPrefix(), funcName)+f, args...)
	return
}

func (m *MContext) Fatal(f string, args ...interface{}) {
	funcName := m.currentHandleName()
	m.Ctx.TGO.GetOpts().Log.Fatal(fmt.Sprintf("%s[%s] -> ", m.getLogPrefix(), funcName)+f, args...)
	return
}

func (m *MContext) getLogPrefix() string {
	return "【Route】"
}

func (m *MContext) currentHandleName() string {
	funcName := ""
	if m.current() != nil {
		funcName = runtime.FuncForPC(reflect.ValueOf(m.current()).Pointer()).Name()
	}

	return funcName
}

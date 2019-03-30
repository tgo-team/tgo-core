package tgo

import (
	"github.com/tgo-team/tgo-core/tgo/packets"
	"sync"
	"sync/atomic"
	"time"
)

type TGO struct {
	Servers []Server
	opts    atomic.Value // options
	*Route
	exitChan                chan int
	waitGroup               WaitGroupWrapper
	Storage                 Storage // storage msg
	monitor                 Monitor // Monitor
	channelMap              map[uint64]Channel
	AcceptConnChan          chan Conn // 接受连接
	AcceptPacketChan        chan *PacketContext
	AcceptConnExitChan      chan Conn                  // 接受连接退出
	AcceptAuthenticatedChan chan *AuthenticatedContext // 接受已认证了的conn
	ConnManager             *connManager
	sync.RWMutex
}

func New(opts *Options) *TGO {
	tg := &TGO{
		exitChan:                make(chan int, 0),
		channelMap:              map[uint64]Channel{},
		AcceptPacketChan:        make(chan *PacketContext, 1024),
		AcceptConnChan:          make(chan Conn, 1024),
		AcceptConnExitChan:      make(chan Conn, 1024),
		AcceptAuthenticatedChan: make(chan *AuthenticatedContext, 1024),
		ConnManager:             newConnManager(),
	}

	lg := NewLog(opts.LogLevel)
	if lg != nil {
		opts.Log =lg
	}
	//if opts.Monitor == nil {
	//	opts.Monitor = tg
	//}
	tg.storeOpts(opts)

	ctx := &Context{
		TGO: tg,
	}

	// server
	tg.Servers = GetServers(ctx)
	if tg.Servers == nil {
		opts.Log.Fatal("请先配置Server！")
	}

	// route
	tg.Route = NewRoute(ctx)

	// storage
	tg.Storage = NewStorage(ctx) // new storage
	if tg.Storage == nil {
		opts.Log.Fatal("请先配置存储！")
	}

	tg.waitGroup.Wrap(tg.msgLoop)
	return tg
}

func (t *TGO) Start() error {
	for _, server := range t.Servers {
		err := server.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TGO) Stop() error {
	close(t.exitChan)
	close(t.AcceptPacketChan)
	close(t.AcceptAuthenticatedChan)
	close(t.AcceptConnChan)
	close(t.AcceptConnExitChan)
	for _, server := range t.Servers {
		err := server.Stop()
		if err != nil {
			return err
		}
	}
	t.waitGroup.Wait()
	t.Info("TGO -> 退出")
	return nil
}

func (t *TGO) storeOpts(opts *Options) {
	t.opts.Store(opts)
}

func (t *TGO) GetOpts() *Options {
	return t.opts.Load().(*Options)
}

func (t *TGO) msgLoop() {
	for {
		select {
		case conn := <-t.AcceptConnChan: // 接受到连接请求
			packet, err := t.GetOpts().Pro.DecodePacket(conn)
			if err != nil {
				t.Error("解析连接数据失败！-> %v", err)
				continue
			}
			if packet.GetFixedHeader().PacketType != packets.Connect {
				t.Error("包类型[%d]错误！发起连接后的第一个包必须为Connect包！", packet.GetFixedHeader().PacketType)
				continue
			}
			t.AcceptPacketChan <- NewPacketContext(packet, conn)
		case authenticatedContext := <-t.AcceptAuthenticatedChan: // 连接已认证
			if authenticatedContext != nil {
				t.Debug("连接[%v]认证成功！", authenticatedContext.Conn)
				channelID := authenticatedContext.ClientID
				t.ConnManager.AddConn(authenticatedContext.ClientID, authenticatedContext.Conn)
				channel, err := t.GetChannel(channelID)
				if err != nil {
					t.Error("获取管道[%d]失败！-> %v", channelID, err)
					continue
				}
				if channel == nil {
					t.Error("管道[%d]不存在！", channelID)
					continue
				}
				// 开始推送离线消息
				t.waitGroup.Wrap(func() {
					t.pushOfflineMsg(authenticatedContext.ClientID,authenticatedContext.Conn)
				})
			}
		case packetContext := <-t.AcceptPacketChan: // 接受到包请求
			if packetContext != nil {
				t.Debug("收到[%v]的包 ->  %v", packetContext.Conn, packetContext.Packet)
				t.Serve(GetMContext(packetContext))
			} else {
				t.Warn("Receive the message is nil")
			}
		case msgContext := <-t.Storage.StorageMsgChan(): // 消息存储成功
			if msgContext != nil {
				channel, err := t.GetChannel(msgContext.ChannelID())
				if err != nil {
					t.Error("获取管道[%d]失败！-> %v", msgContext.ChannelID(), err)
					continue
				}
				if channel == nil {
					t.Error("管道[%d]不存在！", msgContext.ChannelID())
					continue
				}
				channel.DeliveryMsgChan() <- msgContext.msg
			}
		case conn := <-t.AcceptConnExitChan: // 连接退出
			if conn != nil {
				t.Debug("连接[%v]退出！", conn)
				cn, ok := conn.(StatefulConn)
				if ok {
					Online(cn.GetID(),OFFLINE)
					t.ConnManager.RemoveConn(cn.GetID())
				}

			}
		case <-t.exitChan:
			goto exit

		}
		println("test---")
	}
exit:
	t.Debug("停止收取消息。")
}

// GetChannel 通过[channelID]获取管道信息
func (t *TGO) GetChannel(channelID uint64) (Channel, error) {
	defer  t.Unlock()
	t.Lock()
	channel, ok := t.channelMap[channelID]
	if !ok {
		channelModel, err := t.Storage.GetChannel(channelID)
		if err != nil {
			return nil, err
		}
		if channelModel != nil {
			channel = channelModel.NewChannel(t.ctx)
			t.channelMap[channelID] =channel
			if err!=nil {
				return nil,err
			}
			return channel,nil
		}
	}
	return channel, nil
}

// pushOfflineMsg 推送离线消息
func (t *TGO) pushOfflineMsg(clientID uint64, conn Conn) {
	channel, err := t.GetChannel(clientID)
	if err != nil {
		t.Error("查询连接[%v]的Channel失败！-> %v", conn, err)
		return
	}
	if channel == nil {
		t.Warn("客户端[%v]对应的管道不存在！", clientID)
		return
	}

	var currentPageIndex int64= 1 // 当前页码
	var pageSize int64 = 100 // 每页数据量
	var maxPageIndex int64= 1000 // 最大页码数（TODO: 超过最大页码不管有没有推送完离线消息都终止，所以最大页码下标尽量设置大点）
	startPushTimeMill := time.Now().UnixNano()/(1000*1000) // 开始push时间 毫秒

	for currentPageIndex = 1;currentPageIndex<maxPageIndex;currentPageIndex ++ {
		msgList, err := t.Storage.GetMsgWithChannel(channel.Model().ChannelID, currentPageIndex, pageSize)
		if err != nil {
			t.Error("获取管道[%d]的消息失败！-> %v", channel.Model().ChannelID, err)
			return
		}
		for _, msg := range msgList {
			if msg.Timestamp > startPushTimeMill { // 如果消息时间大于开始push时间，说明消息是在线消息（因为只有连接后才会开始push）所以退出离线推送
				goto exit
			}
			channel.DeliveryMsgChan() <- msg
		}
		if int64(len(msgList)) < pageSize {
			goto exit
		}
	}

	exit:
		t.Debug("客户端[%v]的离线消息推送完成！",clientID)

}

package tgo

import (
	"github.com/tgo-team/tgo-core/tgo/packets"
	"sync/atomic"
)

type TGO struct {
	Servers []Server
	opts    atomic.Value // options
	*Route
	exitChan                chan int
	waitGroup               WaitGroupWrapper
	Storage                 Storage // storage msg
	monitor                 Monitor // Monitor
	channelMap              map[uint64]*Channel
	memoryMsgChan           chan *MsgContext
	AcceptConnChan          chan Conn // 接受连接
	AcceptPacketChan        chan *PacketContext
	AcceptConnExitChan      chan Conn                  // 接受连接退出
	AcceptAuthenticatedChan chan *AuthenticatedContext // 接受已认证了的conn
	ConnManager             *connManager
}

func New(opts *Options) *TGO {
	tg := &TGO{
		exitChan:                make(chan int, 0),
		channelMap:              map[uint64]*Channel{},
		memoryMsgChan:           make(chan *MsgContext, opts.MemQueueSize),
		AcceptPacketChan:        make(chan *PacketContext, 1024),
		AcceptConnChan:          make(chan Conn, 1024),
		AcceptConnExitChan:      make(chan Conn, 1024),
		AcceptAuthenticatedChan: make(chan *AuthenticatedContext, 1024),
		ConnManager:             newConnManager(),
	}
	if opts.Log == nil {
		opts.Log = NewLog(opts.LogLevel)
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
				t.ConnManager.AddConn(authenticatedContext.ClientID, authenticatedContext.Conn)
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
				t.waitGroup.Add(1)
				go func(msgContext *MsgContext) {
					channel.DeliveryMsg(msgContext)
					t.waitGroup.Done()
				}(msgContext)
			}
		case msgContext := <-t.memoryMsgChan:
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

				t.waitGroup.Add(1)
				go func(msgContext *MsgContext) {
					channel.DeliveryMsg(msgContext)
					t.waitGroup.Done()
				}(msgContext)
			}
		case conn := <-t.AcceptConnExitChan: // 连接退出
			if conn != nil {
				t.Debug("连接[%v]退出！", conn)
				cn, ok := conn.(StatefulConn)
				if ok {
					t.ConnManager.RemoveConn(cn.GetID())
				}

			}
		case <-t.exitChan:
			goto exit

		}
	}
exit:
	t.Debug("停止收取消息。")
}

// GetChannel 通过[channelID]获取管道信息
func (t *TGO) GetChannel(channelID uint64) (*Channel, error) {
	channel, ok := t.channelMap[channelID]
	var err error
	if !ok {
		channel, err = t.Storage.GetChannel(channelID)
		if err != nil {
			return nil, err
		}
		if channel != nil {
			t.channelMap[channelID] = channel
		}
	}
	return channel, nil
}

// pushOfflineMsg 推送离线消息
func (t *TGO) pushOfflineMsg(clientID uint64,conn Conn)  {
	//channel,err := t.GetChannel(clientID)
	//if err!=nil {
	//	t.Error("查询连接[%v]的Channel失败！-> %v",conn,err)
	//}
}
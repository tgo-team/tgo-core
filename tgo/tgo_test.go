package tgo

import (
	"net"
	"testing"
)


func TestTTGO_pushOfflineMsg(t *testing.T) {
	RegistryStorage(func(context *Context) Storage {
		return NewMemoryStorage(context)
	})
	RegistryServer(func(context *Context) Server {
		return &ServerTest{}
	})
	opts := NewOptions()
	tg := startTGO(opts)

	var clientID uint64= 100

	err := tg.Storage.AddClient(NewClient(clientID, "123456"))
	if err != nil {
		t.Error(err)
	}
	err = tg.Storage.AddChannel(NewChannelModel(clientID,ChannelTypePerson))
	if err != nil {
		t.Error(err)
	}
	err = tg.Storage.Bind(clientID,clientID)
	if err != nil {
		t.Error(err)
	}
	s, _ := net.Pipe()
	for i := 0; i < 999; i++ {
		err = tg.Storage.AddMsgInChannel(NewMsg(uint64(1+i), 99, []byte("hello")),clientID)
		if err != nil {
			t.Error(err)
		}
	}

	tg.pushOfflineMsg(clientID, s)

}

func startTGO(opts *Options) *TGO {
	opts.TCPAddress = "127.0.0.1:0"
	opts.HTTPAddress = "127.0.0.1:0"
	opts.HTTPSAddress = "127.0.0.1:0"
	tg := New(opts)
	err := tg.Start()
	if err != nil {
		panic(err)
	}
	return tg
}

type MemoryStorage struct {
	storageMsgChan chan *MsgContext
	channelMsgMap  map[uint64][]*Msg
	channelMap     map[uint64]*ChannelModel
	clientMap      map[uint64]*Client
	clientChannelRelationMap  map[uint64][]uint64
	ctx            *Context
}

func NewMemoryStorage(ctx *Context) *MemoryStorage {
	return &MemoryStorage{
		storageMsgChan: make(chan *MsgContext, 0),
		channelMsgMap:  make(map[uint64][]*Msg),
		channelMap:     make(map[uint64]*ChannelModel),
		clientMap:      make(map[uint64]*Client),
		clientChannelRelationMap: make(map[uint64][]uint64),
		ctx:            ctx,
	}
}

func (s *MemoryStorage) StorageMsgChan() chan *MsgContext {
	return s.storageMsgChan
}

func (s *MemoryStorage) AddMsgInChannel(msg *Msg, channelID uint64) error {
	msgs := s.channelMsgMap[channelID]
	if msgs == nil {
		msgs = make([]*Msg, 0)
	}
	msgs = append(msgs, msg)
	s.channelMsgMap[channelID] = msgs
	s.storageMsgChan <- NewMsgContext(msg,channelID)
	return nil
}

func (s *MemoryStorage) AddChannel(c *ChannelModel) error {
	s.channelMap[c.ChannelID] = c
	return nil
}
func (s *MemoryStorage) GetChannel(channelID uint64) (*ChannelModel, error) {
	ch := s.channelMap[channelID]
	return ch, nil
}

func (s *MemoryStorage) AddClient(c *Client) error {
	s.clientMap[c.ClientID] = c
	return nil
}

func (s *MemoryStorage) Bind(clientID uint64, channelID uint64) error {
	clientIDs := s.clientChannelRelationMap[channelID]
	if clientIDs==nil {
		clientIDs = make([]uint64,0)
	}
	clientIDs = append(clientIDs,clientID)
	s.clientChannelRelationMap[channelID] = clientIDs
	return nil
}

func (s *MemoryStorage) GetClientIDs(channelID uint64) ([]uint64, error) {
	return s.clientChannelRelationMap[channelID], nil
}

func (s *MemoryStorage) GetClient(clientID uint64) (*Client, error) {

	return s.clientMap[clientID], nil
}

func (s *MemoryStorage) GetMsgInChannel(channelID uint64, pageIndex int64, pageSize int64) ([]*Msg, error) {
	msgList := s.channelMsgMap[channelID]
	if int64(len(msgList)) >= (pageIndex-1)*pageSize+pageSize {
		return msgList[(pageIndex-1)*pageSize : (pageIndex-1)*pageSize+pageSize], nil
	}
	return msgList[(pageIndex-1)*pageSize:], nil
}

func (s *MemoryStorage) RemoveMsgInChannel(messageIDs []uint64, channelID uint64) error   {
	return nil
}
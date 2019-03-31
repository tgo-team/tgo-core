package tgo

import (
	"fmt"
	"time"
)

type Options struct {
	LogLevel             LogLevel
	Log                  Log
	Monitor              Monitor
	LogPrefix            string
	Verbose              bool
	TCPAddress           string
	UDPAddress           string
	HTTPAddress          string
	HTTPSAddress         string
	MaxHeartbeatInterval time.Duration
	DataPath             string
	MaxMsgSize           int32
	MaxBytesPerFile      int64         // 每个文件数据文件最多保存多大的数据 单位byte
	SyncEvery            int64         // 内存队列每满多少消息就同步一次
	SyncTimeout          time.Duration // 超过超时时间没同步就持久化一次
	Pro                  Protocol      // 协议
	MemQueueSize         int64         // 内存队列的chan大小，值表示内存中能堆积多少条消息
	MsgTimeout           time.Duration // 消息发送超时时间
	TestOn               bool          // 是否开启测试模式
}

func NewOptions() *Options {

	return &Options{
		MaxBytesPerFile:      100 * 1024 * 1024,
		MsgTimeout:           60 * time.Second,
		MaxMsgSize:           1024 * 1024,
		Log:                  &DefaultLog{},
		MemQueueSize:         10000,
		SyncEvery:            2500,
		SyncTimeout:          2 * time.Second,
		LogPrefix:            "[tgo-server] ",
		LogLevel:             DebugLevel,
		TCPAddress:           "0.0.0.0:6666",
		UDPAddress:           "0.0.0.0:5555",
		HTTPAddress:          "0.0.0.0:4444",
		HTTPSAddress:         "0.0.0.0:4433",
		MaxHeartbeatInterval: 60 * time.Second,
		TestOn:               false,
		Pro:                  NewProtocol("mqtt-im"),
	}
}

type DefaultLog struct {
}

func (lg *DefaultLog) Info(format string, a ...interface{}) {
	fmt.Println(fmt.Sprintf(fmt.Sprintf("[Info]%s", format), a...))
}
func (lg *DefaultLog) Error(format string, a ...interface{}) {
	fmt.Println(fmt.Sprintf(fmt.Sprintf("[Error]%s", format), a...))
}
func (lg *DefaultLog) Debug(format string, a ...interface{}) {
	fmt.Println(fmt.Sprintf(fmt.Sprintf("[Debug]%s", format), a...))
}
func (lg *DefaultLog) Warn(format string, a ...interface{}) {
	fmt.Println(fmt.Sprintf(fmt.Sprintf("[Warn]%s", format), a...))
}
func (lg *DefaultLog) Fatal(format string, a ...interface{}) {
	fmt.Println(fmt.Sprintf(fmt.Sprintf("[Fatal]%s", format), a...))
}

// Skeleton给Module提供了一个运行骨架.Skeleton实现了ChanRPC(也就是各个模块之间的通信功能).
// 如果一个Module是基于Skeleton实现的,则Skeleton就为这个Module提供了ChanRPC的功能.
package module

import (
	"github.com/name5566/leaf/chanrpc"
	"github.com/name5566/leaf/console"
	"github.com/name5566/leaf/go"
	"github.com/name5566/leaf/timer"
	"time"
)

// 骨架数据结构
type Skeleton struct {
	GoLen              int                  // Go长度
	TimerDispatcherLen int                  // 定时调度器长度
	AsynCallLen        int                  // 异步调用长度
	ChanRPCServer      *chanrpc.Server      // 通道远程调用服务
	g                  *g.Go                // Go调度
	dispatcher         *timer.Dispatcher    // 定时调度器
	client             *chanrpc.Client      // 通道调用客户端
	server             *chanrpc.Server      // 通道调用服务端
	commandServer      *chanrpc.Server      // 通道调用命令行服务端
}

// 骨架初始化
func (s *Skeleton) Init() {
	// 初始Go长度0
	if s.GoLen <= 0 {
		s.GoLen = 0
	}
	// 初始定时调度器长度0
	if s.TimerDispatcherLen <= 0 {
		s.TimerDispatcherLen = 0
	}
	// 初始异步调用长度0
	if s.AsynCallLen <= 0 {
		s.AsynCallLen = 0
	}

	// 初始化Go调度
	s.g = g.New(s.GoLen)
	s.dispatcher = timer.NewDispatcher(s.TimerDispatcherLen) // 调度器为一个时间调度器
	s.client = chanrpc.NewClient(s.AsynCallLen)              // 调度客户端
	s.server = s.ChanRPCServer                               // 调度服务端

	if s.server == nil {
		// 通道调用服务端
		s.server = chanrpc.NewServer(0)
	}
	// 通道调用命令行服务端
	s.commandServer = chanrpc.NewServer(0)
}

// 骨架运行
func (s *Skeleton) Run(closeSig chan bool) {
	// 死循环，从通道接收待处理的数据
	for {
		select {
		// 从结束信号通道中获取到数据
		case <-closeSig:
			s.commandServer.Close()
			s.server.Close()
			for !s.g.Idle() || !s.client.Idle() {
				s.g.Close()
				s.client.Close()
			}
			return
		// 从客户端异步调用结果通道中获取到数据
		case ri := <-s.client.ChanAsynRet:
			s.client.Cb(ri)
		// 从服务端RPC通道中获取到数据
		case ci := <-s.server.ChanCall:
			s.server.Exec(ci)
		// 从命令行服务端RPC通道中获取到数据
		case ci := <-s.commandServer.ChanCall:
			s.commandServer.Exec(ci)
		// 从Go调度器的回调通道中获取到数据
		case cb := <-s.g.ChanCb:
			s.g.Cb(cb)
		// 从定时调度器通道中获取到数据
		case t := <-s.dispatcher.ChanTimer:
			t.Cb()
		}
	}
}

func (s *Skeleton) AfterFunc(d time.Duration, cb func()) *timer.Timer {
	if s.TimerDispatcherLen == 0 {
		panic("invalid TimerDispatcherLen")
	}

	return s.dispatcher.AfterFunc(d, cb)
}

func (s *Skeleton) CronFunc(cronExpr *timer.CronExpr, cb func()) *timer.Cron {
	if s.TimerDispatcherLen == 0 {
		panic("invalid TimerDispatcherLen")
	}

	return s.dispatcher.CronFunc(cronExpr, cb)
}

func (s *Skeleton) Go(f func(), cb func()) {
	if s.GoLen == 0 {
		panic("invalid GoLen")
	}

	s.g.Go(f, cb)
}

func (s *Skeleton) NewLinearContext() *g.LinearContext {
	if s.GoLen == 0 {
		panic("invalid GoLen")
	}

	return s.g.NewLinearContext()
}

func (s *Skeleton) AsynCall(server *chanrpc.Server, id interface{}, args ...interface{}) {
	if s.AsynCallLen == 0 {
		panic("invalid AsynCallLen")
	}

	s.client.Attach(server)
	s.client.AsynCall(id, args...)
}

func (s *Skeleton) RegisterChanRPC(id interface{}, f interface{}) {
	if s.ChanRPCServer == nil {
		panic("invalid ChanRPCServer")
	}

	s.server.Register(id, f)
}

func (s *Skeleton) RegisterCommand(name string, help string, f interface{}) {
	console.Register(name, help, f, s.commandServer)
}

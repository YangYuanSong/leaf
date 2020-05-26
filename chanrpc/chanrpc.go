// 基于 channel 的 RPC 机制，用于游戏服务器模块间通讯。
// 各个模块在协程中运行，并且模块内部完成服务的注册，其他模块可以new一个服务（模块），调用注册的方法
// 各个模块之间的封装后只暴露注册的服务
// chanrpc提供了三种模式
// 同步模式，调用并等待 ChanRPC 返回
// 异步模式，调用并提供回调函数，回调函数会在 ChanRPC 返回后被调用
// Go  模式，调用并立即返回，忽略任何返回值和错误
package chanrpc

import (
	"errors"
	"fmt"
	"github.com/name5566/leaf/conf"
	"github.com/name5566/leaf/log"
	"runtime"
)

// 服务端数据结构
// one server per goroutine (goroutine not safe)
// one client per goroutine (goroutine not safe)
type Server struct {
	// 存放服务端调用函数
	// 服务端通过Register注册被调用的函数
	// 服务端支持三种类型的函数注册（所有的函数都可以通过这三种函数模拟）
	// id -> function
	//
	// function:
	// func(args []interface{})
	// func(args []interface{}) interface{}
	// func(args []interface{}) []interface{}
	functions map[interface{}]interface{}
	// 存放调用函数的通道
	// 服务端从这个通道中循环接收调用函数，然后执行函数
	// 客户端通过Call，把调用函数也写到这个通道中
	ChanCall  chan *CallInfo
}

// 调用传递信息
type CallInfo struct {
	f       interface{}    // 调用函数
	args    []interface{}  // 函数参数
	chanRet chan *RetInfo  // 调用结果通道（服务端执行完成后把数据写入到此通道）
	cb      interface{}    // 服务端执行调用后需要执行的回调
}

// 调用结果信息
type RetInfo struct {
	// 调用结果
	// nil
	// interface{}
	// []interface{}
	ret interface{}
	// 错误信息
	err error
	// 客户端需要执行的回调，数据来源于CallInfo中的cb
	// callback:
	// func(err error)
	// func(ret interface{}, err error)
	// func(ret []interface{}, err error)
	cb interface{}      
}

// 客户端数据结构
type Client struct {
	s               *Server          // 需要调用的服务端
	chanSyncRet     chan *RetInfo    // 同步结果通道（容量为1保证同步，服务端把结果数据写入到通道中，客户端从通道获取数据）
	ChanAsynRet     chan *RetInfo    // 异步结果通道（容量为自定义）
	pendingAsynCall int              // 异步回调计数
}

// 创建远程调用服务端
func NewServer(l int) *Server {
	s := new(Server)
	// 支持调用的函数
	s.functions = make(map[interface{}]interface{})
	s.ChanCall = make(chan *CallInfo, l)
	return s
}

// 断言是否是接口切片类型
func assert(i interface{}) []interface{} {
	if i == nil {
		return nil
	} else {
		// 断言类型转换
		return i.([]interface{})
	}
}

// 向服务端注册调用函数
// you must call the function before calling Open and Go
func (s *Server) Register(id interface{}, f interface{}) {
	// 函数类型判断
	switch f.(type) {
	case func([]interface{}):
	case func([]interface{}) interface{}:
	case func([]interface{}) []interface{}:
	default:
		panic(fmt.Sprintf("function id %v: definition of function is invalid", id))
	}

	// 判断调用函数是否已注册
	if _, ok := s.functions[id]; ok {
		panic(fmt.Sprintf("function id %v: already registered", id))
	}

	// 完成函数注册
	s.functions[id] = f
}

// 返回给客户端调用信息
// ci 调用信息
// ri 结果信息
func (s *Server) ret(ci *CallInfo, ri *RetInfo) (err error) {
	// 如果通道已关闭，则不继续执行
	if ci.chanRet == nil {
		return
	}

	defer func() {
		// 异常捕获
		if r := recover(); r != nil {
			// 设置函数返回的错误信息
			err = r.(error)
		}
	}()

	// 结果信息的回调函数被赋值为调用信息时的回调函数
	// 只要通道没被关闭，客户端都会收到回调，客户端执行回调函数
	ri.cb = ci.cb
	
	// 把结果信息放到代用信息结果通道
	// 客户端通过此通道来获取调用的结果信息
	ci.chanRet <- ri
	return
}

// 执行过程调用
func (s *Server) exec(ci *CallInfo) (err error) {
	defer func() {
		// 捕获异常
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				// 打印堆栈信息和错误信息
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				err = fmt.Errorf("%v: %s", r, buf[:l])
			} else {
				// 打印错误信息
				err = fmt.Errorf("%v", r)
			}

			// 给客户端的返回异常信息
			s.ret(ci, &RetInfo{err: fmt.Errorf("%v", r)})
		}
	}()

	// 根据过程调用函数格式，采用不同的方式调用函数
	// 函数类型断言判断
	// execute
	switch ci.f.(type) {
	case func([]interface{}):
		// 接口类型的f通过类型断言转换成对应的函数，并使用参数执行函数
		ci.f.(func([]interface{}))(ci.args)
		// 返回调用结果（是否有错误）
		return s.ret(ci, &RetInfo{})
	case func([]interface{}) interface{}:
		ret := ci.f.(func([]interface{}) interface{})(ci.args)
		return s.ret(ci, &RetInfo{ret: ret})
	case func([]interface{}) []interface{}:
		ret := ci.f.(func([]interface{}) []interface{})(ci.args)
		return s.ret(ci, &RetInfo{ret: ret})
	}

	// 触发异常
	panic("bug")
}

// 服务端执行一个调用
// 一般通过循环接收ChanCall通道，获得执行调用的函数信息
func (s *Server) Exec(ci *CallInfo) {
	err := s.exec(ci)
	// 执行错误打印错误信息
	if err != nil {
		log.Error("%v", err)
	}
}

// Go方式调用
// goroutine safe
func (s *Server) Go(id interface{}, args ...interface{}) {
	// 获取调用方法
	f := s.functions[id]
	if f == nil {
		return
	}

	defer func() {
		// 捕获异常
		recover()
	}()

	// 把调用信息写入通道
	s.ChanCall <- &CallInfo{
		f:    f,
		args: args,
	}
}

// 服务端自己调用 - 第一种类型函数
// goroutine safe
func (s *Server) Call0(id interface{}, args ...interface{}) error {
	// 阻塞通道的形式通信
	// 调用会被阻塞，直到服务端调用开始运行
	return s.Open(0).Call0(id, args...)
}

// 服务端自己调用 - 第二种类型函数
// goroutine safe
func (s *Server) Call1(id interface{}, args ...interface{}) (interface{}, error) {
	return s.Open(0).Call1(id, args...)
}

// 服务端自己调用 - 第三种类型函数
// goroutine safe
func (s *Server) CallN(id interface{}, args ...interface{}) ([]interface{}, error) {
	return s.Open(0).CallN(id, args...)
}

// 服务端关闭
func (s *Server) Close() {
	// 关闭通道（客户端调用失败）
	close(s.ChanCall)

	// 循环遍历通道中的待调用数据
	for ci := range s.ChanCall {
		// 直接返回调用服务已关闭的错误信息
		s.ret(ci, &RetInfo{
			err: errors.New("chanrpc server closed"),
		})
	}
}

// 服务端暴露的Open方法创建对应的客户端
// 并且客户端关联此服务端
// 客户端可以通过Call*()执行同步调用、AsynCall()执行异步调用、Cb()执行回调
// goroutine safe
func (s *Server) Open(l int) *Client {
	c := NewClient(l)
	c.Attach(s)
	return c
}

// 创建过程调用客户端
// 只创建客户端，并不关联具体的服务端
func NewClient(l int) *Client {
	c := new(Client)
	// 同步调用采用缓冲通道实现（通道容量为1）
	c.chanSyncRet = make(chan *RetInfo, 1)
	// 异步调用采用缓冲通道实现（通道容量为自定义）
	// 如果长度为0则为非缓冲通道，服务端执行完成和客户端开始执行同步
	c.ChanAsynRet = make(chan *RetInfo, l)
	return c
}

// 设置服务端（客户端一个时刻只能有一个服务端）
func (c *Client) Attach(s *Server) {
	c.s = s
}

// 发起远程过程调用
// CallInfo - 调用信息
// block - 调用是否阻塞
func (c *Client) call(ci *CallInfo, block bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	if block {
		// 阻塞的话把调用信息加入通道
		c.s.ChanCall <- ci
	} else {
		// 异步调用
		select {
		case c.s.ChanCall <- ci:
		default:
			err = errors.New("chanrpc channel full")
		}
	}
	return
}

// 客户端获取调用函数（函数用于执行）
// id是服务端注册的函数
// n是调用函数的类型
func (c *Client) f(id interface{}, n int) (f interface{}, err error) {
	if c.s == nil {
		err = errors.New("server not attached")
		return
	}

	// 获取服务端函数
	f = c.s.functions[id]
	if f == nil {
		err = fmt.Errorf("function id %v: function not registered", id)
		return
	}

	var ok bool
	switch n {
		// 根据不同形式，使用断言把接口转换成函数返回
		// 不同形式的函数，参数和返回值的内存分配会不同
	case 0:
		_, ok = f.(func([]interface{}))
	case 1:
		_, ok = f.(func([]interface{}) interface{})
	case 2:
		_, ok = f.(func([]interface{}) []interface{})
	default:
		panic("bug")
	}

	if !ok {
		err = fmt.Errorf("function id %v: return type mismatch", id)
	}
	return
}

// 同步调用 - 第一种类型函数
// 接受切片类型的参数，无返回值
func (c *Client) Call0(id interface{}, args ...interface{}) error {
	// 获取调用函数
	f, err := c.f(id, 0)
	if err != nil {
		return err
	}
	// 组装调用信息，执行过程调用
	err = c.call(&CallInfo{
		f:       f,              // 函数名称
		args:    args,           // 函数参数
		chanRet: c.chanSyncRet,  // 客户端同步结果通道
	}, true)
	// 判断客户端调用时是否发生错误
	if err != nil {
		return err
	}
	// 阻塞等待调用结果
	// 从同步调用结果通道获取调用结果信息
	ri := <-c.chanSyncRet
	// 返回调用错误信息
	return ri.err
}

// 同步调用 - 第二种类型函数
// 接受切片类型的参数，返回非切片数据
func (c *Client) Call1(id interface{}, args ...interface{}) (interface{}, error) {
	f, err := c.f(id, 1)
	if err != nil {
		return nil, err
	}

	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet,
	}, true)
	if err != nil {
		return nil, err
	}

	ri := <-c.chanSyncRet
	// 返回调用结果和错误信息
	return ri.ret, ri.err
}

// 同步调用 - 第三种类型函数
// 接受切片类型的参数，返回切片数据
func (c *Client) CallN(id interface{}, args ...interface{}) ([]interface{}, error) {
	f, err := c.f(id, 2)
	if err != nil {
		return nil, err
	}

	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet,
	}, true)
	if err != nil {
		return nil, err
	}

	ri := <-c.chanSyncRet
	// 返回调用结果信息（通过断言转换为切片类型）和错误信息
	return assert(ri.ret), ri.err
}

// 执行异步调用
func (c *Client) asynCall(id interface{}, args []interface{}, cb interface{}, n int) {
	// 获取异步调用的函数
	f, err := c.f(id, n)
	if err != nil {
		// 获取到执行的函数后，异步调用结果信息写入异步结果通道
		// 
		c.ChanAsynRet <- &RetInfo{err: err, cb: cb}
		return
	}

	// 准备调用信息，发起异步调用
	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.ChanAsynRet,
		cb:      cb,
	}, false)
	if err != nil {
		// 执行异步调用后，异步调用结果信息写入异步结果通道
		// 
		c.ChanAsynRet <- &RetInfo{err: err, cb: cb}
		return
	}
}

// 异步调用
// id    注册函数的ID
// _args 参数集合
func (c *Client) AsynCall(id interface{}, _args ...interface{}) {
	// 参数判断，必须要有参数
	if len(_args) < 1 {
		panic("callback function not found")
	}

	// 调用参数（可以没有）
	args := _args[:len(_args)-1]
	// 最后一个参数必须是回调函数
	cb := _args[len(_args)-1]

	// 回调函数类型
	var n int
	// 类型断言，确定回调函数的类型
	switch cb.(type) {
	case func(error):
		n = 0
	case func(interface{}, error):
		n = 1
	case func([]interface{}, error):
		n = 2
	default:
		panic("definition of callback function is invalid")
	}

	// 待执行的回调太多，不进行异步调用，直接执行回调函数（并报错误，调用失败）
	// too many calls
	if c.pendingAsynCall >= cap(c.ChanAsynRet) {
		execCb(&RetInfo{err: errors.New("too many calls"), cb: cb})
		return
	}

	// 异步调用
	c.asynCall(id, args, cb, n)
	// 异步调用计数自增
	c.pendingAsynCall++
}

// 执行回调
// 参数为调用的结果信息
func execCb(ri *RetInfo) {
	defer func() {
		// 异常判断
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				// 打印堆栈信息和错误信息
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Error("%v: %s", r, buf[:l])
			} else {
				// 打印错误信息
				log.Error("%v", r)
			}
		}
	}()

	// 执行回调
	// execute
	switch ri.cb.(type) {
		// 回调类型通过类型断言判断
	case func(error):
		// 通过类型转换为对应的函数类型，并执行函数
		// 函数的参数为调用结果的错误信息
		ri.cb.(func(error))(ri.err)
	case func(interface{}, error):
		// 函数的参数为调用结果的结果信息和错误信息
		ri.cb.(func(interface{}, error))(ri.ret, ri.err)
	case func([]interface{}, error):
		// 函数的参数为调用结果的结果信息（断言转换为切片类型）和错误信息
		ri.cb.(func([]interface{}, error))(assert(ri.ret), ri.err)
	default:
		panic("bug")
	}
	return
}

// 客户端执行回调（异步调用时需要）
// 如果需要进行回调，则在调用的时候把回调函数注册到CallInfo中
func (c *Client) Cb(ri *RetInfo) {
	// 异步回调计数自减
	c.pendingAsynCall--
	// 执行回调
	execCb(ri)
}

// 关闭客户端
func (c *Client) Close() {
	// 循环判断异步回调是否执行完毕
	for c.pendingAsynCall > 0 {
		// 从通道获取调用结果，执行回调
		c.Cb(<-c.ChanAsynRet)
	}
}

// 判断客户端是否空闲
func (c *Client) Idle() bool {
	// 异步回调计数是否为0
	return c.pendingAsynCall == 0
}

// Go模块是对golang中go提供一些额外功能。
// Go提供回调功能
// LinearContext提供顺序调用功能。
package g

import (
	"container/list"
	"github.com/name5566/leaf/conf"
	"github.com/name5566/leaf/log"
	"runtime"
	"sync"
)

// Go 封装
// one Go per goroutine (goroutine not safe)
type Go struct {
	ChanCb    chan func()   // 通道中存放回调函数
	pendingGo int           // 回调函数数量计数器
}

// 线性Go
type LinearGo struct {
	f  func()   // 函数
	cb func()   // 回调函数
}


type LinearContext struct {
	g              *Go         // Go
	linearGo       *list.List  // LinearGo，列表数据结果，模拟的是队列
	mutexLinearGo  sync.Mutex  // 多资源 互斥锁（队列操作并发安全）
	mutexExecution sync.Mutex  // 多执行 互斥锁（函数执行并发安全，线性顺序依次执行）
}

// 新建Go，通过l控制回调函数通道的长度
func New(l int) *Go {
	// 新增结构
	g := new(Go)
	// 初始化回调函数通道
	g.ChanCb = make(chan func(), l)
	return g
}

// 在Go中调用函数，并把回调函数添加到通道中
// 调用函数和回调函数 都在新的协程中执行
func (g *Go) Go(f func(), cb func()) {
	// 计数器自增
	g.pendingGo++

	go func() {
		defer func() {
			// 函数执行完后，把回调函数添加到通道中
			g.ChanCb <- cb
			// 出现异常
			if r := recover(); r != nil {
				if conf.LenStackBuf > 0 {
					// 打印堆栈和错误信息
					buf := make([]byte, conf.LenStackBuf)
					l := runtime.Stack(buf, false)
					log.Error("%v: %s", r, buf[:l])
				} else {
					// 打印错误信息
					log.Error("%v", r)
				}
			}
		}()

		// 执行函数
		f()
	}()
}

// 在Go中执行回调函数
func (g *Go) Cb(cb func()) {
	defer func() {
		// 计数器自减
		g.pendingGo--
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Error("%v: %s", r, buf[:l])
			} else {
				log.Error("%v", r)
			}
		}
	}()

	// 执行回调函数
	if cb != nil {
		cb()
	}
}

// Go关闭
func (g *Go) Close() {
	// 依次完成回调函数的调用
	for g.pendingGo > 0 {
		// 执行回调
		g.Cb(<-g.ChanCb)
	}
}

// 判断是否空闲
func (g *Go) Idle() bool {
	return g.pendingGo == 0
}

// 通过Go来创建新的Linear上下文
func (g *Go) NewLinearContext() *LinearContext {
	c := new(LinearContext)
	c.g = g                  // Go
	c.linearGo = list.New()  // 标准库提供的List列表（用作队列，支持动态扩容）
	return c
}

// Linear上下文  运行 Go
func (c *LinearContext) Go(f func(), cb func()) {
	// 回调计数器自增
	c.g.pendingGo++

	// 加锁
	c.mutexLinearGo.Lock()
	// 往队列中Push执行函数和回调函数（入队）
	c.linearGo.PushBack(&LinearGo{f: f, cb: cb})
	// 解锁
	c.mutexLinearGo.Unlock()

	// 新开一个协程运行程序
	go func() {
		// 加锁 - 保证只有一个协程运行
		c.mutexExecution.Lock()
		// 解锁 - 协程执行结束
		defer c.mutexExecution.Unlock()

		// 加锁
		c.mutexLinearGo.Lock()
		// 从队列中取数据（出队）
		e := c.linearGo.Remove(c.linearGo.Front()).(*LinearGo)
		// 解锁
		c.mutexLinearGo.Unlock()

		defer func() {
			// 函数执行完成后，将回调函数写入回调函数通道
			c.g.ChanCb <- e.cb
			// 捕获异常
			if r := recover(); r != nil {
				if conf.LenStackBuf > 0 {
					// 打印堆栈和错误信息
					buf := make([]byte, conf.LenStackBuf)
					l := runtime.Stack(buf, false)
					log.Error("%v: %s", r, buf[:l])
				} else {
					// 打印错误信息
					log.Error("%v", r)
				}
			}
		}()

		// 执行函数
		e.f()
	}()
}

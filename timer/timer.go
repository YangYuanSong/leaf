// 定时器

package timer

import (
	"github.com/name5566/leaf/conf"
	"github.com/name5566/leaf/log"
	"runtime"
	"time"
)

// 调度器（非协程安全）
// 通过Timer通道方式模拟
// one dispatcher per goroutine (goroutine not safe)
type Dispatcher struct {
	ChanTimer chan *Timer
}

// 创建新的调度器
// l长度的通道
func NewDispatcher(l int) *Dispatcher {
	disp := new(Dispatcher)
	disp.ChanTimer = make(chan *Timer, l)
	return disp
}

// 计时器
// Timer
type Timer struct {
	t  *time.Timer  // 标准计时器
	cb func()       // 回调方法
}

// 计时器停止
func (t *Timer) Stop() {
	t.t.Stop()      // 计时器停止
	t.cb = nil      // 回调方法置空
}

// 计时器回调方法
func (t *Timer) Cb() {
	defer func() {
		t.cb = nil
		// 回调异常处理
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				// 异常信息压入堆栈
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				// 打印异常和堆栈信息
				log.Error("%v: %s", r, buf[:l])
			} else {
				// 打印异常信息
				log.Error("%v", r)
			}
		}
	}()
	// 回调方法
	if t.cb != nil {
		t.cb()
	}
}

// 调度器添加 时间间隔后回调方法
func (disp *Dispatcher) AfterFunc(d time.Duration, cb func()) *Timer {
	t := new(Timer)
	t.cb = cb
	t.t = time.AfterFunc(d, func() {
		// 定时器加入到通道队列中
		disp.ChanTimer <- t
	})
	return t
}

// 定时任务
// Cron
type Cron struct {
	// 计时器
	t *Timer
}

// 定时任务 - 停止
func (c *Cron) Stop() {
	if c.t != nil {
		c.t.Stop()
	}
}

// 定时任务 - 使用调度器生成定时任务
func (disp *Dispatcher) CronFunc(cronExpr *CronExpr, _cb func()) *Cron {
	c := new(Cron)

	now := time.Now()
	nextTime := cronExpr.Next(now)
	if nextTime.IsZero() {
		return c
	}

	// callback
	var cb func()
	cb = func() {
		defer _cb()

		now := time.Now()
		nextTime := cronExpr.Next(now)
		if nextTime.IsZero() {
			return
		}
		c.t = disp.AfterFunc(nextTime.Sub(now), cb)
	}

	c.t = disp.AfterFunc(nextTime.Sub(now), cb)
	return c
}

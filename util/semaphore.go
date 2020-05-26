// 工具包 - 信号量
package util

// 使用空结构体通道模拟信号量
type Semaphore chan struct{}

// 创建信号量（n长度的通道）
func MakeSemaphore(n int) Semaphore {
	return make(Semaphore, n)
}

// 获取信号量
func (s Semaphore) Acquire() {
	s <- struct{}{}
}

// 释放信号量
func (s Semaphore) Release() {
	<-s
}

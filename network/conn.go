package network

import (
	"net"
)

// 定义连接接口
type Conn interface {
	// 读信息
	ReadMsg() ([]byte, error)
	// 写信息
	WriteMsg(args ...[]byte) error
	// 获取本地地址
	LocalAddr() net.Addr
	// 获取远程地址
	RemoteAddr() net.Addr
	// 关闭连接
	Close()
	// 销毁连接
	Destroy()
}

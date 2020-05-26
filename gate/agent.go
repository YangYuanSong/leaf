// 传输门
package gate

import (
	"net"
)

// 定义代理的数据接口方法
type Agent interface {
	WriteMsg(msg interface{})        // 写入信息
	LocalAddr() net.Addr             // 获取本地的IP地址
	RemoteAddr() net.Addr            // 获取远程的IP地址
	Close()                          // 代理关闭
	Destroy()                        // 销毁
	UserData() interface{}           // 获取用户数据
	SetUserData(data interface{})    // 设置用户数据
}

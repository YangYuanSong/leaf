// WEB socket
package network

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/name5566/leaf/log"
	"net"
	"sync"
)

// 定义WebSocket连接池
type WebsocketConnSet map[*websocket.Conn]struct{}

// 定义WebSocket数据结构
type WSConn struct {
	sync.Mutex                // 同步锁
	conn      *websocket.Conn // Web Socket连接
	writeChan chan []byte     // 数据写通道
	maxMsgLen uint32          // 最大信息长度
	closeFlag bool            // 关闭标识
}

// 新建WebSocket链接
func newWSConn(conn *websocket.Conn, pendingWriteNum int, maxMsgLen uint32) *WSConn {
	// New一个WebSocket数据结构（分配内存）
	wsConn := new(WSConn)
	// 链接赋值
	wsConn.conn = conn
	// 创建写的数据通道
	wsConn.writeChan = make(chan []byte, pendingWriteNum)
	// 赋值信息最大长度
	wsConn.maxMsgLen = maxMsgLen

	// 每创建一个连接都会新创建一个协程用于写数据
	go func() {
		// 遍历写的通道
		for b := range wsConn.writeChan {
			// 主动关闭写通道
			if b == nil {
				break
			}

			// 二进制方式把通道数据写入链接中
			err := conn.WriteMessage(websocket.BinaryMessage, b)
			// 写数据报错
			if err != nil {
				break
			}
		}

		// 链接关闭
		conn.Close()
		// 链接锁住
		wsConn.Lock()
		// 设置连接关闭标识
		wsConn.closeFlag = true
		// 解锁
		wsConn.Unlock()
	}()

	// 返回创建的连接
	return wsConn
}

// 执行销毁WebSocket
func (wsConn *WSConn) doDestroy() {
	// 连接丢弃未发送的数据
	wsConn.conn.UnderlyingConn().(*net.TCPConn).SetLinger(0)
	// 连接关闭
	wsConn.conn.Close()

	// 判断连接标识是否已关闭
	if !wsConn.closeFlag {
		// 关闭通道
		close(wsConn.writeChan)
		// 设置关闭标识符
		wsConn.closeFlag = true
	}
}

// 销毁WebSocket连接
func (wsConn *WSConn) Destroy() {
	// 加锁
	wsConn.Lock()
	defer wsConn.Unlock()

	// 执行销毁WebSocket
	wsConn.doDestroy()
}

// 关闭WebSocket连接
func (wsConn *WSConn) Close() {
	// 加锁
	wsConn.Lock()
	defer wsConn.Unlock()
	// 判读是否已关闭
	if wsConn.closeFlag {
		return
	}

	// 向通道写入nil，主动关闭
	wsConn.doWrite(nil)
	// 设置关闭标识
	wsConn.closeFlag = true
}

// WebSocket执行写数据
func (wsConn *WSConn) doWrite(b []byte) {
	// 判断通道是否已写满
	if len(wsConn.writeChan) == cap(wsConn.writeChan) {
		// 输出日志、销毁连接
		log.Debug("close conn: channel full")
		wsConn.doDestroy()
		return
	}

	// 向通道中写入数据
	wsConn.writeChan <- b
}

// 获取本地地址
func (wsConn *WSConn) LocalAddr() net.Addr {
	return wsConn.conn.LocalAddr()
}

// 获取远程地址
func (wsConn *WSConn) RemoteAddr() net.Addr {
	return wsConn.conn.RemoteAddr()
}

// 读数据
// goroutine not safe
func (wsConn *WSConn) ReadMsg() ([]byte, error) {
	// 读数据
	_, b, err := wsConn.conn.ReadMessage()
	return b, err
}

// 写数据
// args must not be modified by the others goroutines
func (wsConn *WSConn) WriteMsg(args ...[]byte) error {
	// 加锁
	wsConn.Lock()
	defer wsConn.Unlock()
	// 判断链接关闭标识
	if wsConn.closeFlag {
		return nil
	}

	// 获取信息长度
	// get len
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		// 循环叠加切片数据长度
		msgLen += uint32(len(args[i]))
	}

	// 信息长度判断
	// check len
	if msgLen > wsConn.maxMsgLen {
		return errors.New("message too long")
	} else if msgLen < 1 {
		return errors.New("message too short")
	}

	// 直接写入一个字节切片
	// don't copy
	if len(args) == 1 {
		// 执行写入
		wsConn.doWrite(args[0])
		return nil
	}

	// 根据总长度创建字节切片
	// merge the args
	msg := make([]byte, msgLen)
	l := 0
	for i := 0; i < len(args); i++ {
		// 循环拷贝每个字节切片到待发送的切片中
		copy(msg[l:], args[i])
		// 长度增加
		l += len(args[i])
	}

	// 所有数据一次性写入
	wsConn.doWrite(msg)

	return nil
}

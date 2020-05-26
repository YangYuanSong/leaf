// 封装了TCP连接，实现了conn接口
// 写入的时候通过通道异步写入，提高效率
package network

import (
	"github.com/name5566/leaf/log"
	"net"
	"sync"
)

// 定义连接池
type ConnSet map[net.Conn]struct{}

// TCP链接结构，实现了conn接口
type TCPConn struct {
	sync.Mutex                // 互斥锁
	conn      net.Conn        // 网络连接
	writeChan chan []byte     // 写通道
	closeFlag bool            // 关闭标识
	msgParser *MsgParser      // 信息解析器（json、protobuf）
}

// 创建一个TCPConn链接
// conn            TCP的网络连接
// pendingWriteNum 通道容量大小
// msgParser       信息解析器
func newTCPConn(conn net.Conn, pendingWriteNum int, msgParser *MsgParser) *TCPConn {
	// 创建链接
	tcpConn := new(TCPConn)
	tcpConn.conn = conn
	tcpConn.writeChan = make(chan []byte, pendingWriteNum)
	tcpConn.msgParser = msgParser

	// 链接的写是异步的，通过通道在一个新的协程中进行
	go func() {
		// 从写通道遍历待写数据
		for b := range tcpConn.writeChan {
			if b == nil {
				// 通道关闭（主动关闭）
				break
			}

			// 往连接中写入数据
			_, err := conn.Write(b)
			if err != nil {
				// 连接写入异常（异常关闭）
				break
			}
		}
		
		// 关闭链接
		conn.Close()
		// 锁住TCP链接
		tcpConn.Lock()
		// 标记链接关闭标识
		tcpConn.closeFlag = true
		// 解锁TCP链接
		tcpConn.Unlock()
	}()

	// 返回链接
	return tcpConn
}

// 执行TCP链接销毁
func (tcpConn *TCPConn) doDestroy() {
	// 连接丢弃未发送的数据
	tcpConn.conn.(*net.TCPConn).SetLinger(0)
	// 连接关闭
	tcpConn.conn.Close()

	if !tcpConn.closeFlag {
		// 关闭TCP链接写通道
		close(tcpConn.writeChan)
		// 设置TCP链接关闭标识
		tcpConn.closeFlag = true
	}
}

// 销毁TCP链接
func (tcpConn *TCPConn) Destroy() {
	// 锁住TCP链接
	tcpConn.Lock()
	defer tcpConn.Unlock()

	// 执行销毁
	tcpConn.doDestroy()
}

// 关闭TCP链接
func (tcpConn *TCPConn) Close() {
	// 锁住TCP链接
	tcpConn.Lock()
	defer tcpConn.Unlock()
	// 判断TCP链接是否已关闭
	if tcpConn.closeFlag {
		return
	}

	// 连接执行写入nil（主动关闭）
	tcpConn.doWrite(nil)
	// 设置TCP链接关闭标识
	tcpConn.closeFlag = true
}

// TCP链接执行写入
func (tcpConn *TCPConn) doWrite(b []byte) {
	// 判断写入通道是否已写满
	// 带宽太小可能会触发这种情况（写的太快，来不及发送，通道队列被占满）
	// 必须要控制链接数（房间人数等）和测试最大写入带宽要求
	if len(tcpConn.writeChan) == cap(tcpConn.writeChan) {
		// 打印日志
		log.Debug("close conn: channel full")
		// 销毁TCP链接
		tcpConn.doDestroy()
		return
	}

	// 链接通道写入数据
	tcpConn.writeChan <- b
}

// TCP链接写入数据
// b must not be modified by the others goroutines
func (tcpConn *TCPConn) Write(b []byte) {
	// 锁住TCP链接
	tcpConn.Lock()
	defer tcpConn.Unlock()
	// 判断TCP链接是否关闭和不能写入nil,nil代表主动关闭链接
	if tcpConn.closeFlag || b == nil {
		return
	}

	// 连接执行写入
	tcpConn.doWrite(b)
}

// TCP链接读取数据（原生的字节流）
func (tcpConn *TCPConn) Read(b []byte) (int, error) {
	return tcpConn.conn.Read(b)
}

// TCP链接获取本地地址
func (tcpConn *TCPConn) LocalAddr() net.Addr {
	return tcpConn.conn.LocalAddr()
}

// TCP链接获取远程地址
func (tcpConn *TCPConn) RemoteAddr() net.Addr {
	return tcpConn.conn.RemoteAddr()
}

// TCP链接读取数据（用解析器解析后的数据）
func (tcpConn *TCPConn) ReadMsg() ([]byte, error) {
	return tcpConn.msgParser.Read(tcpConn)
}

// TCP链接写入数据（用解析器编码后的数据）
func (tcpConn *TCPConn) WriteMsg(args ...[]byte) error {
	return tcpConn.msgParser.Write(tcpConn, args...)
}

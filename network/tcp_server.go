// 封装的TCP链接服务器
package network

import (
	"github.com/name5566/leaf/log"
	"net"
	"sync"
	"time"
)

// TCP服务器数据结构
type TCPServer struct {
	Addr            string                // 服务器绑定的地址
	MaxConnNum      int                   // 最大链接数
	PendingWriteNum int                   // 
	NewAgent        func(*TCPConn) Agent  // 代理（根据一个TCP链接返回一个代理接口的函数）
	ln              net.Listener          // TCP网络监听器
	conns           ConnSet               // 连接池
	mutexConns      sync.Mutex            // 多链接，互斥锁
	wgLn            sync.WaitGroup        // 组等待（Accept连接协程）
	wgConns         sync.WaitGroup        // 组等待（应用连接所有协程）

	// msg parser
	LenMsgLen    int         // 业务消息长度字节数
	MinMsgLen    uint32      // 消息最小长度
	MaxMsgLen    uint32      // 消息最大长度
	LittleEndian bool        // 字节序（用于获取消息长度）
	msgParser    *MsgParser  // 信息解析器
}

// 开始TCP服务器
func (server *TCPServer) Start() {
	// 服务器初始化
	server.init()
	// 在一个新协程中运行服务器
	go server.run()
}

// 初始化TCP服务器
func (server *TCPServer) init() {
	// 建立监听
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatal("%v", err)
	}

	// 设置最大链接数
	if server.MaxConnNum <= 0 {
		server.MaxConnNum = 100
		// 输出设置为默认值信息
		log.Release("invalid MaxConnNum, reset to %v", server.MaxConnNum)
	}
	// 设置挂起写数量
	if server.PendingWriteNum <= 0 {
		server.PendingWriteNum = 100
		log.Release("invalid PendingWriteNum, reset to %v", server.PendingWriteNum)
	}
	// 检查代理
	if server.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}

	// 赋值监听器
	server.ln = ln
	// 初始化连接池
	server.conns = make(ConnSet)

	// 信息解析器处理
	// msg parser
	// 创建新的解析器
	msgParser := NewMsgParser() 
	// 设置解析器参数
	msgParser.SetMsgLen(server.LenMsgLen, server.MinMsgLen, server.MaxMsgLen)
	// 设置解析器的字节序（默认false,采用大端序）
	msgParser.SetByteOrder(server.LittleEndian)
	// 服务器解析器赋值
	server.msgParser = msgParser
}

// TCP服务器运行
func (server *TCPServer) run() {
	// 开启组同步
	server.wgLn.Add(1)
	// 函数返回结束组同步
	defer server.wgLn.Done()

	var tempDelay time.Duration
	for {
		// 循环接受连接
		conn, err := server.ln.Accept()
		if err != nil {
			// 接受链接发生网络错误
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					// 初始默认临时延迟5毫秒
					tempDelay = 5 * time.Millisecond
				} else {
					// 临时延迟翻倍
					tempDelay *= 2
				}
				// 计算最大临时延迟为1秒
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				// 输出accept错误
				log.Release("accept error: %v; retrying in %v", err, tempDelay)
				// 程序sleep,继续Accept网络链接
				time.Sleep(tempDelay)
				continue
			}
			// 发生非网络错误，则结束TCP服务器
			return
		}
		// 重置临时延时时间
		tempDelay = 0

		// 多链接加锁
		server.mutexConns.Lock()
		// 链接数达到临界值，则关闭当前连接
		if len(server.conns) >= server.MaxConnNum {
			// 解锁
			server.mutexConns.Unlock()
			// 关闭链接
			conn.Close()
			// 输出链接过多的日志
			log.Debug("too many connections")
			// 继续等待Accept新连接
			continue
		}
		// 把当前连接加入到链接集合
		server.conns[conn] = struct{}{}
		// 解锁
		server.mutexConns.Unlock()

		// 应用连接组同步+1
		server.wgConns.Add(1)

		// 接收到的连接创建新的TCP链接
		tcpConn := newTCPConn(conn, server.PendingWriteNum, server.msgParser)
		// 使用新建立的TCP链接创建一个新代理
		agent := server.NewAgent(tcpConn)
		// 在一个新协程中运行代理（处理具体的事物）
		go func() {
			// 代理运行
			agent.Run()

			// 关闭TCP链接
			// cleanup
			tcpConn.Close()
			// 锁住链接
			server.mutexConns.Lock()
			// 删除链接
			delete(server.conns, conn)
			// 解锁
			server.mutexConns.Unlock()
			// 执行代理OnClose方法
			agent.OnClose()
			
			// 应用连接组同步-1
			server.wgConns.Done()
		}()
	}
}

// TCP服务器关闭
func (server *TCPServer) Close() {
	// 关闭网络监听器
	server.ln.Close()
	// 等待Accept连接处理协程结束（TCP服务器run方法结束）
	server.wgLn.Wait()
	// 锁住链接
	server.mutexConns.Lock()
	// 遍历所有服务器连接，依次关闭链接
	for conn := range server.conns {
		conn.Close()
	}
	server.conns = nil
	// 解锁
	server.mutexConns.Unlock()
	// 等待应用来接组
	server.wgConns.Wait()
}

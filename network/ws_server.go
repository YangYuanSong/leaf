// WebSocket
package network

import (
	"crypto/tls"
	"github.com/gorilla/websocket"    
	"github.com/name5566/leaf/log"
	"net"
	"net/http"
	"sync"
	"time"
)

// WebSocket服务器数据结构
type WSServer struct {
	Addr            string                 // 服务器地址
	MaxConnNum      int                    // 最大连接数
	PendingWriteNum int                    // 写挂起数量
	MaxMsgLen       uint32                 // 信息最大长度
	HTTPTimeout     time.Duration          // 超时
	CertFile        string                 // 证书文件
	KeyFile         string                 // 秘钥文件
	NewAgent        func(*WSConn) Agent    // 新代理（输入WebSocket链接返回一个代理的函数）
	ln              net.Listener           // 监听器
	handler         *WSHandler             // WebSocket处理句柄
}

// WebSocket处理句柄数据结构
type WSHandler struct {
	maxConnNum      int                    // 最大链接数
	pendingWriteNum int                    // 写挂起数量
	maxMsgLen       uint32                 // 信息最大长度
	newAgent        func(*WSConn) Agent    // 新代理（输入WebSocket链接返回一个代理的函数）
	upgrader        websocket.Upgrader     // 
	conns           WebsocketConnSet       // WebSocket链接池
	mutexConns      sync.Mutex             // 同步互斥锁
	wg              sync.WaitGroup         // 组同步
}

// 实现HTTP的ServeHTTP服务接口，获取请求，输出响应
func (handler *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 请求方法判断（只支持GET）
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	// 获取到底层的连接
	conn, err := handler.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("upgrade error: %v", err)
		return
	}
	// 设置底层链接读取字节限制
	conn.SetReadLimit(int64(handler.maxMsgLen))

	// 组同步+1
	handler.wg.Add(1)
	defer handler.wg.Done()

	// 锁住句柄
	handler.mutexConns.Lock()
	// 句柄链接判断
	if handler.conns == nil {
		// 句柄的连接为nil,关闭
		handler.mutexConns.Unlock()
		conn.Close()
		return
	}
	// 链接已触发最大限制临界值
	if len(handler.conns) >= handler.maxConnNum {
		// 关闭链接，并输出日志信息
		handler.mutexConns.Unlock()
		conn.Close()
		log.Debug("too many connections")
		return
	}
	// 连接加入到连接池中
	handler.conns[conn] = struct{}{}
	// 解锁
	handler.mutexConns.Unlock()

	// 根据链接创建新的WebSocket连接
	wsConn := newWSConn(conn, handler.pendingWriteNum, handler.maxMsgLen)
	// 根据WebSocket连接创建一个新的代理
	agent := handler.newAgent(wsConn)
	// 运行代理
	agent.Run()

	// 清理，关闭WebSocket链接
	// cleanup
	wsConn.Close()
	handler.mutexConns.Lock()
	// 删除连接池中的链接
	delete(handler.conns, conn)
	handler.mutexConns.Unlock()
	// 代理执行OnClose方法
	agent.OnClose()
}

// 启动WebSocket服务器
func (server *WSServer) Start() {
	// 创建TCP监听器
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatal("%v", err)
	}

	// 初始最大连接数
	if server.MaxConnNum <= 0 {
		server.MaxConnNum = 100
		log.Release("invalid MaxConnNum, reset to %v", server.MaxConnNum)
	}
	// 初始挂起写连接数
	if server.PendingWriteNum <= 0 {
		server.PendingWriteNum = 100
		log.Release("invalid PendingWriteNum, reset to %v", server.PendingWriteNum)
	}
	// 初始信息最大长度（默认4k）
	if server.MaxMsgLen <= 0 {
		server.MaxMsgLen = 4096
		log.Release("invalid MaxMsgLen, reset to %v", server.MaxMsgLen)
	}
	// 初始HTTP超时时间
	if server.HTTPTimeout <= 0 {
		// 默认超时时间设置为10秒
		server.HTTPTimeout = 10 * time.Second
		log.Release("invalid HTTPTimeout, reset to %v", server.HTTPTimeout)
	}
	// 服务器代理判断
	if server.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}

	// HTTPS通信设置
	if server.CertFile != "" || server.KeyFile != "" {
		// 创建TLS配置
		config := &tls.Config{}
		config.NextProtos = []string{"http/1.1"}

		// TLS配置证书信息
		var err error
		config.Certificates = make([]tls.Certificate, 1)
		config.Certificates[0], err = tls.LoadX509KeyPair(server.CertFile, server.KeyFile)
		if err != nil {
			log.Fatal("%v", err)
		}

		// 根据现有的监听器，创建HTTPS的监听器
		ln = tls.NewListener(ln, config)
	}

	// 服务器数据设置
	server.ln = ln   // 监听器
	server.handler = &WSHandler{                   // 创建一个新的服务器处理句柄
		maxConnNum:      server.MaxConnNum,        // 设置最大连接数
		pendingWriteNum: server.PendingWriteNum,   // 挂起写连接数
		maxMsgLen:       server.MaxMsgLen,         // 信息最大长度
		newAgent:        server.NewAgent,          // 新代理
		conns:           make(WebsocketConnSet),   // 连接池
		upgrader: websocket.Upgrader{              // WebSocket升级器（协议升级，从http协议升级为WebSocket协议）
			HandshakeTimeout: server.HTTPTimeout,  // 协议升级，握手认证超时时间
			CheckOrigin:      func(_ *http.Request) bool { return true },  // 协议升级检查源程序
		},
	}

	// 初始化HTTP服务器
	httpServer := &http.Server{
		Addr:           server.Addr,         // 服务器地址
		Handler:        server.handler,      // 服务器处理句柄
		ReadTimeout:    server.HTTPTimeout,  // HTTP头读取超时
		WriteTimeout:   server.HTTPTimeout,  // HTTP写超时
		MaxHeaderBytes: 1024,                // HTTP头最大长度（默认1k）
	}

	// 在一个新协程中运行HTTP服务器
	go httpServer.Serve(ln)
}

// 关闭WEBSocket服务器
func (server *WSServer) Close() {
	// 关闭服务器监听器
	server.ln.Close()

	// 锁住
	server.handler.mutexConns.Lock()
	// 遍历连接
	for conn := range server.handler.conns {
		// 依次关闭链接
		conn.Close()
	}
	server.handler.conns = nil
	// 解锁
	server.handler.mutexConns.Unlock()

	// 组同步，等待
	server.handler.wg.Wait()
}

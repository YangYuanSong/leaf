// WebSocket
package network

import (
	"github.com/gorilla/websocket"
	"github.com/name5566/leaf/log"
	"sync"
	"time"
)

// WebSocket客户端
type WSClient struct {
	sync.Mutex                            // 同步互斥锁
	Addr             string               // 连接地址
	ConnNum          int                  // 连接数
	ConnectInterval  time.Duration        // 连接间隔时间
	PendingWriteNum  int                  // 挂起写连接数
	MaxMsgLen        uint32               // 信息最大长度
	HandshakeTimeout time.Duration        // 协议升级握手超时时间
	AutoReconnect    bool                 // 是否自动重新链接
	NewAgent         func(*WSConn) Agent  // 新代理（输入WebSocket链接返回一个代理的函数）
	dialer           websocket.Dialer     // WebSocket会话
	conns            WebsocketConnSet     // WebSocket连接池
	wg               sync.WaitGroup       // 组同步
	closeFlag        bool                 // 关闭标识
}

// 启动WebSocket客户端
func (client *WSClient) Start() {
	// 初始化客户端
	client.init()

	// 根据连接数初始创建连接
	for i := 0; i < client.ConnNum; i++ {
		// 组同步+1
		client.wg.Add(1)
		// 在一个新的协程中创建连接
		go client.connect()
	}
}

func (client *WSClient) init() {
	// 锁住客户端
	client.Lock()
	defer client.Unlock()

	// 初始连接数
	if client.ConnNum <= 0 {
		// 默认打开一个链接
		client.ConnNum = 1
		log.Release("invalid ConnNum, reset to %v", client.ConnNum)
	}
	// 初始连接间隔时间
	if client.ConnectInterval <= 0 {
		// 默认间隔3秒钟
		client.ConnectInterval = 3 * time.Second
		log.Release("invalid ConnectInterval, reset to %v", client.ConnectInterval)
	}
	// 初始写挂起链接数
	if client.PendingWriteNum <= 0 {
		// 默认挂起链接数为100
		client.PendingWriteNum = 100
		log.Release("invalid PendingWriteNum, reset to %v", client.PendingWriteNum)
	}
	// 初始信息最大长度
	if client.MaxMsgLen <= 0 {
		// 默认信息最大长度为4K
		client.MaxMsgLen = 4096
		log.Release("invalid MaxMsgLen, reset to %v", client.MaxMsgLen)
	}
	// 协议升级握手超时时间
	if client.HandshakeTimeout <= 0 {
		// 默认超时时间为10秒
		client.HandshakeTimeout = 10 * time.Second
		log.Release("invalid HandshakeTimeout, reset to %v", client.HandshakeTimeout)
	}
	if client.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}
	if client.conns != nil {
		log.Fatal("client is running")
	}

	// 初始WebSocket链接池
	client.conns = make(WebsocketConnSet)
	// 设置关闭标识
	client.closeFlag = false
	// 新建WebSocket对话
	client.dialer = websocket.Dialer{
		HandshakeTimeout: client.HandshakeTimeout, // 设置握手超时时间
	}
}

// 客户端建立WebSocket会话
func (client *WSClient) dial() *websocket.Conn {
	// 死循环，确保返回的是打开的链接
	for {
		// 根据链接地址建立WebSocket链接
		conn, _, err := client.dialer.Dial(client.Addr, nil)
		if err == nil || client.closeFlag {
			return conn
		}

		// 打印日志，休眠再次链接
		log.Release("connect to %v error: %v", client.Addr, err)
		time.Sleep(client.ConnectInterval)
		continue
	}
}

// 建立WebSocket链接
func (client *WSClient) connect() {
	// 完成连接同步组-1
	defer client.wg.Done()

reconnect:
	// 获取客户端会话
	conn := client.dial()
	if conn == nil {
		return
	}
	// 设置客户端连接（读取最大长度）
	conn.SetReadLimit(int64(client.MaxMsgLen))

	// 客户端加锁
	client.Lock()
	// 判断客户端的关闭标识
	if client.closeFlag {
		client.Unlock()
		conn.Close()
		return
	}
	// 新链接加入到连接池
	client.conns[conn] = struct{}{}
	// 解锁
	client.Unlock()

	// 根据链接创建一个新的WebSocket链接
	wsConn := newWSConn(conn, client.PendingWriteNum, client.MaxMsgLen)
	// 初始代理
	agent := client.NewAgent(wsConn)
	// 代理运行
	agent.Run()

	// 清理工作
	// cleanup
	wsConn.Close()
	client.Lock()
	// 删除连接池中的连接
	delete(client.conns, conn)
	client.Unlock()
	// 执行代理的OnClose方法
	agent.OnClose()

	// 根据设置是否进行重新连接
	if client.AutoReconnect {
		// 休眠
		time.Sleep(client.ConnectInterval)
		goto reconnect
	}
}

// 关闭WebSocket客户端
func (client *WSClient) Close() {
	// 加锁
	client.Lock()
	// 设置关闭标识
	client.closeFlag = true
	// 遍历客户端连接诶
	for conn := range client.conns {
		// 关闭链接
		conn.Close()
	}
	client.conns = nil
	// 解锁
	client.Unlock()
	
	// 等待组同步
	client.wg.Wait()
}

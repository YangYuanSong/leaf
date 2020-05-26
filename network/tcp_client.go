package network

import (
	"github.com/name5566/leaf/log"
	"net"
	"sync"
	"time"
)

// TCP客户端
type TCPClient struct {
	sync.Mutex                           // 同步锁          
	Addr            string               // 地址
	ConnNum         int                  // 连接数
	ConnectInterval time.Duration        // 重复连接间隔时间
	PendingWriteNum int                  // 挂起链接
	AutoReconnect   bool                 // 是否自动重新连接
	NewAgent        func(*TCPConn) Agent // 代理（根据一个TCP链接返回一个代理接口的函数）
	conns           ConnSet              // 连接池（集合）
	wg              sync.WaitGroup       // 客户端连接组（同步）
	closeFlag       bool                 // 客户端关闭标识

	// 信息解析器
	// msg parser
	LenMsgLen    int        // 信息体长度占用字节数
	MinMsgLen    uint32     // 最小长度
	MaxMsgLen    uint32     // 最大长度
	LittleEndian bool       // 小端字节序（false,默认使用大端字节序）
	msgParser    *MsgParser // 信息解析器
}

// TCP客户端运行
func (client *TCPClient) Start() {
	// TCP客户端初始化
	client.init()

	// 根据最大连接数初始化完成链接
	for i := 0; i < client.ConnNum; i++ {
		// 客户度链接组同步+1
		client.wg.Add(1)
		// 新协程中完成连接
		go client.connect()
	}
}

// TCP客户端初始化
func (client *TCPClient) init() {
	// 锁住TCP客户端
	client.Lock()
	defer client.Unlock()

	// 初始连接数
	if client.ConnNum <= 0 {
		// 默认连接数为1
		client.ConnNum = 1
		// 输出初始化连接数
		log.Release("invalid ConnNum, reset to %v", client.ConnNum)
	}
	// 初始连接间隔
	if client.ConnectInterval <= 0 {
		client.ConnectInterval = 3 * time.Second
		log.Release("invalid ConnectInterval, reset to %v", client.ConnectInterval)
	}
	// 初始连接挂起数
	if client.PendingWriteNum <= 0 {
		client.PendingWriteNum = 100
		log.Release("invalid PendingWriteNum, reset to %v", client.PendingWriteNum)
	}
	// 代理初始判断
	if client.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}
	// 初始连接判断
	if client.conns != nil {
		log.Fatal("client is running")
	}

	// 初始化一个空的连接池
	client.conns = make(ConnSet)
	// 初始化关闭标识
	client.closeFlag = false

	// 信息解析器
	// msg parser
	msgParser := NewMsgParser()
	// 设置信息解析器的长度参数
	msgParser.SetMsgLen(client.LenMsgLen, client.MinMsgLen, client.MaxMsgLen)
	// 设置信息解析器的字节序（false，大端）
	msgParser.SetByteOrder(client.LittleEndian)
	// 信息解析器赋值
	client.msgParser = msgParser
}

// 执行链接对话
func (client *TCPClient) dial() net.Conn {
	// 死循环，保证返回是打开的链接
	for {
		// 打开Tcp链接
		conn, err := net.Dial("tcp", client.Addr)
		if err == nil || client.closeFlag {
			// 获取到链接，并且客户端未关闭，返回链接
			return conn
		}

		// 输出建立链接的错误	
		log.Release("connect to %v error: %v", client.Addr, err)
		//休眠
		time.Sleep(client.ConnectInterval)
		// 重复建立连接
		continue
	}
}

// TCP客户端完成连接
func (client *TCPClient) connect() {
	// 建立链接后组同步-1
	defer client.wg.Done()

// 重新建立链接
reconnect:
	// 执行链接对话
	conn := client.dial()
	if conn == nil {
		return
	}
	// 锁住TCP客户端
	client.Lock()
	if client.closeFlag {
		// 判断客户端是否已关闭
		client.Unlock()
		conn.Close()
		return
	}
	// 连接放入客户端链接池
	client.conns[conn] = struct{}{}
	// 解锁TCP客户端
	client.Unlock()

	// 根据连接创建一个新的TCP连接
	tcpConn := newTCPConn(conn, client.PendingWriteNum, client.msgParser)
	// 根据TCP链接，新建一个代理
	agent := client.NewAgent(tcpConn)
	// 运行代理
	agent.Run()

	// TCP链接关闭
	// cleanup
	tcpConn.Close()
	client.Lock()
	// 删除连接池中的连接
	delete(client.conns, conn)
	client.Unlock()
	// 执行代理OnClose方法
	agent.OnClose()

	// 判断是否自动重新链接
	if client.AutoReconnect {
		// 休眠
		time.Sleep(client.ConnectInterval)
		// 执行重新连接
		goto reconnect
	}
}

// TCP客户端关闭
func (client *TCPClient) Close() {
	// 锁住客户端
	client.Lock()
	// 设置客户端关闭标识
	client.closeFlag = true
	// 遍历所有链接
	for conn := range client.conns {
		// 关闭链接
		conn.Close()
	}
	client.conns = nil
	// 解锁
	client.Unlock()
	// 组同步等待
	client.wg.Wait()
}

// Gate 模块为Leaf提供接入功能。这个模块的功能很重要，是服务器的入口。它能同时监听TcpSocket和WebSocket。
// 主要流程是在接入连接的时候创建一个Agent，并将这个Agent通知给AgentRpc。
// 其核心其实是一个TcpServer和WebScoketServer，他的协议函数能够将socket字节流分包，封装为Msg传递给Agent。其工作流可以查看Server模块。
package gate

import (
	"github.com/name5566/leaf/chanrpc"
	"github.com/name5566/leaf/log"
	"github.com/name5566/leaf/network"
	"net"
	"reflect"
	"time"
)

// 门数据结构
type Gate struct {
	MaxConnNum      int                // 最大连接数
	PendingWriteNum int                // 挂起写连接最大数
	MaxMsgLen       uint32             // 信息最大长度
	Processor       network.Processor  // 网络处理器（对应数据的处理）
	AgentChanRPC    *chanrpc.Server    // 对应的RPC服务

	// websocket
	WSAddr      string        // 监听地址
	HTTPTimeout time.Duration // 超时间隔
	CertFile    string        // 证书文件
	KeyFile     string        // 秘钥文件

	// tcp
	TCPAddr      string       // 监听地址
	LenMsgLen    int          // 信息最大长度
	LittleEndian bool         // 字节序（默认大端序）
}

// 门运行（同时运行WebSocket和TCP服务器）
func (gate *Gate) Run(closeSig chan bool) {
	// WebSocket服务器
	var wsServer *network.WSServer                      // 服务器数据类型声明
	if gate.WSAddr != "" {
		wsServer = new(network.WSServer)                // 创建服务器
		wsServer.Addr = gate.WSAddr                     // 监听地址
		wsServer.MaxConnNum = gate.MaxConnNum           // 最大链接数
		wsServer.PendingWriteNum = gate.PendingWriteNum // 挂起写最大连接数
		wsServer.MaxMsgLen = gate.MaxMsgLen             // 信息最大长度
		wsServer.HTTPTimeout = gate.HTTPTimeout         // 超时设置
		wsServer.CertFile = gate.CertFile               // 证书文件
		wsServer.KeyFile = gate.KeyFile                 // 秘钥文件
		// 新代理实现函数定义（传入一个WebSocket链接，返回一个网络代理）
		wsServer.NewAgent = func(conn *network.WSConn) network.Agent {
			a := &agent{conn: conn, gate: gate}         // 新建一个代理
			if gate.AgentChanRPC != nil {               // 判断代理是否有通道过程调用
				gate.AgentChanRPC.Go("NewAgent", a)     // 通过通道调用 NewAgent模块
			}
			return a
		}
	}

	// TCP服务器
	var tcpServer *network.TCPServer
	if gate.TCPAddr != "" {
		tcpServer = new(network.TCPServer)               // 创建服务器
		tcpServer.Addr = gate.TCPAddr                    // 监听地址
		tcpServer.MaxConnNum = gate.MaxConnNum           // 最大链接数
		tcpServer.PendingWriteNum = gate.PendingWriteNum // 挂起写最大链接数
		tcpServer.LenMsgLen = gate.LenMsgLen             // 信息长度字节数
		tcpServer.MaxMsgLen = gate.MaxMsgLen             // 信息最大长度
		tcpServer.LittleEndian = gate.LittleEndian       // 字节序
		// 新代理实现函数定义（出入一个WebSocket链接，返回一个网络代理）
		tcpServer.NewAgent = func(conn *network.TCPConn) network.Agent {
			a := &agent{conn: conn, gate: gate}          // 新建一个代理
			if gate.AgentChanRPC != nil {                // 判断代理是否有通道过程调用
				gate.AgentChanRPC.Go("NewAgent", a)      // 通过通道调用NewAgent模块
			}
			return a
		}
	}

	// 启动WebSocket服务器
	if wsServer != nil {
		wsServer.Start()
	}
	// 启动TCP服务器
	if tcpServer != nil {
		tcpServer.Start()
	}
	// 从通道中获取关闭信号数据
	<-closeSig
	// 关闭WebSocket服务器
	if wsServer != nil {
		wsServer.Close()
	}
	// 关闭TCP服务器
	if tcpServer != nil {
		tcpServer.Close()
	}
}

func (gate *Gate) OnDestroy() {}

// 代理数据结构
// 实现了Agent接口, Gate
type agent struct {
	conn     network.Conn  // 网络连接
	gate     *Gate         // 传送门
	userData interface{}   // 用户数据
}

// 代理运行
func (a *agent) Run() {
	// 死循环，进行数据处理
	for {
		// 代理的连接中读取数据
		data, err := a.conn.ReadMsg()
		if err != nil {
			log.Debug("read message: %v", err)
			break
		}

		// 判断代理是否有处理器
		if a.gate.Processor != nil {
			// 调用处理器，解码数据
			msg, err := a.gate.Processor.Unmarshal(data)
			if err != nil {
				log.Debug("unmarshal message error: %v", err)
				break
			}
			// 调用处理器路由处理解析的数据
			err = a.gate.Processor.Route(msg, a)
			if err != nil {
				log.Debug("route message error: %v", err)
				break
			}
		}
	}
}

// 代理OnClose方法
func (a *agent) OnClose() {
	// 判断是否有模块通道调用
	if a.gate.AgentChanRPC != nil {
		// 通过通道调用CloseAgent方法，并且把代理信息作为参数来传输
		err := a.gate.AgentChanRPC.Call0("CloseAgent", a)
		if err != nil {
			log.Error("chanrpc error: %v", err)
		}
	}
}

// 代理写数据
func (a *agent) WriteMsg(msg interface{}) {
	if a.gate.Processor != nil {
		// 调用代理处理器，编码待传输的数据
		data, err := a.gate.Processor.Marshal(msg)
		if err != nil {
			log.Error("marshal message %v error: %v", reflect.TypeOf(msg), err)
			return
		}
		// 写数据
		err = a.conn.WriteMsg(data...)
		if err != nil {
			log.Error("write message %v error: %v", reflect.TypeOf(msg), err)
		}
	}
}

// 获取本地地址
func (a *agent) LocalAddr() net.Addr {
	return a.conn.LocalAddr()
}

// 获取远程地址
func (a *agent) RemoteAddr() net.Addr {
	return a.conn.RemoteAddr()
}

// 关闭代理
func (a *agent) Close() {
	a.conn.Close()
}

// 销毁代理
func (a *agent) Destroy() {
	a.conn.Destroy()
}

// 获取用户数据
func (a *agent) UserData() interface{} {
	return a.userData
}

func (a *agent) SetUserData(data interface{}) {
	a.userData = data
}

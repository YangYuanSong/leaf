package network

// 定义处理器接口
type Processor interface {
	// 路由方法
	// must goroutine safe
	Route(msg interface{}, userData interface{}) error
	// 解析数据
	// must goroutine safe
	Unmarshal(data []byte) (interface{}, error)
	// 加密数据
	// must goroutine safe
	Marshal(msg interface{}) ([][]byte, error)
}

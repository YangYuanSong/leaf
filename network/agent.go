package network

// 定义代理接口
type Agent interface {
	// 运行方法
	Run()
	// 进行关闭方法
	OnClose()
}

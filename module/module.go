// Module 为Leaf提供模块化支持。
// module可以算做是整个leaf框架的入口
package module

import (
	"github.com/name5566/leaf/conf"
	"github.com/name5566/leaf/log"
	"runtime"
	"sync"
)

// 定义接口，确定模型开放的方法
// 数据完全被封装，只暴露对应的接口方法
type Module interface {
	OnInit()
	OnDestroy()
	Run(closeSig chan bool)
}

// 定义数据类型，确定具体的数据
type module struct {
	mi       Module          // 关联接口
	closeSig chan bool       // 关闭信号
	wg       sync.WaitGroup  // 组同步
}

// 定义一个全局的模型切片
var mods []*module

// 注册一个模型
// 传入的接口数据类型可以自带方法和数据
func Register(mi Module) {
	// 新模型
	m := new(module)
	// 接口类型数据
	m.mi = mi
	// 关闭信号采用 容量为1的bool 通道实现
	m.closeSig = make(chan bool, 1)
	// 新模型添加到全局模型中
	mods = append(mods, m)
}

// 模型初始化
func Init() {
	// 依次调用模型的OnInit方法，完成模型的初始化
	for i := 0; i < len(mods); i++ {
		mods[i].mi.OnInit()
	}
	// 依次运行模型，并使用组同步，来同步结束每个模型
	for i := 0; i < len(mods); i++ {
		m := mods[i]
		m.wg.Add(1)
		// 在一个新的协程中运行模型
		go run(m)
	}
}

// 模型销毁
func Destroy() {
	// 依次处理模型
	for i := len(mods) - 1; i >= 0; i-- {
		m := mods[i]
		// 发送模型关闭信号
		m.closeSig <- true
		//等待模型结束（阻塞）
		m.wg.Wait()
		// 销毁模型
		destroy(m)
	}
}

// 运行模型
func run(m *module) {
	// 调用实现接口数据的Run方法
	m.mi.Run(m.closeSig)
	// 同步结束
	m.wg.Done()
}

// 销毁模型
func destroy(m *module) {
	defer func() {
		// 捕获异常
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				// 输出堆栈信息和错误信息
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Error("%v: %s", r, buf[:l])
			} else {
				// 输出错误信息
				log.Error("%v", r)
			}
		}
	}()

	// 调用 OnDestroy 方法
	m.mi.OnDestroy()
}

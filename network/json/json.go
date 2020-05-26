package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/name5566/leaf/chanrpc"
	"github.com/name5566/leaf/log"
	"reflect"
)

// 处理器数据结构
type Processor struct {
	msgInfo map[string]*MsgInfo
}

// 信息数据结构
type MsgInfo struct {
	msgType       reflect.Type      // 信息类型
	msgRouter     *chanrpc.Server   // 通道调用服务
	msgHandler    MsgHandler        // 信息处理句柄
	msgRawHandler MsgHandler        // 信息原生句柄
}

// 信息句柄
type MsgHandler func([]interface{})

// 原始信息
type MsgRaw struct {
	msgID      string           // 信息ID
	msgRawData json.RawMessage  // JSON原始信息（原始编码的json）
}

// 新的处理器
func NewProcessor() *Processor {
	// 新建一个处理器
	p := new(Processor)
	// 初始信息内容
	p.msgInfo = make(map[string]*MsgInfo)
	return p
}

// 根据信息注册一个处理器
// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) Register(msg interface{}) string {
	// 获取信息的反射类型
	msgType := reflect.TypeOf(msg)
	// 只支持指针类型数据
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}
	// 反射元素的名称定义为信息ID
	msgID := msgType.Elem().Name()
	if msgID == "" {
		log.Fatal("unnamed json message")
	}
	// 判断信息处理器是否已经存在
	if _, ok := p.msgInfo[msgID]; ok {
		log.Fatal("message %v is already registered", msgID)
	}

	// 创建新的信息处理器
	i := new(MsgInfo)
	// 处理器类型
	i.msgType = msgType
	// 数据写入处理器
	p.msgInfo[msgID] = i
	return msgID
}

// 信息处理器设置路由
// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetRouter(msg interface{}, msgRouter *chanrpc.Server) {
	// 获取反射类型
	msgType := reflect.TypeOf(msg)
	// 只支持指针数据类型
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}
	// 反射元素的名称定义为信息ID
	msgID := msgType.Elem().Name()
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}
	
	// 设置路由器
	i.msgRouter = msgRouter
}

// 处理器设置处理句柄
// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetHandler(msg interface{}, msgHandler MsgHandler) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}
	// 获取ID
	msgID := msgType.Elem().Name()
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}
	
	// 设置句柄
	i.msgHandler = msgHandler
}

// 处理器设置原始处理句柄
// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetRawHandler(msgID string, msgRawHandler MsgHandler) {
	// 获取处理器
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}

	// 设置原始句柄
	i.msgRawHandler = msgRawHandler
}

// 处理器路由
// goroutine safe
func (p *Processor) Route(msg interface{}, userData interface{}) error {
	// 判断信息是否是原始信息
	// raw
	if msgRaw, ok := msg.(MsgRaw); ok {
		i, ok := p.msgInfo[msgRaw.msgID]
		if !ok {
			return fmt.Errorf("message %v not registered", msgRaw.msgID)
		}
		if i.msgRawHandler != nil {
			i.msgRawHandler([]interface{}{msgRaw.msgID, msgRaw.msgRawData, userData})
		}
		return nil
	}

	// 获取信息的反射类型
	// json
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return errors.New("json message pointer required")
	}
	// 获取ID
	msgID := msgType.Elem().Name()
	// 获取处理器
	i, ok := p.msgInfo[msgID]
	if !ok {
		return fmt.Errorf("message %v not registered", msgID)
	}
	if i.msgHandler != nil {
		// 处理句柄
		i.msgHandler([]interface{}{msg, userData})
	}
	if i.msgRouter != nil {
		// 处理路由
		i.msgRouter.Go(msgType, msg, userData)
	}
	return nil
}

// 解码数据
// goroutine safe
func (p *Processor) Unmarshal(data []byte) (interface{}, error) {
	// 预定义数据
	var m map[string]json.RawMessage
	// 通过JSON的Unmarshal解码字节切片的JSON字符串，并且结果填充到预定义数据m中
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}
	if len(m) != 1 {
		return nil, errors.New("invalid json data")
	}

	// 遍历解码的JSON数据
	for msgID, data := range m {
		i, ok := p.msgInfo[msgID]
		if !ok {
			return nil, fmt.Errorf("message %v not registered", msgID)
		}

		// msg
		if i.msgRawHandler != nil {
			// 获取原始JSON信息
			return MsgRaw{msgID, data}, nil
		} else {
			// 获取解析后的JSON数据
			msg := reflect.New(i.msgType.Elem()).Interface()
			return msg, json.Unmarshal(data, msg)
		}
	}

	panic("bug")
}

// 编码数据
// goroutine safe
func (p *Processor) Marshal(msg interface{}) ([][]byte, error) {
	// 反射类型
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return nil, errors.New("json message pointer required")
	}
	// 获取处理器ID
	msgID := msgType.Elem().Name()
	if _, ok := p.msgInfo[msgID]; !ok {
		return nil, fmt.Errorf("message %v not registered", msgID)
	}

	// 调用JSON的Marshal编码数据
	// data
	m := map[string]interface{}{msgID: msg}
	data, err := json.Marshal(m)
	return [][]byte{data}, err
}

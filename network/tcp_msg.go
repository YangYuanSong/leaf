// TCP链接信息解析器
// 主要实现业务数据长度设置和判断；根据设置业务数据的开头（1、2、4）个字节存储了数据的长度
// 同时完成业务数据的读取（去掉业务数据开头的几个表示长度的字节）
package network

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)


// 信息解析器
// --------------
// | len | data |
// --------------
type MsgParser struct {
	lenMsgLen    int      // 信息长度
	minMsgLen    uint32   // 最小信息长度
	maxMsgLen    uint32   // 最大信息长度
	littleEndian bool     // 小端字节序标识
}

// 新建一个信息解析器
func NewMsgParser() *MsgParser {
	p := new(MsgParser)
	p.lenMsgLen = 2         // 信息最长数量限制类型(1\2\4字节无符号整数)
	p.minMsgLen = 1         // 信息默认最短1
	p.maxMsgLen = 4096      // 信息默认最长4k
	p.littleEndian = false  // 设置成大端字节序

	return p
}

// 设置解析器信息的最小和最大长度
// It's dangerous to call the method on reading or writing
func (p *MsgParser) SetMsgLen(lenMsgLen int, minMsgLen uint32, maxMsgLen uint32) {
	// 支持1、2、4个字节长度的非符号整数
	if lenMsgLen == 1 || lenMsgLen == 2 || lenMsgLen == 4 {
		p.lenMsgLen = lenMsgLen
	}
	if minMsgLen != 0 {
		p.minMsgLen = minMsgLen
	}
	if maxMsgLen != 0 {
		p.maxMsgLen = maxMsgLen
	}

	// 根据长度限制类型确定信息体最大长度
	var max uint32
	switch p.lenMsgLen {
	case 1:
		max = math.MaxUint8
	case 2:
		max = math.MaxUint16
	case 4:
		max = math.MaxUint32
	}
	if p.minMsgLen > max {
		p.minMsgLen = max
	}
	if p.maxMsgLen > max {
		p.maxMsgLen = max
	}
}

// 设置字节序
// It's dangerous to call the method on reading or writing
func (p *MsgParser) SetByteOrder(littleEndian bool) {
	p.littleEndian = littleEndian
}

// 解析器读取数据
// goroutine safe
func (p *MsgParser) Read(conn *TCPConn) ([]byte, error) {
	// 定义4个字节的数组
	var b [4]byte
	// 业务数据的长度（字节切片）
	bufMsgLen := b[:p.lenMsgLen]

	// 数据长度读取数据到bufMsgLen中
	// read len
	if _, err := io.ReadFull(conn, bufMsgLen); err != nil {
		return nil, err
	}

	// 计算buffer的长度，根据大小端计算存储在切片中的整数值
	// parse len
	var msgLen uint32
	switch p.lenMsgLen {
	case 1:
		// 一个字节无符号整数长度
		msgLen = uint32(bufMsgLen[0])
	case 2:
		// 两个字节无符号整数长度
		if p.littleEndian {
			msgLen = uint32(binary.LittleEndian.Uint16(bufMsgLen))
		} else {
			msgLen = uint32(binary.BigEndian.Uint16(bufMsgLen))
		}
	case 4:
		// 四个字节无符号整数长度
		if p.littleEndian {
			msgLen = binary.LittleEndian.Uint32(bufMsgLen)
		} else {
			msgLen = binary.BigEndian.Uint32(bufMsgLen)
		}
	}

	// 判断长度
	// check len
	if msgLen > p.maxMsgLen {
		// 长度大于最大长度
		return nil, errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		// 长度小于最小长度
		return nil, errors.New("message too short")
	}

	// 读取业务数据到数据中
	// data
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgData); err != nil {
		return nil, err
	}

	// 返回去掉长度的业务数据
	return msgData, nil
}

// 解析器写入数据
// 一次可以写入多个字节切片
// goroutine safe
func (p *MsgParser) Write(conn *TCPConn, args ...[]byte) error {
	// 计算写入数据的长度
	// get len
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		// 循环计算切片的长度
		msgLen += uint32(len(args[i]))
	}

	// 判断写入数据的长度
	// check len
	if msgLen > p.maxMsgLen {
		return errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		return errors.New("message too short")
	}

	// 生成业务数据切片（包含数据长度的部分）
	msg := make([]byte, uint32(p.lenMsgLen)+msgLen)

	// 写入数据长度部分的数据
	// write len
	switch p.lenMsgLen {
	case 1:
		// 一个字节
		msg[0] = byte(msgLen)
	case 2:
		// 两个字节
		if p.littleEndian {
			binary.LittleEndian.PutUint16(msg, uint16(msgLen))
		} else {
			binary.BigEndian.PutUint16(msg, uint16(msgLen))
		}
	case 4:
		// 4个字节
		if p.littleEndian {
			binary.LittleEndian.PutUint32(msg, msgLen)
		} else {
			binary.BigEndian.PutUint32(msg, msgLen)
		}
	}

	// 写入实际数据
	// write data
	//业务数据切片开始位置
	l := p.lenMsgLen
	// 循环待写入的数据
	for i := 0; i < len(args); i++ {
		// 拷贝待写入的数据到业务数据切片
		copy(msg[l:], args[i])
		// 移动切片拷贝写入的开始位置
		l += len(args[i])
	}

	// 数据写入连接
	conn.Write(msg)

	return nil
}

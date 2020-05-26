// 工具包 - 随机功能
package util

import (
	"math/rand"
	"time"
)

// 包初始化
func init() {
	// 随机种子
	rand.Seed(time.Now().UnixNano())
}

// 从给定数据中随机返回一个数的下标
func RandGroup(p ...uint32) int {
	if p == nil {
		panic("args not found")
	}

	r := make([]uint32, len(p))
	for i := 0; i < len(p); i++ {
		if i == 0 {
			r[0] = p[0]
		} else {
			// 与前一位相加（放大随机区间）
			r[i] = r[i-1] + p[i]
		}
	}
	
	// 最后一个元素是否为0
	rl := r[len(r)-1]
	if rl == 0 {
		return 0
	}

	// 放大后的最大值中取随机数
	rn := uint32(rand.Int63n(int64(rl)))
	for i := 0; i < len(r); i++ {
		if rn < r[i] {
			// 返回随机区间的某个值
			return i
		}
	}

	panic("bug")
}

// 获取两个整数间的一个随机数
func RandInterval(b1, b2 int32) int32 {
	if b1 == b2 {
		return b1
	}
	// 确定大小数
	min, max := int64(b1), int64(b2)
	if min > max {
		min, max = max, min
	}
	// 返回0到差集之间的一个随机整数 + 小的数
	return int32(rand.Int63n(max-min+1) + min)
}

// 获取两个整数间的n个随机数
func RandIntervalN(b1, b2 int32, n uint32) []int32 {
	if b1 == b2 {
		return []int32{b1}
	}

	min, max := int64(b1), int64(b2)
	if min > max {
		min, max = max, min
	}
	// 区间长度
	l := max - min + 1
	if int64(n) > l {
		n = uint32(l)
	}

	// 使用切片存储返回数据
	r := make([]int32, n)
	// 使用映射存储中间运算数据（收敛的、不取最大值）
	m := make(map[int32]int32)
	for i := uint32(0); i < n; i++ {
		// 取随机数
		v := int32(rand.Int63n(l) + min)
		// 随机数是否在map中
		if mv, ok := m[v]; ok {
			r[i] = mv
		} else {
			r[i] = v
		}
		// 最大值
		lv := int32(l - 1 + min)
		// map中不存储最大值
		if v != lv {
			if mv, ok := m[lv]; ok {
				m[v] = mv
			} else {
				m[v] = lv
			}
		}

		// 随机区间最大值收敛
		l--
	}

	return r
}

// 定时器表达式
// 类似linux crontab 表达式，解析定时设置
package timer

// reference: https://github.com/robfig/cron
import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Field name   | Mandatory? | Allowed values | Allowed special characters
// ----------   | ---------- | -------------- | --------------------------
// Seconds      | No         | 0-59           | * / , -
// Minutes      | Yes        | 0-59           | * / , -
// Hours        | Yes        | 0-23           | * / , -
// Day of month | Yes        | 1-31           | * / , -
// Month        | Yes        | 1-12           | * / , -
// Day of week  | Yes        | 0-6            | * / , -
type CronExpr struct {
	sec   uint64   // 秒
	min   uint64   // 分
	hour  uint64   // 小时
	dom   uint64   // 一月中的第几天
	month uint64   // 月份
	dow   uint64   // 一周中的第几天
}

// 新建定时器表达式
// goroutine safe
func NewCronExpr(expr string) (cronExpr *CronExpr, err error) {
	// 空格分割字符串
	fields := strings.Fields(expr)
	if len(fields) != 5 && len(fields) != 6 {
		err = fmt.Errorf("invalid expr %v: expected 5 or 6 fields, got %v", expr, len(fields))
		return
	}

	// 不足6位的补足6位
	if len(fields) == 5 {
		fields = append([]string{"0"}, fields...)
	}

	// 新建一个定时表达式
	cronExpr = new(CronExpr)
	// 第一位参数 - 秒
	// Seconds
	cronExpr.sec, err = parseCronField(fields[0], 0, 59)
	if err != nil {
		goto onError
	}
	// 第二位参数 - 分 
	// Minutes
	cronExpr.min, err = parseCronField(fields[1], 0, 59)
	if err != nil {
		goto onError
	}
	// 第三位参数 - 小时 
	// Hours
	cronExpr.hour, err = parseCronField(fields[2], 0, 23)
	if err != nil {
		goto onError
	}
	// 第四位参数 - 一月中的第几天
	// Day of month
	cronExpr.dom, err = parseCronField(fields[3], 1, 31)
	if err != nil {
		goto onError
	}
	// 第五位参数 - 月份
	// Month
	cronExpr.month, err = parseCronField(fields[4], 1, 12)
	if err != nil {
		goto onError
	}
	// 第六位参数 - 一周中的第几天
	// Day of week
	cronExpr.dow, err = parseCronField(fields[5], 0, 6)
	if err != nil {
		goto onError
	}
	return

// 参数解析出现错误，打印错误信息
onError:
	err = fmt.Errorf("invalid expr %v: %v", expr, err)
	return
}

// 解析字符串形式的字段参数到整数形式
// 可以设置最大和最小的区间值, 区间接收 / , - 分割 
// 可以接收通配符 * 匹配全部
// 1. *                           匹配全部
// 2. num                         匹配某个值
// 3. num-num                     匹配一个区间
// 4. */num                       匹配每多少进行一次（频率）
// 5. num/num (means num-max/num) 匹配每多少进行固定多少次（频率）
// 6. num-num/num                 匹配每多少进行区间多少次（频率）
func parseCronField(field string, min int, max int) (cronField uint64, err error) {
	// 使用逗号分割字段
	fields := strings.Split(field, ",")
	// 循环迭代字段
	for _, field := range fields {
		// 使用反斜杠分割字段，确定频率
		rangeAndIncr := strings.Split(field, "/")
		if len(rangeAndIncr) > 2 {
			err = fmt.Errorf("too many slashes: %v", field)
			return
		}

		// 使用短横线分割字段，确定开始和结束
		// range
		startAndEnd := strings.Split(rangeAndIncr[0], "-")
		if len(startAndEnd) > 2 {
			err = fmt.Errorf("too many hyphens: %v", rangeAndIncr[0])
			return
		}

		// 时间区间的开始和结束
		var start, end int
		if startAndEnd[0] == "*" {
			// 匹配全部的情况时间区间：开始取最小值，结束取最大值
			if len(startAndEnd) != 1 {
				err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
				return
			}
			start = min
			end = max
		} else {
			// 开始值
			// start
			start, err = strconv.Atoi(startAndEnd[0])
			if err != nil {
				err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
				return
			}
			// 结束值
			// end
			if len(startAndEnd) == 1 {
				if len(rangeAndIncr) == 2 {
					end = max
				} else {
					end = start
				}
			} else {
				end, err = strconv.Atoi(startAndEnd[1])
				if err != nil {
					err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
					return
				}
			}
		}

		if start > end {
			err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
			return
		}
		if start < min {
			err = fmt.Errorf("out of range [%v, %v]: %v", min, max, rangeAndIncr[0])
			return
		}
		if end > max {
			err = fmt.Errorf("out of range [%v, %v]: %v", min, max, rangeAndIncr[0])
			return
		}

		// 增量
		// increment
		var incr int
		if len(rangeAndIncr) == 1 {
			incr = 1
		} else {
			incr, err = strconv.Atoi(rangeAndIncr[1])
			if err != nil {
				err = fmt.Errorf("invalid increment: %v", rangeAndIncr[1])
				return
			}
			if incr <= 0 {
				err = fmt.Errorf("invalid increment: %v", rangeAndIncr[1])
				return
			}
		}

		// 定时字段,通过掩码的方式确定是否有某项定时
		// cronField
		if incr == 1 {
			cronField |= ^(math.MaxUint64 << uint(end+1)) & (math.MaxUint64 << uint(start))
		} else {
			for i := start; i <= end; i += incr {
				cronField |= 1 << uint(i)
			}
		}
	}

	return
}

// 某个时间是否匹配某一天
func (e *CronExpr) matchDay(t time.Time) bool {
	// 一月中的某天
	// day-of-month blank
	if e.dom == 0xfffffffe {
		return 1<<uint(t.Weekday())&e.dow != 0
	}

	// 一周中的某天
	// day-of-week blank
	if e.dow == 0x7f {
		return 1<<uint(t.Day())&e.dom != 0
	}

	return 1<<uint(t.Weekday())&e.dow != 0 ||
		1<<uint(t.Day())&e.dom != 0
}

// 定时表达式  某个时间点的下次时间
// goroutine safe
func (e *CronExpr) Next(t time.Time) time.Time {
	// the upcoming second
	t = t.Truncate(time.Second).Add(time.Second)

	year := t.Year()
	initFlag := false

// 通过递归确定时间
retry:
	// Year
	if t.Year() > year+1 {
		return time.Time{}
	}

	// Month
	for 1<<uint(t.Month())&e.month == 0 {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		}

		t = t.AddDate(0, 1, 0)
		if t.Month() == time.January {
			goto retry
		}
	}

	// Day
	for !e.matchDay(t) {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		}

		t = t.AddDate(0, 0, 1)
		if t.Day() == 1 {
			goto retry
		}
	}

	// Hours
	for 1<<uint(t.Hour())&e.hour == 0 {
		if !initFlag {
			initFlag = true
			t = t.Truncate(time.Hour)
		}

		t = t.Add(time.Hour)
		if t.Hour() == 0 {
			goto retry
		}
	}

	// Minutes
	for 1<<uint(t.Minute())&e.min == 0 {
		if !initFlag {
			initFlag = true
			t = t.Truncate(time.Minute)
		}

		t = t.Add(time.Minute)
		if t.Minute() == 0 {
			goto retry
		}
	}

	// Seconds
	for 1<<uint(t.Second())&e.sec == 0 {
		if !initFlag {
			initFlag = true
		}

		t = t.Add(time.Second)
		if t.Second() == 0 {
			goto retry
		}
	}

	return t
}

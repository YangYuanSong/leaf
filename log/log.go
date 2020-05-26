// 日志模块
package log

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

// 日志等级
// levels
const (
	debugLevel   = 0 // 调试
	releaseLevel = 1 // 
	errorLevel   = 2 // 错误
	fatalLevel   = 3 // 致命终止
)

// 日志等级前缀
const (
	printDebugLevel   = "[debug  ] "
	printReleaseLevel = "[release] "
	printErrorLevel   = "[error  ] "
	printFatalLevel   = "[fatal  ] "
)

// 日志器数据格式
type Logger struct {
	level      int          // 日志器等级
	baseLogger *log.Logger  // 标准的日志记录器
	baseFile   *os.File     // 日志文件
}

// 创建一个新的日志器
// strLevel  日志等级
// pathname  日志文件路径
// flag      标准日志的标识
func New(strLevel string, pathname string, flag int) (*Logger, error) {
	// 日志等级
	// level
	var level int
	switch strings.ToLower(strLevel) {
	case "debug":
		level = debugLevel
	case "release":
		level = releaseLevel
	case "error":
		level = errorLevel
	case "fatal":
		level = fatalLevel
	default:
		return nil, errors.New("unknown level: " + strLevel)
	}

	// 日志器
	// logger
	var baseLogger *log.Logger
	var baseFile *os.File
	if pathname != "" {
		// 当前时间
		now := time.Now()
		// 利用当前时间创建日志文件名
		filename := fmt.Sprintf("%d%02d%02d_%02d_%02d_%02d.log",
			now.Year(),
			now.Month(),
			now.Day(),
			now.Hour(),
			now.Minute(),
			now.Second())
		// 创建日志文件
		file, err := os.Create(path.Join(pathname, filename))
		if err != nil {
			return nil, err
		}
		// 利用标准日志创建新的日志记录器
		baseLogger = log.New(file, "", flag)
		baseFile = file
	} else {
		// 日志路径为空，日志输出到STDOUT上
		baseLogger = log.New(os.Stdout, "", flag)
	}

	// new
	logger := new(Logger)
	logger.level = level            // 日志等级
	logger.baseLogger = baseLogger  // 标准的日志记录器
	logger.baseFile = baseFile      // 日志文件

	return logger, nil
}

// 关闭标准日志记录器
// It's dangerous to call the method on logging
func (logger *Logger) Close() {
	if logger.baseFile != nil {
		// 关闭打开文件
		logger.baseFile.Close()
	}
	// 关闭日志器
	logger.baseLogger = nil
	logger.baseFile = nil
}

// 打印日志
func (logger *Logger) doPrintf(level int, printLevel string, format string, a ...interface{}) {
	// ....
	if level < logger.level {
		return
	}
	// 日志器是否关闭
	if logger.baseLogger == nil {
		panic("logger closed")
	}
	// 利用打印级别 和 日志格式组合成日志记录的格式
	// 采用标准日志进行 格式化输出
	format = printLevel + format
	logger.baseLogger.Output(3, fmt.Sprintf(format, a...))

	// 如果日志是致命的则结束程序
	if level == fatalLevel {
		os.Exit(1)
	}
}

// 记录日志 - 调试
func (logger *Logger) Debug(format string, a ...interface{}) {
	logger.doPrintf(debugLevel, printDebugLevel, format, a...)
}

// 记录日志 - 发布
func (logger *Logger) Release(format string, a ...interface{}) {
	logger.doPrintf(releaseLevel, printReleaseLevel, format, a...)
}

// 记录日志 - 发生错误
func (logger *Logger) Error(format string, a ...interface{}) {
	logger.doPrintf(errorLevel, printErrorLevel, format, a...)
}

// 记录日志 - 致命结束
func (logger *Logger) Fatal(format string, a ...interface{}) {
	logger.doPrintf(fatalLevel, printFatalLevel, format, a...)
}

// 初始化默认的日志器（默认级别debug）
var gLogger, _ = New("debug", "", log.LstdFlags)

// 替换默认日志器
// It's dangerous to call the method on logging
func Export(logger *Logger) {
	if logger != nil {
		gLogger = logger
	}
}

// 默认日志器 - 调试
func Debug(format string, a ...interface{}) {
	gLogger.doPrintf(debugLevel, printDebugLevel, format, a...)
}

// 默认日志器 - 发布
func Release(format string, a ...interface{}) {
	gLogger.doPrintf(releaseLevel, printReleaseLevel, format, a...)
}

// 默认日志器 - 错误
func Error(format string, a ...interface{}) {
	gLogger.doPrintf(errorLevel, printErrorLevel, format, a...)
}

// 默认日志器 - 致命结束
func Fatal(format string, a ...interface{}) {
	gLogger.doPrintf(fatalLevel, printFatalLevel, format, a...)
}

// 关闭默认的日志器
func Close() {
	gLogger.Close()
}

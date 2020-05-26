package conf

var (
	LenStackBuf = 4096  // 堆栈 Buffer 大小

	// log
	LogLevel string     // 日志级别
	LogPath  string     // 日志文件路径
	LogFlag  int        // 日志标识

	// console
	ConsolePort   int                 // 控制台端口
	ConsolePrompt string = "Leaf# "   // 
	ProfilePath   string

	// cluster
	ListenAddr      string
	ConnAddrs       []string
	PendingWriteNum int
)

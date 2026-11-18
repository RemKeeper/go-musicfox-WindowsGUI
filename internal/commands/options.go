package commands

var GlobalOptions struct {
	PProfMode bool
	DebugMode bool
	PureMode  bool
	// SourceOverride 是命令行 --source 指定的音乐渠道，空串表示沿用配置。
	SourceOverride string
}

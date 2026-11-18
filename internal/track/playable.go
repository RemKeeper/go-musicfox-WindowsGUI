package track

// PlayableInfo 是歌曲的可播放信息，由各渠道的 Fetcher 产出。
//
// 该类型原先定义在 utils/netease，为支持多渠道已提升到 track 包，
// 使 track 不再依赖具体渠道实现。
type PlayableInfo struct {
	URL       string
	MusicType string
	Size      int64
	// Quality 是实际拿到的音质标签（如 "flac"、"mp3 320k"、"ogg"）。
	//
	// 各渠道自行填写；状态栏用它显示「当前歌曲实际音质」，
	// 而不是配置里的期望音质——两者常常不同（降级链会回退）。
	Quality string
}

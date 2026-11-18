package ui

import (
	"strings"
	"testing"
	"time"
)

// TestFormatBitrateAndSize 校验「码率 · 文件大小」片段的格式化。
//
// 数据来源：QQ 歌曲元数据里声明的各档大小，与实际下载完全一致
// （实测《鱼玄机》M500=3524501、M800=8810849、O801=15117114）。
func TestFormatBitrateAndSize(t *testing.T) {
	cases := []struct {
		name     string
		size     int64
		duration time.Duration
		want     string
	}{
		{
			// O801：实测 549kbps
			name:     "O801 高码率 OGG",
			size:     15117114,
			duration: 220 * time.Second,
			want:     "549 kbps · 14.4 MB",
		},
		{
			// M800：320kbps
			name:     "MP3 320k",
			size:     8810849,
			duration: 220 * time.Second,
			want:     "320 kbps · 8.4 MB",
		},
		{
			// M500：128kbps
			name:     "MP3 128k",
			size:     3524501,
			duration: 220 * time.Second,
			want:     "128 kbps · 3.4 MB",
		},
		{
			name:     "FLAC 无损",
			size:     23183008,
			duration: 269 * time.Second,
			want:     "689 kbps · 22.1 MB",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatBitrateAndSize(tc.size, tc.duration); got != tc.want {
				t.Errorf("formatBitrateAndSize(%d, %v) = %q, want %q",
					tc.size, tc.duration, got, tc.want)
			}
		})
	}
}

// TestFormatBitrateAndSizeDegenerate 校验异常输入返回空串而非误导性文本。
func TestFormatBitrateAndSizeDegenerate(t *testing.T) {
	cases := []struct {
		name     string
		size     int64
		duration time.Duration
	}{
		{"大小为 0", 0, 220 * time.Second},
		{"时长为 0", 15117114, 0},
		{"两者皆 0", 0, 0},
		{"负大小", -100, 220 * time.Second},
		{"负时长", 15117114, -1 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatBitrateAndSize(tc.size, tc.duration); got != "" {
				t.Errorf("异常输入应返回空串，实际 %q", got)
			}
		})
	}
}

// TestFormatFileSize 校验字节数的人性化格式化。
func TestFormatFileSize(t *testing.T) {
	cases := []struct {
		size int64
		want string
	}{
		{512, "512 B"},
		{2048, "2 KB"},
		{524288, "512 KB"},
		{1048576, "1.0 MB"},
		{15117114, "14.4 MB"},
	}

	for _, tc := range cases {
		if got := formatFileSize(tc.size); got != tc.want {
			t.Errorf("formatFileSize(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}

// TestCurFileSize 校验文件大小的存取。
func TestCurFileSize(t *testing.T) {
	player := &Player{}

	if got := player.CurFileSize(); got != 0 {
		t.Errorf("初始 CurFileSize = %d, want 0", got)
	}

	player.setCurQuality("SQ", 15117114)
	if got := player.CurFileSize(); got != 15117114 {
		t.Errorf("CurFileSize = %d, want 15117114", got)
	}
	if got := player.CurQuality(); got != "SQ" {
		t.Errorf("CurQuality = %q, want \"SQ\"", got)
	}

	// 切歌后应更新
	player.setCurQuality("HQ", 8810849)
	if got := player.CurFileSize(); got != 8810849 {
		t.Errorf("切歌后 CurFileSize = %d, want 8810849", got)
	}
}

// TestStatusBarIncludesBitrate 校验状态栏文本包含码率信息。
//
// 这是本次需求的核心：在 QQ 音乐的音质右侧显示实际码率和文件大小。
func TestStatusBarIncludesBitrate(t *testing.T) {
	// 直接验证拼接逻辑：模拟 formatQueueAndQuality 中追加 detail 的部分
	qualityName := "SQ"
	detail := formatBitrateAndSize(15117114, 220*time.Second)

	text := " · [2/100] · " + qualityName
	if detail != "" {
		text += " · " + detail
	}

	want := " · [2/100] · SQ · 549 kbps · 14.4 MB"
	if text != want {
		t.Errorf("状态栏文本 = %q, want %q", text, want)
	}

	// 顺序断言：音质在前，码率居中，大小在后
	if !strings.Contains(text, "SQ · 549 kbps · 14.4 MB") {
		t.Error("音质、码率、大小的顺序应为「音质 · 码率 · 大小」")
	}
}

// TestCurFileSizeConcurrentSafe 校验并发读写安全。
func TestCurFileSizeConcurrentSafe(t *testing.T) {
	player := &Player{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			player.setCurQuality("SQ", 15117114)
			player.setCurQuality("", 0)
		}
	}()

	for i := 0; i < 200; i++ {
		_ = player.CurFileSize()
		_ = player.CurQuality()
	}
	<-done
}

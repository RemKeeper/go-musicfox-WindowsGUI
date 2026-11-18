package track

import (
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/netease-music/service"
)

// TestCacheKeyIncludesSource 是渠道隔离的关键回归测试：
// 网易云与 QQ 音乐的同号歌曲必须落在不同的缓存文件上。
func TestCacheKeyIncludesSource(t *testing.T) {
	c := &Cacher{}

	neteaseSong := structs.Song{Id: 12345}
	qqSong := structs.Song{Id: 12345, Source: structs.SourceQQ, Mid: "003abc"}

	neteaseKey := c.buildKey(neteaseSong, service.Standard, "mp3")
	qqKey := c.buildKey(qqSong, service.Standard, "mp3")

	if neteaseKey == qqKey {
		t.Fatalf("不同渠道的同号歌曲产生了相同缓存键: %q", neteaseKey)
	}
	if neteaseKey != "netease-12345-1.mp3" {
		t.Errorf("网易云缓存键格式不符: %q", neteaseKey)
	}
	if qqKey != "qq-12345-1.mp3" {
		t.Errorf("QQ 音乐缓存键格式不符: %q", qqKey)
	}
}

// TestCacheKeyEmptySourceIsNetease 校验空 Source 归一为网易云，
// 保证旧版本写入的缓存文件仍能被命中。
func TestCacheKeyEmptySourceIsNetease(t *testing.T) {
	c := &Cacher{}

	legacy := structs.Song{Id: 999} // 旧快照没有 Source 字段
	if got := c.buildKey(legacy, service.Lossless, "flac"); got != "netease-999-4.flac" {
		t.Errorf("旧数据缓存键不符: %q", got)
	}
}

// TestCacheKeyQualityPriority 校验音质优先级参与键构造。
func TestCacheKeyQualityPriority(t *testing.T) {
	c := &Cacher{}
	song := structs.Song{Id: 1, Source: structs.SourceQQ}

	standard := c.buildKey(song, service.Standard, "mp3")
	lossless := c.buildKey(song, service.Lossless, "flac")

	if standard == lossless {
		t.Error("不同音质应产生不同缓存键")
	}
	if standard != "qq-1-1.mp3" {
		t.Errorf("标准音质键不符: %q", standard)
	}
	if lossless != "qq-1-4.flac" {
		t.Errorf("无损音质键不符: %q", lossless)
	}
}

// TestEffectiveSource 校验渠道标识归一化。
func TestEffectiveSource(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"", structs.SourceNetease},
		{structs.SourceNetease, structs.SourceNetease},
		{structs.SourceQQ, structs.SourceQQ},
	}
	for _, tc := range cases {
		s := structs.Song{Source: tc.source}
		if got := s.EffectiveSource(); got != tc.want {
			t.Errorf("EffectiveSource(%q) = %q, want %q", tc.source, got, tc.want)
		}
	}
}

// TestIsQQ 校验 QQ 渠道判定。
func TestIsQQ(t *testing.T) {
	if (structs.Song{}).IsQQ() {
		t.Error("空 Source 不应判定为 QQ")
	}
	if !(structs.Song{Source: structs.SourceQQ}).IsQQ() {
		t.Error("SourceQQ 应判定为 QQ")
	}
	if (structs.Song{Source: structs.SourceNetease}).IsQQ() {
		t.Error("网易云不应判定为 QQ")
	}
}

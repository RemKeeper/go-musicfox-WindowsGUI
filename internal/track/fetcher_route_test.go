package track

import (
	"context"
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/netease-music/service"
)

// qqFetcherStub 记录 QQ 渠道被调用的参数，用于验证路由。
type qqFetcherStub struct {
	playableCalls int
	lyricCalls    int
	lastSong      qqmusic.Song
	lastQuality   qqmusic.Quality
	lastLyricMid  string
}

func (s *qqFetcherStub) FetchPlayableInfo(song qqmusic.Song, quality qqmusic.Quality) (*qqmusic.PlayableInfo, error) {
	s.playableCalls++
	s.lastSong = song
	s.lastQuality = quality
	return &qqmusic.PlayableInfo{
		URL:       "https://example.com/qq.mp3",
		MusicType: "mp3",
		Size:      1234,
		Quality:   quality,
	}, nil
}

func (s *qqFetcherStub) FetchLyric(songMid string) (structs.LRCData, error) {
	s.lyricCalls++
	s.lastLyricMid = songMid
	return structs.LRCData{
		Original: "[00:00.00]qq lyric",
		Yrc:      "[0,1000]qq(0,500)",
	}, nil
}

// TestFetcherRoutesQQSongToQQChannel 是渠道路由的核心测试：
// Source=qq 的歌曲必须走 QQ 渠道，且 mid / media_mid 被正确传递。
func TestFetcherRoutesQQSongToQQChannel(t *testing.T) {
	stub := &qqFetcherStub{}
	f := NewFetcher(
		WithFetcherSongQuality(service.Lossless),
		withQQMusicFetcher(stub),
	)

	song := structs.Song{
		Id:       12345,
		Name:     "QQ 歌曲",
		Source:   structs.SourceQQ,
		Mid:      "003abc",
		MediaMid: "001xyz",
	}

	info, err := f.FetchPlayableInfo(context.Background(), song)
	if err != nil {
		t.Fatalf("FetchPlayableInfo 失败: %v", err)
	}
	if stub.playableCalls != 1 {
		t.Errorf("QQ 渠道应被调用 1 次，实际 %d", stub.playableCalls)
	}
	if stub.lastSong.Mid != "003abc" {
		t.Errorf("mid 未正确传递: %q", stub.lastSong.Mid)
	}
	if stub.lastSong.File.MediaMid != "001xyz" {
		t.Errorf("media_mid 未正确传递: %q", stub.lastSong.File.MediaMid)
	}
	if stub.lastQuality != qqmusic.QualitySQ {
		t.Errorf("lossless 应映射为 QualitySQ，实际 %v", stub.lastQuality)
	}
	if info.URL != "https://example.com/qq.mp3" {
		t.Errorf("URL 未正确返回: %q", info.URL)
	}
	if info.MusicType != "mp3" {
		t.Errorf("MusicType 未正确返回: %q", info.MusicType)
	}
}

// TestFetcherRoutesQQLyric 校验 QQ 歌词走 QQ 渠道且传 mid。
func TestFetcherRoutesQQLyric(t *testing.T) {
	stub := &qqFetcherStub{}
	f := NewFetcher(withQQMusicFetcher(stub))

	song := structs.Song{
		Id:     12345,
		Source: structs.SourceQQ,
		Mid:    "003abc",
	}

	data, err := f.FetchLyric(context.Background(), song)
	if err != nil {
		t.Fatalf("FetchLyric 失败: %v", err)
	}
	if stub.lyricCalls != 1 {
		t.Errorf("QQ 渠道应被调用 1 次，实际 %d", stub.lyricCalls)
	}
	if stub.lastLyricMid != "003abc" {
		t.Errorf("歌词请求的 mid 不正确: %q", stub.lastLyricMid)
	}
	if data.Original == "" {
		t.Error("歌词原文为空")
	}
	if data.Yrc == "" {
		t.Error("逐字歌词为空")
	}
}

// TestFetcherQQWithoutMid 校验缺少 mid 时返回明确错误而非静默失败。
func TestFetcherQQWithoutMid(t *testing.T) {
	stub := &qqFetcherStub{}
	f := NewFetcher(withQQMusicFetcher(stub))

	song := structs.Song{Id: 1, Source: structs.SourceQQ} // 无 Mid
	if _, err := f.FetchPlayableInfo(context.Background(), song); err == nil {
		t.Error("缺少 mid 时应报错")
	}
	if stub.playableCalls != 0 {
		t.Error("缺少 mid 时不应调用 QQ 渠道")
	}
}

// TestFetcherQQWithoutChannel 校验未注入 QQ 渠道时返回明确错误。
func TestFetcherQQWithoutChannel(t *testing.T) {
	f := NewFetcher() // 未注入

	song := structs.Song{Id: 1, Source: structs.SourceQQ, Mid: "003abc"}
	if _, err := f.FetchPlayableInfo(context.Background(), song); err == nil {
		t.Error("未注入 QQ 渠道时应报错")
	}
}

// TestQQQualityMapping 校验网易云音质到 QQ 音质的映射。
//
// 设计意图：只要用户设置不低于 HQ，就优先请求无损（SQ）。
// 实测「我喜欢」里 10 首有 9 首 F000 无损可免费播放，
// 而 QQ 侧自带降级链会在不可用时自动回退。
func TestQQQualityMapping(t *testing.T) {
	cases := []struct {
		in   service.SongQualityLevel
		want qqmusic.Quality
	}{
		{service.Standard, qqmusic.QualityStandard},
		// Higher 及以上都先试无损
		{service.Higher, qqmusic.QualitySQ},
		{service.Exhigh, qqmusic.QualitySQ},
		{service.Lossless, qqmusic.QualitySQ},
		{service.Hires, qqmusic.QualitySQ},
	}
	for _, tc := range cases {
		if got := qqQualityFrom(tc.in); got != tc.want {
			t.Errorf("qqQualityFrom(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestQQQualityAlwaysTriesLossless 校验非 Standard 设置都会尝试无损。
//
// 这是「自动拉取无损」的核心保证：用户不需要单独配置，
// 默认的 higher 就会拿到 FLAC（若账号有权限）。
func TestQQQualityAlwaysTriesLossless(t *testing.T) {
	for _, level := range []service.SongQualityLevel{
		service.Higher, service.Exhigh, service.Lossless, service.Hires,
	} {
		if got := qqQualityFrom(level); got != qqmusic.QualitySQ {
			t.Errorf("qqQualityFrom(%v) = %v, 应优先尝试无损 SQ", level, got)
		}
	}
	// Standard 保持标准音质，不强行拉无损
	if got := qqQualityFrom(service.Standard); got != qqmusic.QualityStandard {
		t.Errorf("qqQualityFrom(Standard) = %v, want Standard", got)
	}
}

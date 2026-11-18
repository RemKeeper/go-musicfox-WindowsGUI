package track

import (
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/netease-music/service"
)

// TestQQQualityCoversAllLevels 校验所有网易云音质档位都有明确映射。
//
// 历史 bug：jyeffect / sky / jymaster 三档未处理，落到 default 分支
// 只请求标准音质（M500），表现为「几首歌拉取的都是标准」。
// 用户配置 songLevel = "jymaster" 时正是命中这个缺陷。
func TestQQQualityCoversAllLevels(t *testing.T) {
	cases := []struct {
		level service.SongQualityLevel
		want  qqmusic.Quality
		desc  string
	}{
		{service.Standard, qqmusic.QualityStandard, "标准"},
		{service.Higher, qqmusic.QualitySQ, "较高 → 无损"},
		{service.Exhigh, qqmusic.QualitySQ, "极高 → 无损"},
		{service.Lossless, qqmusic.QualitySQ, "无损"},
		{service.Hires, qqmusic.QualitySQ, "Hi-Res → 无损"},
		{service.JYEffect, qqmusic.QualitySQ, "高清环绕 → 无损"},
		{service.Sky, qqmusic.QualitySQ, "沉浸环绕 → 无损"},
		{service.JYMaster, qqmusic.QualityMaster, "超清母带 → 母带"},
	}

	for _, tc := range cases {
		if got := qqQualityFrom(tc.level); got != tc.want {
			t.Errorf("qqQualityFrom(%v) = %v, want %v（%s）",
				tc.level, got, tc.want, tc.desc)
		}
	}
}

// TestQQQualityNeverSilentlyDowngrades 校验非 Standard 档位不会静默降级。
//
// 核心保证：用户设置的音质越高，请求的档位就越高。
// 任何非 Standard 档位都不应映射成 QualityStandard。
func TestQQQualityNeverSilentlyDowngrades(t *testing.T) {
	levels := []service.SongQualityLevel{
		service.Higher, service.Exhigh, service.Lossless, service.Hires,
		service.JYEffect, service.Sky, service.JYMaster,
	}

	for _, level := range levels {
		got := qqQualityFrom(level)
		if got == qqmusic.QualityStandard {
			t.Errorf("qqQualityFrom(%v) = QualityStandard，"+
				"非 Standard 档位不应静默降级到标准音质", level)
		}
	}
}

// TestQQQualityUnknownLevelDefaultsHigh 校验未知档位按无损处理。
//
// 避免因配置值拼写差异（如手写 "lossless " 带空格）而静默降级。
func TestQQQualityUnknownLevelDefaultsHigh(t *testing.T) {
	unknown := service.SongQualityLevel("some_unknown_level")
	if got := qqQualityFrom(unknown); got != qqmusic.QualitySQ {
		t.Errorf("未知档位应默认为无损 SQ，实际 %v", got)
	}
}

// TestJYMasterRequestsMaster 单独锁定 jymaster 的行为。
//
// 用户实测配置为 jymaster，必须请求母带档位（AI00），
// 母带不可用时由降级链回退到 FLAC。
func TestJYMasterRequestsMaster(t *testing.T) {
	if got := qqQualityFrom(service.JYMaster); got != qqmusic.QualityMaster {
		t.Fatalf("jymaster 应映射为 QualityMaster，实际 %v", got)
	}
}

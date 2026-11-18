package track

import (
	"strings"
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/netease-music/service"
)

// TestQQQualityLabel 校验 QQ 音质标签映射。
//
// 状态栏显示的是「实际拿到的音质」，必须反映降级后的真实结果：
// 请求 SQ 但降级到 ogg 时，应显示 HQ(OGG) 而非「无损」。
func TestQQQualityLabel(t *testing.T) {
	cases := []struct {
		name      string
		quality   qqmusic.Quality
		musicType string
		want      string
	}{
		{"FLAC 无损", qqmusic.QualitySQ, "flac", "无损"},
		{"FLAC 母带", qqmusic.QualityMaster, "flac", "臻品母带"},
		// O801(OGG 640) 实际码率约 450kbps，高于 M800 的 265kbps，
		// 属于 SQ 档，不应标注成有损的「SQ(OGG)」误导用户。
		{"OGG 640", qqmusic.QualityOGG640, "ogg", "SQ"},
		{"OGG 320", qqmusic.QualityOGG320, "ogg", "HQ(OGG)"},
		{"MP3 320", qqmusic.QualityHQ, "mp3", "HQ"},
		{"MP3 128", qqmusic.QualityStandard, "mp3", "标准"},
		// 未知类型时用大写扩展名兜底
		{"未知类型", qqmusic.QualityStandard, "m4a", "M4A"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := qqQualityLabel(tc.quality, tc.musicType); got != tc.want {
				t.Errorf("qqQualityLabel(%v, %q) = %q, want %q",
					tc.quality, tc.musicType, got, tc.want)
			}
		})
	}
}

// TestNeteaseQualityLabel 校验网易云音质标签映射。
func TestNeteaseQualityLabel(t *testing.T) {
	cases := []struct {
		level service.SongQualityLevel
		want  string
	}{
		{service.Standard, "标准"},
		{service.Higher, "较高"},
		{service.Exhigh, "极高"},
		{service.Lossless, "无损"},
		{service.Hires, "Hi-Res"},
	}

	for _, tc := range cases {
		if got := neteaseQualityLabel(tc.level, "mp3"); got != tc.want {
			t.Errorf("neteaseQualityLabel(%v) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

// TestQualityLabelReflectsDowngrade 校验标签反映真实降级结果。
//
// 这是状态栏显示的核心需求：用户请求无损但实际拿到有损编码时，
// 必须如实反映，不能一律显示「无损」。
//
// 注意 O801(OGG 640) 属于 SQ 档（码率约 450kbps，高于 M800 的 265kbps），
// 因此显示「SQ」是准确的，不属于虚假的无损标注。
func TestQualityLabelReflectsDowngrade(t *testing.T) {
	// 请求 FLAC 无损，但实际降级到 OGG 640（仍是 SQ 档）
	got := qqQualityLabel(qqmusic.QualityOGG640, "ogg")
	if got != "SQ" {
		t.Errorf("OGG640 应显示 \"SQ\"，实际 %q", got)
	}

	// 真正降级到 MP3 320 时必须显示 HQ，不能显示无损/SQ
	got = qqQualityLabel(qqmusic.QualityHQ, "mp3")
	if got == "无损" || got == "SQ" {
		t.Errorf("降级到 MP3 320 时应显示 HQ，实际 %q", got)
	}
	if got != "HQ" {
		t.Errorf("MP3 320 应显示 \"HQ\"，实际 %q", got)
	}

	// 降级到 MP3 128 时显示标准
	got = qqQualityLabel(qqmusic.QualityStandard, "mp3")
	if got != "标准" {
		t.Errorf("MP3 128 应显示 \"标准\"，实际 %q", got)
	}
}

// TestOgg640NotLabeledAsLossy 是「显示错误」问题的回归测试。
//
// 用户反馈：《鱼玄机》在 App 显示 HQ，本应用显示 SQ，
// 怀疑是显示错误或拉取异常。实测该曲没有 FLAC（sizeFlac=0），
// 但有 O801（456kbps）和 M800（265kbps），应用优先取 O801 是正确的。
// 原先标签为「SQ(OGG)」，会让用户误以为拿到了劣于 HQ 的有损文件。
func TestOgg640NotLabeledAsLossy(t *testing.T) {
	got := qqQualityLabel(qqmusic.QualityOGG640, "ogg")
	if strings.Contains(got, "OGG") {
		t.Errorf("O801 不应在标签中暴露编码名（实际 %q）："+
			"它的码率高于 M800，属于 SQ 档", got)
	}
	if got != "SQ" {
		t.Errorf("O801 应显示为 \"SQ\"，实际 %q", got)
	}
}

// TestOgg640OutranksMp3320 校验 O801 档位高于 M800。
//
// 实测同一首歌：M800 = 265kbps，O801 = 456kbps。
// 降级链必须把 O801 排在 M800 之前，否则会白白降级到低码率。
// 该断言在 qqmusic 包内由 TestQualityFallbacksOrderPreserved 覆盖，
// 这里只做标签层面的交叉确认。
func TestOgg640OutranksMp3320(t *testing.T) {
	label640 := qqQualityLabel(qqmusic.QualityOGG640, "ogg")
	labelHQ := qqQualityLabel(qqmusic.QualityHQ, "mp3")
	if label640 == labelHQ {
		t.Errorf("O801(%q) 与 M800(%q) 的标签不应相同，"+
			"前者码率更高", label640, labelHQ)
	}
	if label640 != "SQ" {
		t.Errorf("O801 应标为 SQ，实际 %q", label640)
	}
	if labelHQ != "HQ" {
		t.Errorf("M800 应标为 HQ，实际 %q", labelHQ)
	}
}

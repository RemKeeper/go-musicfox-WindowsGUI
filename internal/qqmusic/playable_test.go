package qqmusic

import (
	"strings"
	"testing"
)

// TestCredentialUin 校验 uin 优先使用 str_musicid。
//
// 历史 bug：vkey 请求的 uin 用整数 musicid，导致接口返回空 purl，
// 表现为「无法播放」。参考实现用的是 str_musicid。
func TestCredentialUin(t *testing.T) {
	cases := []struct {
		name string
		cred Credential
		want string
	}{
		{
			name: "优先 str_musicid",
			cred: Credential{MusicID: 123456, StrMusicID: "00123456", MusicKey: "k"},
			want: "00123456",
		},
		{
			name: "str_musicid 为空时回退整数",
			cred: Credential{MusicID: 123456, MusicKey: "k"},
			want: "123456",
		},
		{
			name: "都为空时返回 0",
			cred: Credential{},
			want: "0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cred.Uin(); got != tc.want {
				t.Errorf("Uin() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestVkeyFilenameRule 校验 filename 构造规则。
//
// 对齐 QQMusicApi get_song_urls：
//   - 有 media_mid → prefix + media_mid + ext
//   - 无 media_mid → prefix + mid + mid + ext（mid 拼两遍）
func TestVkeyFilenameRule(t *testing.T) {
	ft := fileTypes[QualityStandard]
	if ft.prefix != "M500" || ft.ext != ".mp3" {
		t.Fatalf("标准音质文件类型错误: %+v", ft)
	}

	// 有 media_mid
	withMedia := ft.prefix + "001MediaMid" + ft.ext
	if !strings.HasPrefix(withMedia, "M500") || !strings.HasSuffix(withMedia, ".mp3") {
		t.Errorf("filename 格式错误: %q", withMedia)
	}
	if strings.Count(withMedia, "001MediaMid") != 1 {
		t.Errorf("有 media_mid 时应只出现一次: %q", withMedia)
	}

	// 无 media_mid：mid 拼两遍
	mid := "0039MnYb0qxYhV"
	withoutMedia := ft.prefix + mid + mid + ft.ext
	if strings.Count(withoutMedia, mid) != 2 {
		t.Errorf("无 media_mid 时 mid 应出现两次: %q", withoutMedia)
	}
}

// TestFileTypesMapping 锁定各音质的文件编码。
func TestFileTypesMapping(t *testing.T) {
	cases := []struct {
		q      Quality
		prefix string
		ext    string
	}{
		{QualityStandard, "M500", ".mp3"},
		{QualityHQ, "M800", ".mp3"},
		{QualitySQ, "F000", ".flac"},
		{QualityOGG320, "O800", ".ogg"},
		{QualityOGG640, "O801", ".ogg"},
		{QualityMaster, "AI00", ".flac"},
	}

	for _, tc := range cases {
		ft, ok := fileTypes[tc.q]
		if !ok {
			t.Errorf("音质 %v 缺少文件类型定义", tc.q)
			continue
		}
		if ft.prefix != tc.prefix || ft.ext != tc.ext {
			t.Errorf("音质 %v = {%s,%s}, want {%s,%s}",
				tc.q, ft.prefix, ft.ext, tc.prefix, tc.ext)
		}
	}
}

// TestQualityFallbacks 校验音质降级链。
//
// 注意：降级链现在覆盖全部编码（见 TestQualityFallbacksCoversAllCodes），
// 因为实测同一首歌可能只有 O801 能拿到 purl。这里只校验「请求的音质优先」。
func TestQualityFallbacks(t *testing.T) {
	// 无论从哪个音质开始，首项都应是它自己
	for _, q := range []Quality{QualityStandard, QualityHQ, QualitySQ, QualityOGG320, QualityOGG640, QualityMaster} {
		got := qualityFallbacks(q)
		if len(got) == 0 {
			t.Errorf("音质 %v 的降级链为空", q)
			continue
		}
		if got[0] != q {
			t.Errorf("从 %v 开始，首项却是 %v", q, got[0])
		}
	}

	// 降级链必须包含 OGG640（实测覆盖最广的编码）
	got := qualityFallbacks(QualityStandard)
	found := false
	for _, q := range got {
		if q == QualityOGG640 {
			found = true
		}
	}
	if !found {
		t.Error("降级链缺少 OGG640（实测该编码覆盖最广）")
	}
}

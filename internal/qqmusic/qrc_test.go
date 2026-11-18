package qqmusic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadQRCFixture 读取真实的 QRC 密文夹具（来自 GetPlayLyricInfo 的 qrc=1 响应）。
func loadQRCFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join("testdata", "qrc_sample.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取夹具 %s 失败: %v", path, err)
	}
	return strings.TrimSpace(string(data))
}

// TestQrcDecrypt 校验 QRC 解密与 QQMusicApi 的 Python 实现一致。
func TestQrcDecrypt(t *testing.T) {
	cipher := loadQRCFixture(t)

	got, err := QrcDecrypt(cipher)
	if err != nil {
		t.Fatalf("QrcDecrypt 失败: %v", err)
	}

	// 解密结果必须是 QRC XML。
	if !strings.HasPrefix(got, "<?xml") {
		t.Errorf("解密结果不是 XML，前 80 字符: %q", got[:min(80, len(got))])
	}
	if !strings.Contains(got, "<QrcInfos>") {
		t.Error("解密结果缺少 <QrcInfos> 根节点")
	}
	if !strings.Contains(got, `LyricContent="`) {
		t.Error("解密结果缺少 LyricContent 属性")
	}

	// 逐字歌词特征：形如 `[0,4503]最(0,450)伟(450,450)`。
	if !strings.Contains(got, "(0,450)") {
		t.Error("解密结果缺少逐字时间元组")
	}
}

// TestQrcDecryptEmpty 校验空输入返回空串而非报错。
func TestQrcDecryptEmpty(t *testing.T) {
	got, err := QrcDecrypt("")
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if got != "" {
		t.Errorf("空输入应返回空串，实际 %q", got)
	}
}

// TestQrcDecryptInvalidHex 校验非法十六进制输入报错。
func TestQrcDecryptInvalidHex(t *testing.T) {
	if _, err := QrcDecrypt("zzzz"); err == nil {
		t.Error("非法十六进制应报错")
	}
}

// TestQrcDecryptUnaligned 校验非 8 字节对齐输入报错。
func TestQrcDecryptUnaligned(t *testing.T) {
	if _, err := QrcDecrypt("B7159F"); err == nil {
		t.Error("非 8 字节对齐应报错")
	}
}

// TestQrcLyricContent 校验从 XML 中提取 LyricContent 并反转义。
func TestQrcLyricContent(t *testing.T) {
	cipher := loadQRCFixture(t)
	decrypted, err := QrcDecrypt(cipher)
	if err != nil {
		t.Fatalf("QrcDecrypt 失败: %v", err)
	}

	content := QrcLyricContent(decrypted)
	if content == "" {
		t.Fatal("未能提取 LyricContent")
	}
	if strings.Contains(content, "&quot;") || strings.Contains(content, "&amp;") {
		t.Error("LyricContent 仍有未反转义的 XML 实体")
	}
	// 必须是 LRC 风格的逐字行。
	if !strings.Contains(content, "[0,4503]") {
		t.Errorf("LyricContent 缺少首行时间标记，前 120 字符: %q", content[:min(120, len(content))])
	}
	if !strings.Contains(content, "[ti:") {
		t.Error("LyricContent 缺少 LRC 元信息头")
	}
}

// TestUnescapeXMLAttr 校验 XML 实体反转义，含 &amp; 必须最后处理。
func TestUnescapeXMLAttr(t *testing.T) {
	cases := []struct{ in, want string }{
		{`a&quot;b`, `a"b`},
		{`a&amp;b`, `a&b`},
		{`a&lt;b&gt;c`, `a<b>c`},
		{`line1&#10;line2`, "line1\nline2"},
		// &amp;quot; 应解成 &quot; 而非 "
		{`a&amp;quot;b`, `a&quot;b`},
		{`plain`, `plain`},
	}
	for _, tc := range cases {
		if got := unescapeXMLAttr(tc.in); got != tc.want {
			t.Errorf("unescapeXMLAttr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestQrcToYRC 校验 QRC 转 YRC 后能被 musicfox 的逐字解析器消费。
func TestQrcToYRC(t *testing.T) {
	cipher := loadQRCFixture(t)
	decrypted, err := QrcDecrypt(cipher)
	if err != nil {
		t.Fatalf("QrcDecrypt 失败: %v", err)
	}

	yrc := qrcToYRC(decrypted)
	if yrc == "" {
		t.Fatal("qrcToYRC 返回空串")
	}
	if strings.Contains(yrc, "<?xml") {
		t.Error("qrcToYRC 应剥离 XML 外壳")
	}
	// 首行必须是 `[start,duration]word(time,dur)...`
	// 首行逐字歌词必须是 `[start,duration]word(time,dur)...` 形式。
	var body string
	for _, line := range strings.Split(yrc, "\n") {
		if strings.HasPrefix(line, "[0,") {
			body = line
			break
		}
	}
	if body == "" {
		t.Fatalf("未找到首行逐字歌词，前 200 字符: %q", yrc[:min(200, len(yrc))])
	}
	if !strings.Contains(body, "(") || !strings.Contains(body, ")") {
		t.Errorf("首行缺少词时间元组: %q", body)
	}
}

// TestAlbumCoverURL 校验封面 URL 拼接规则。
func TestAlbumCoverURL(t *testing.T) {
	cases := []struct {
		mid  string
		size int
		want string
	}{
		{"0042cH172YJ0mz", 300, "https://y.gtimg.cn/music/photo_new/T002R300x300M0000042cH172YJ0mz.jpg"},
		{"0042cH172YJ0mz", 0, "https://y.gtimg.cn/music/photo_new/T002R300x300M0000042cH172YJ0mz.jpg"},
		{"", 300, ""},
	}
	for _, tc := range cases {
		if got := AlbumCoverURL(tc.mid, tc.size); got != tc.want {
			t.Errorf("AlbumCoverURL(%q, %d) = %q, want %q", tc.mid, tc.size, got, tc.want)
		}
	}
}

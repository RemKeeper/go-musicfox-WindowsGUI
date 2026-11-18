package qqmusic

import (
	"strings"
	"testing"
)

// TestSign 校验 zzc 签名与 QQMusicApi 的 Python 实现逐字节一致。
//
// 期望值由 `python scripts/qq_probe.py sign '<payload>'` 生成。
func TestSign(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "simple object",
			payload: `{"a":1}`,
			want:    "zzc7746109xq501htmv4ipz7c8owlxfpihjm1fd695c7",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sign([]byte(tc.payload))
			if got != tc.want {
				t.Fatalf("Sign(%q) = %q, want %q", tc.payload, got, tc.want)
			}
		})
	}
}

// TestSignFormat 校验签名结构：zzc 前缀 + 小写 + 无 base64 特殊字符。
func TestSignFormat(t *testing.T) {
	got := Sign([]byte(`{"module":"music.search.SearchCgiService"}`))

	if !strings.HasPrefix(got, "zzc") {
		t.Errorf("签名缺少 zzc 前缀: %q", got)
	}
	if got != strings.ToLower(got) {
		t.Errorf("签名应为小写: %q", got)
	}
	for _, bad := range []string{`\`, "/", "+", "="} {
		if strings.Contains(got, bad) {
			t.Errorf("签名不应包含 %q: %q", bad, got)
		}
	}
	// zzc(3) + part1(7) + part2(8) = 18 个固定字符，其余为去填充后的 base64。
	if len(got) < 18 {
		t.Errorf("签名长度异常: %d", len(got))
	}
}

// TestSignDeterministic 校验同一输入产生同一签名。
func TestSignDeterministic(t *testing.T) {
	payload := []byte(`{"comm":{"ct":11},"req_0":{"module":"m","method":"x"}}`)
	first := Sign(payload)
	for i := 0; i < 5; i++ {
		if got := Sign(payload); got != first {
			t.Fatalf("签名不稳定: %q != %q", got, first)
		}
	}
}

// TestGTK 校验 g_tk 计算（hash33，初值 5381）。
func TestGTK(t *testing.T) {
	if got := gTK(""); got != 5381 {
		t.Errorf("空 key 的 g_tk = %d, want 5381", got)
	}
	// 非空 key 必须落在 31 位正整数范围。
	got := gTK("Q_H_L_abcdefg")
	if got < 0 || got > 0x7FFFFFFF {
		t.Errorf("g_tk 越界: %d", got)
	}
}

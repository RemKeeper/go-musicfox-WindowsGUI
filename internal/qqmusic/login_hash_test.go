package qqmusic

import "testing"

// TestPtqrToken 锁定 ptqrtoken 的 Hash33 算法（初值 0）。
//
// 这是扫码登录的致命点：初值用错（5381）会让服务端返回 403，
// 表现为「一直等待扫码」，且不会报错。
// 期望值由 QQMusicApi 的 hash33(s, h=0) 生成。
func TestPtqrToken(t *testing.T) {
	cases := []struct {
		qrsig string
		want  int
	}{
		{"058947800461101f2d34a957fc4450", 232837829},
		{"abc", 108966},
		{"", 0},
		{
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" +
				"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			936479760,
		},
	}

	for _, tc := range cases {
		if got := ptqrToken(tc.qrsig); got != tc.want {
			t.Errorf("ptqrToken(%q) = %d, want %d", tc.qrsig, got, tc.want)
		}
	}
}

// TestGtkFromSkey 锁定 g_tk 的 Hash33 算法（初值 5381）。
//
// 期望值由 QQMusicApi 的 hash33(s, h=5381) 生成。
func TestGtkFromSkey(t *testing.T) {
	cases := []struct {
		skey string
		want int
	}{
		{"abcdef", 1900596090},
		{"p_skey_sample_123", 147902918},
	}

	for _, tc := range cases {
		if got := gtkFromSkey(tc.skey); got != tc.want {
			t.Errorf("gtkFromSkey(%q) = %d, want %d", tc.skey, got, tc.want)
		}
	}
}

// TestPtqrTokenDiffersFromGtk 确保两个用途的初值没有被混用。
//
// 历史上两者共用一个 hash33（初值 5381），导致扫码轮询 403。
func TestPtqrTokenDiffersFromGtk(t *testing.T) {
	const s = "058947800461101f2d34a957fc4450"

	token := ptqrToken(s)
	gtk := gtkFromSkey(s)

	if token == gtk {
		t.Fatalf("ptqrtoken 与 g_tk 不应相同（说明初值被混用）: 都是 %d", token)
	}
	// ptqrtoken 必须是初值 0 的结果
	if token != hash33(s, 0) {
		t.Errorf("ptqrToken 未使用初值 0")
	}
	if gtk != hash33(s, 5381) {
		t.Errorf("gtkFromSkey 未使用初值 5381")
	}
}

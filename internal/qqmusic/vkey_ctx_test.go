package qqmusic

import (
	"testing"
)

// TestVkeyContextsCovered 锁定 vkey 的 ctx 尝试序列。
//
// 这是《告白》无法播放的根因：ctx=0 时该歌曲所有编码都返回 104003，
// 但 ctx=1/2 能正常拿到 purl 并下载到完整音频。
// 官方客户端按播放场景传不同 ctx，因此必须逐个尝试。
func TestVkeyContextsCovered(t *testing.T) {
	if len(vkeyContexts) < 2 {
		t.Fatalf("vkeyContexts 只包含 %d 个值，至少需要尝试 ctx=0 与 ctx=1", len(vkeyContexts))
	}
	// 必须从 0 开始（默认场景优先）
	if vkeyContexts[0] != 0 {
		t.Errorf("vkeyContexts[0] = %d, want 0（默认场景应优先）", vkeyContexts[0])
	}
	// 必须包含 1（实测《告白》靠它才可播）
	found := false
	for _, ctx := range vkeyContexts {
		if ctx == 1 {
			found = true
		}
	}
	if !found {
		t.Error("vkeyContexts 缺少 ctx=1（实测《告白》需要它）")
	}
}

// TestVkeyContextsNoDuplicate 校验 ctx 序列无重复，避免多余请求。
func TestVkeyContextsNoDuplicate(t *testing.T) {
	seen := make(map[int]bool, len(vkeyContexts))
	for _, ctx := range vkeyContexts {
		if seen[ctx] {
			t.Errorf("vkeyContexts 中 ctx=%d 重复出现", ctx)
		}
		seen[ctx] = true
	}
}

// TestQualityFallbacksOrderPreserved 校验降级链仍以请求音质优先。
//
// 加入 ctx 循环后，音质优先级不能被打乱：用户请求标准音质时
// 不应先尝试母带。
func TestQualityFallbacksOrderPreserved(t *testing.T) {
	for _, q := range []Quality{QualityStandard, QualityHQ, QualitySQ} {
		chain := qualityFallbacks(q)
		if len(chain) == 0 {
			t.Fatalf("音质 %v 的降级链为空", q)
		}
		if chain[0] != q {
			t.Errorf("从 %v 开始，首项却是 %v", q, chain[0])
		}
	}
}

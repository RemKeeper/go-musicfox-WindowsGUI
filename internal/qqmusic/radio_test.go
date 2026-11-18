package qqmusic

import (
	"testing"
)

// TestGuessRecommendReturnsSongs 校验「猜你喜欢」电台能返回歌曲。
//
// 这是 QQ 音乐侧对应网易云「心动模式」的接口：
// music.radioProxy.MbTrackRadioSvr / get_radio_track，固定 id=99。
func TestGuessRecommendReturnsSongs(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	songs, err := c.GetGuessRecommend(5)
	if err != nil {
		t.Fatalf("获取猜你喜欢失败: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("猜你喜欢返回空列表")
	}

	for _, s := range songs {
		if s.Mid == "" {
			t.Errorf("歌曲 %q 缺少 mid，无法播放", s.Name)
		}
		if s.Name == "" {
			t.Errorf("歌曲 mid=%s 缺少名称", s.Mid)
		}
	}
}

// TestGuessRecommendIsRandom 校验连续调用返回不同歌曲。
//
// 心动模式的核心特性：每次推送都是新歌，不能重复返回同一批。
// 实测服务端按听歌画像实时挑选，连续两次结果不重叠。
func TestGuessRecommendIsRandom(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	first, err := c.GetGuessRecommend(5)
	if err != nil {
		t.Fatalf("第一次获取失败: %v", err)
	}
	second, err := c.GetGuessRecommend(5)
	if err != nil {
		t.Fatalf("第二次获取失败: %v", err)
	}

	if len(first) == 0 || len(second) == 0 {
		t.Fatal("返回空列表，无法比较随机性")
	}

	// 统计重叠
	firstMids := make(map[string]bool, len(first))
	for _, s := range first {
		firstMids[s.Mid] = true
	}
	overlap := 0
	for _, s := range second {
		if firstMids[s.Mid] {
			overlap++
		}
	}

	// 允许偶尔重复，但不应完全一致
	if overlap == len(second) && len(second) > 1 {
		t.Errorf("两次返回完全相同的 %d 首歌，心动模式应返回不同推荐", overlap)
	}
}

// TestGuessRecommendCapsAtFive 校验单次请求上限。
//
// 服务端单次最多返回 5 首，传更大值也只返回 5 首，
// 客户端应主动裁剪以避免无意义的大请求。
func TestGuessRecommendCapsAtFive(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	songs, err := c.GetGuessRecommend(100)
	if err != nil {
		t.Fatalf("获取失败: %v", err)
	}
	if len(songs) > 5 {
		t.Errorf("单次返回 %d 首，服务端上限为 5 首", len(songs))
	}
}

// TestRadarRecommendPaginates 校验雷达推荐的分页行为。
//
// 雷达推荐与猜你喜欢互补：它按页稳定返回，适合「查看更多」。
func TestRadarRecommendPaginates(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	page1, hasMore, err := c.GetRadarRecommend(1)
	if err != nil {
		t.Fatalf("第 1 页失败: %v", err)
	}
	if len(page1) == 0 {
		t.Fatal("第 1 页为空")
	}
	if !hasMore {
		t.Log("注意：第 1 页 hasMore=false，可能已到末页")
	}

	page2, _, err := c.GetRadarRecommend(2)
	if err != nil {
		t.Fatalf("第 2 页失败: %v", err)
	}
	if len(page2) == 0 {
		t.Fatal("第 2 页为空")
	}

	// 两页不应完全相同
	p1 := make(map[string]bool, len(page1))
	for _, s := range page1 {
		p1[s.Mid] = true
	}
	same := 0
	for _, s := range page2 {
		if p1[s.Mid] {
			same++
		}
	}
	if same == len(page2) {
		t.Error("第 1 页与第 2 页完全相同，分页未生效")
	}
}

// TestGuessRecommendSongsPlayable 校验推荐歌曲带有播放所需字段。
func TestGuessRecommendSongsPlayable(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	songs, err := c.GetGuessRecommend(3)
	if err != nil {
		t.Fatalf("获取失败: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("返回空列表")
	}

	for _, s := range songs {
		if s.Mid == "" {
			t.Errorf("歌曲 %q 缺少 mid", s.Name)
			continue
		}
		if s.File.MediaMid == "" {
			t.Logf("注意：歌曲 %q 无 media_mid，将回退用 mid 拼 filename", s.Name)
		}
	}
}

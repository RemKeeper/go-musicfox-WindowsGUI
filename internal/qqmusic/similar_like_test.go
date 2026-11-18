package qqmusic

import (
	"testing"
)

// TestSimilarSongsReturnsSongs 校验相似歌曲推荐（心动模式）能返回数据。
//
// 这是 QQ 侧对应网易云「心动模式」的接口：
// music.recommend.TrackRelationServer / GetSimilarSongs。
// 以某首歌为种子，返回同风格歌曲。
func TestSimilarSongsReturnsSongs(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	// 用「我喜欢」里的第一首作种子
	favs, _, err := c.GetFavSongs(c.Credential().MusicID, 1, 1)
	if err != nil || len(favs) == 0 {
		t.Skipf("无法获取种子歌曲: %v", err)
	}
	seed := favs[0]

	similar, err := c.GetSimilarSongs(seed.ID)
	if err != nil {
		t.Fatalf("获取相似歌曲失败: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("相似歌曲为空")
	}

	for _, s := range similar {
		if s.Mid == "" {
			t.Errorf("相似歌曲 %q 缺少 mid，无法播放", s.Name)
		}
		if s.Name == "" {
			t.Errorf("相似歌曲 mid=%s 缺少名称", s.Mid)
		}
	}
}

// TestSimilarSongsRejectsInvalidID 校验无效 ID 被拒绝。
func TestSimilarSongsRejectsInvalidID(t *testing.T) {
	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	if _, err := c.GetSimilarSongs(0); err == nil {
		t.Error("songID=0 应返回错误")
	}
}

// TestSimilarSongsPlayable 校验相似歌曲带有播放所需字段。
func TestSimilarSongsPlayable(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	favs, _, err := c.GetFavSongs(c.Credential().MusicID, 1, 1)
	if err != nil || len(favs) == 0 {
		t.Skipf("无法获取种子歌曲: %v", err)
	}

	similar, err := c.GetSimilarSongs(favs[0].ID)
	if err != nil {
		t.Fatalf("获取失败: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("相似歌曲为空")
	}

	// 至少验证前 3 首能拿到播放链接
	ok := 0
	for i, s := range similar {
		if i >= 3 {
			break
		}
		if _, err := c.FetchPlayableInfo(s, QualitySQ); err != nil {
			t.Logf("注意：相似歌曲 %q 无法播放: %v", s.Name, err)
			continue
		}
		ok++
	}
	if ok == 0 {
		t.Error("前 3 首相似歌曲都无法播放")
	}
}

// TestLikeSongWorks 校验收藏/取消收藏可用。
//
// 历史 bug：写接口未标记 Android 平台，导致 code=1000。
func TestLikeSongWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	favs, _, err := c.GetFavSongs(c.Credential().MusicID, 1, 1)
	if err != nil || len(favs) == 0 {
		t.Skipf("无法获取测试歌曲: %v", err)
	}
	s := favs[0]

	// 先取消再重新收藏，验证两个方向都可用
	if err := c.UnlikeSong(s.ID, s.Type); err != nil {
		t.Fatalf("取消收藏失败: %v", err)
	}
	if err := c.LikeSong(s.ID, s.Type); err != nil {
		t.Fatalf("收藏失败: %v", err)
	}
}

// TestLikeSongRequiresLogin 校验未登录时收藏返回明确错误。
func TestLikeSongRequiresLogin(t *testing.T) {
	c := NewClient()
	// 不设置凭证

	if err := c.LikeSong(123, 0); err == nil {
		t.Error("未登录时应返回错误")
	}
}

// TestLikeSongRejectsInvalidID 校验无效歌曲 ID 被拒绝。
func TestLikeSongRejectsInvalidID(t *testing.T) {
	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))

	if err := c.LikeSong(0, 0); err == nil {
		t.Error("songID=0 应返回错误")
	}
}

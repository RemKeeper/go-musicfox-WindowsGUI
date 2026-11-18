package qqmusic

import (
	"testing"
)

// TestRecommendPlaylistIDUsesTid 锁定推荐歌单的 ID 字段选择。
//
// 历史 bug（用户反馈「打开推荐歌单报错」）：
// GetRecommendFeed 返回的 basic 里有两个 ID 字段：
//   - dirid：推荐位内部目录号（如 92）
//   - tid：真正的歌单 ID（如 9082901147）
//
// 代码误用 dirid 填 Playlist.ID，导致进入详情时 GetSonglistDetail(92)
// 静默返回空列表（不报错，但 0 首歌），UI 表现为「无法获取」。
//
// 本测试用真实接口校验：推荐列表返回的 ID 必须能拉到详情。
func TestRecommendPlaylistIDUsesTid(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	cred := loadCredentialFromDB(t)
	c.SetCredential(cred)

	lists, err := c.GetRecommendSonglists(1, 5)
	if err != nil {
		t.Fatalf("获取推荐歌单失败: %v", err)
	}
	if len(lists) == 0 {
		t.Fatal("推荐歌单为空")
	}

	for _, p := range lists {
		if p.ID == 0 {
			t.Errorf("歌单 %q 的 ID 为 0（应使用 tid）", p.Title)
			continue
		}
		// tid 是 10 位左右的雪花 ID，dirid 通常是个位数到三位数
		if p.ID < 1000 {
			t.Errorf("歌单 %q 的 ID = %d，看起来是 dirid 而非 tid", p.Title, p.ID)
		}
	}
}

// TestRecommendPlaylistDetailNotEmpty 校验推荐歌单能拉到歌曲。
//
// 这是用户可见行为的直接验证：点开推荐歌单必须能看到歌。
func TestRecommendPlaylistDetailNotEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	cred := loadCredentialFromDB(t)
	c.SetCredential(cred)

	lists, err := c.GetRecommendSonglists(1, 3)
	if err != nil {
		t.Fatalf("获取推荐歌单失败: %v", err)
	}
	if len(lists) == 0 {
		t.Fatal("推荐歌单为空")
	}

	for _, p := range lists {
		detail, err := c.GetSonglistDetail(p.ID, 1, 5)
		if err != nil {
			t.Errorf("歌单 %q (id=%d) 详情拉取失败: %v", p.Title, p.ID, err)
			continue
		}
		if detail == nil || len(detail.Songs) == 0 {
			t.Errorf("歌单 %q (id=%d) 详情为空，说明 ID 用错了", p.Title, p.ID)
			continue
		}
		if detail.Songs[0].Name == "" {
			t.Errorf("歌单 %q 首曲名称为空", p.Title)
		}
	}
}

// TestRecommendPlaylistFieldsPopulated 校验推荐歌单的展示字段完整。
func TestRecommendPlaylistFieldsPopulated(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	cred := loadCredentialFromDB(t)
	c.SetCredential(cred)

	lists, err := c.GetRecommendSonglists(1, 5)
	if err != nil {
		t.Fatalf("获取推荐歌单失败: %v", err)
	}

	for i, p := range lists {
		if p.Title == "" {
			t.Errorf("第 %d 条歌单标题为空", i)
		}
		if p.SongNum == 0 {
			t.Errorf("歌单 %q 的歌曲数为 0", p.Title)
		}
		// 封面与创建者昵称可能个别缺失，只统计不强制失败
		if p.PicURL == "" {
			t.Logf("注意：歌单 %q 无封面", p.Title)
		}
	}
}

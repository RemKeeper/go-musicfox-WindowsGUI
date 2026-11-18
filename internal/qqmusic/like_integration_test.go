package qqmusic

import (
	"errors"
	"os"
	"testing"
)

// 收藏链路的真实接口测试。
//
// 需要登录态：设置环境变量 QQMUSIC_MUSICID 与 QQMUSIC_MUSICKEY 后运行；
// 未设置时自动跳过，不影响 CI。

// loginClientForTest 从环境变量构造已登录客户端。
func loginClientForTest(t *testing.T) *Client {
	t.Helper()

	musicID := os.Getenv("QQMUSIC_MUSICID")
	musicKey := os.Getenv("QQMUSIC_MUSICKEY")
	if musicID == "" || musicKey == "" {
		t.Skip("未设置 QQMUSIC_MUSICID / QQMUSIC_MUSICKEY，跳过需要登录的测试")
	}

	var id int64
	if _, err := fmtSscan(musicID, &id); err != nil {
		t.Fatalf("QQMUSIC_MUSICID 无效: %v", err)
	}

	c := NewClient()
	c.SetCredential(Credential{MusicID: id, MusicKey: musicKey})
	if !c.LoggedIn() {
		t.Fatal("凭证未生效")
	}
	return c
}

// fmtSscan 是 fmt.Sscanf 的薄封装，避免测试文件引入 fmt。
func fmtSscan(s string, out *int64) (int, error) {
	var v int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errors.New("非数字")
		}
		v = v*10 + int64(ch-'0')
	}
	*out = v
	return 1, nil
}

// TestIntegrationFavSongsReadable 校验登录后能读取「我喜欢」。
func TestIntegrationFavSongsReadable(t *testing.T) {
	c := loginClientForTest(t)
	cred := c.Credential()

	songs, total, err := c.GetFavSongs(cred.MusicID, 1, 10)
	if err != nil {
		t.Fatalf("GetFavSongs 失败: %v", err)
	}
	t.Logf("我喜欢共 %d 首，本页 %d 首", total, len(songs))

	for _, s := range songs[:min(3, len(songs))] {
		if s.ID == 0 || s.Mid == "" {
			t.Errorf("收藏歌曲缺少 id/mid: %+v", s)
		}
		t.Logf("  %s - %s (id=%d, type=%d)", s.Name, s.SingerName(), s.ID, s.Type)
	}
}

// TestIntegrationLikeRoundTrip 校验收藏 → 读取可见 → 取消收藏的完整往返。
//
// 这是「我喜欢」对齐的核心验证：写入必须能被读取接口观察到。
func TestIntegrationLikeRoundTrip(t *testing.T) {
	c := loginClientForTest(t)
	cred := c.Credential()

	// 取一首「我喜欢」里已有的歌做取消/恢复，避免污染用户数据。
	songs, total, err := c.GetFavSongs(cred.MusicID, 1, 1)
	if err != nil {
		t.Fatalf("GetFavSongs 失败: %v", err)
	}
	if total == 0 || len(songs) == 0 {
		t.Skip("「我喜欢」为空，跳过往返测试")
	}

	target := songs[0]
	t.Logf("测试目标: %s (id=%d, type=%d)", target.Name, target.ID, target.Type)

	// 1. 取消收藏
	if err := c.UnlikeSong(target.ID, target.Type); err != nil {
		t.Fatalf("UnlikeSong 失败: %v", err)
	}

	// 2. 确认已不在列表
	if containsSongID(t, c, cred.MusicID, target.ID) {
		t.Error("取消收藏后歌曲仍在「我喜欢」中")
	}

	// 3. 恢复收藏
	if err := c.LikeSong(target.ID, target.Type); err != nil {
		t.Fatalf("LikeSong 失败: %v", err)
	}

	// 4. 确认已回到列表
	if !containsSongID(t, c, cred.MusicID, target.ID) {
		t.Error("重新收藏后歌曲未出现在「我喜欢」中")
	}
	t.Log("收藏往返验证通过")
}

// containsSongID 在前几页「我喜欢」中查找指定歌曲。
func containsSongID(t *testing.T, c *Client, musicID, songID int64) bool {
	t.Helper()
	for page := 1; page <= 3; page++ {
		songs, total, err := c.GetFavSongs(musicID, page, 100)
		if err != nil {
			t.Fatalf("GetFavSongs 失败: %v", err)
		}
		for _, s := range songs {
			if s.ID == songID {
				return true
			}
		}
		if len(songs) == 0 || page*100 >= total {
			break
		}
	}
	return false
}

// SingerName 返回首位歌手名，便于日志阅读。
func (s Song) SingerName() string {
	if len(s.Singer) == 0 {
		return ""
	}
	return s.Singer[0].Name
}

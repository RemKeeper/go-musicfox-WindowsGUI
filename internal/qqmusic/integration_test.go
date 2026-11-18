package qqmusic

import (
	"testing"
)

// 集成测试：直接调用真实 QQ 音乐接口，验证 API 层装配正确。
//
// 这些测试依赖外网，且搜索接口有服务端风控（Client.Do 已内置换指纹重试）。
// 使用 `go test -short` 可跳过。

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("跳过需要外网的集成测试")
	}
}

// TestIntegrationSearch 校验搜索接口返回可用歌曲。
func TestIntegrationSearch(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	result, err := c.Search("love", SearchSong, 1, 5)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(result.Songs) == 0 {
		t.Fatal("搜索结果为空")
	}

	first := result.Songs[0]
	if first.ID == 0 || first.Mid == "" {
		t.Errorf("歌曲缺少 id/mid: %+v", first)
	}
	if first.Name == "" {
		t.Error("歌曲缺少名称")
	}
	if first.Interval <= 0 {
		t.Errorf("歌曲时长异常: %d", first.Interval)
	}
	if len(first.Singer) == 0 {
		t.Error("歌曲缺少歌手")
	}
	if first.Album.Mid == "" {
		t.Error("歌曲缺少专辑 mid")
	}
	if first.File.MediaMid == "" {
		t.Error("歌曲缺少 media_mid（播放链接必需）")
	}

	t.Logf("首条结果: id=%d mid=%s name=%q singer=%q albumMid=%s mediaMid=%s interval=%ds",
		first.ID, first.Mid, first.Name, first.Singer[0].Name,
		first.Album.Mid, first.File.MediaMid, first.Interval)
}

// TestIntegrationQuerySong 校验歌曲详情接口。
func TestIntegrationQuerySong(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	songs, err := c.QuerySong([]string{"003w2xz20QlUZt"})
	if err != nil {
		t.Fatalf("QuerySong 失败: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("查询结果为空")
	}

	s := songs[0]
	if s.Mid != "003w2xz20QlUZt" {
		t.Errorf("mid 不匹配: %s", s.Mid)
	}
	if s.Name == "" {
		t.Error("歌曲缺少名称")
	}
	if s.Duration().Seconds() <= 0 {
		t.Errorf("时长异常: %v", s.Duration())
	}
	if got := s.CoverURL(300); got == "" {
		t.Error("封面 URL 为空")
	}

	t.Logf("%s - %s, %v, cover=%s", s.Name, s.Singer[0].Name, s.Duration(), s.CoverURL(300))
}

// TestIntegrationSonglist 校验歌单详情接口。
func TestIntegrationSonglist(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	detail, err := c.GetSonglistDetail(7011264340, 1, 5)
	if err != nil {
		t.Fatalf("GetSonglistDetail 失败: %v", err)
	}
	if detail.Info.Title == "" {
		t.Error("歌单缺少标题")
	}
	if len(detail.Songs) == 0 {
		t.Fatal("歌单歌曲为空")
	}
	if detail.Total <= 0 {
		t.Errorf("歌单总数异常: %d", detail.Total)
	}

	t.Logf("歌单 %q, 共 %d 首, 本页 %d 首, hasMore=%v",
		detail.Info.Title, detail.Total, len(detail.Songs), detail.HasMore)
}

// TestIntegrationTop 校验榜单接口。
func TestIntegrationTop(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	categories, err := c.GetTopCategories()
	if err != nil {
		t.Fatalf("GetTopCategories 失败: %v", err)
	}
	if len(categories) == 0 {
		t.Fatal("榜单分类为空")
	}

	// 找到第一个有榜单的分组
	var topID int64
	for _, cat := range categories {
		if len(cat.Toplist) > 0 {
			topID = cat.Toplist[0].TopID
			break
		}
	}
	if topID == 0 {
		t.Fatal("未找到任何榜单")
	}

	detail, err := c.GetTopDetail(topID, 1, 5)
	if err != nil {
		t.Fatalf("GetTopDetail 失败: %v", err)
	}
	if len(detail.Songs) == 0 {
		t.Fatal("榜单歌曲为空")
	}

	t.Logf("%d 个分组，榜单 %q 共 %d 首，本页 %d 首",
		len(categories), detail.Info.Title, detail.Info.TotalNum, len(detail.Songs))
}

// TestIntegrationAlbum 校验专辑接口。
func TestIntegrationAlbum(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	songs, total, err := c.GetAlbumSongs("0042cH172YJ0mz", 1, 5)
	if err != nil {
		t.Fatalf("GetAlbumSongs 失败: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("专辑歌曲为空")
	}
	if total <= 0 {
		t.Errorf("专辑总数异常: %d", total)
	}

	t.Logf("专辑共 %d 首，本页 %d 首，首曲 %q", total, len(songs), songs[0].Name)
}

// TestIntegrationSinger 校验歌手接口。
func TestIntegrationSinger(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	songs, total, err := c.GetSingerSongs("0025NhlN2yWrP4", 1, 5)
	if err != nil {
		t.Fatalf("GetSingerSongs 失败: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("歌手歌曲为空")
	}
	if total <= 0 {
		t.Errorf("歌手歌曲总数异常: %d", total)
	}

	t.Logf("歌手共 %d 首，本页 %d 首，首曲 %q", total, len(songs), songs[0].Name)
}

// TestIntegrationLyric 校验歌词接口（LRC + QRC）。
func TestIntegrationLyric(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	data, err := c.FetchLyric("003w2xz20QlUZt")
	if err != nil {
		t.Fatalf("FetchLyric 失败: %v", err)
	}
	if data.Original == "" {
		t.Error("LRC 原文为空")
	}
	if data.Yrc == "" {
		t.Error("QRC 逐字歌词为空")
	}

	t.Logf("LRC %d 字符，QRC %d 字符", len(data.Original), len(data.Yrc))
}

// TestIntegrationCdnDispatch 校验 CDN 调度接口。
func TestIntegrationCdnDispatch(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	sips, err := c.GetCdnDispatch()
	if err != nil {
		t.Fatalf("GetCdnDispatch 失败: %v", err)
	}
	if len(sips) == 0 {
		t.Fatal("CDN 列表为空")
	}

	t.Logf("获取到 %d 个 CDN 域名，首个: %s", len(sips), sips[0])
}

// TestIntegrationPlayableRequiresLogin 校验未登录时播放链接返回 ErrNotLoggedIn。
func TestIntegrationPlayableRequiresLogin(t *testing.T) {
	skipIfShort(t)
	c := NewClient()

	song := Song{Mid: "003w2xz20QlUZt"}
	if _, err := c.FetchPlayableInfo(song, QualitySQ); err != ErrNotLoggedIn {
		t.Errorf("未登录应返回 ErrNotLoggedIn，实际: %v", err)
	}
}

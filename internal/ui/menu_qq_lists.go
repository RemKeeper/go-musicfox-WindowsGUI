package ui

import (
	"fmt"

	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/menux"
)

// qqRecommendPlaylistMenu 展示 QQ 音乐推荐歌单。
type qqRecommendPlaylistMenu struct {
	baseMenu
	menus     []model.MenuItem
	playlists []structs.Playlist
	page      int
	hasMore   bool
}

// NewQQRecommendPlaylistMenu 创建推荐歌单菜单。
func NewQQRecommendPlaylistMenu(base baseMenu) *qqRecommendPlaylistMenu {
	return &qqRecommendPlaylistMenu{baseMenu: base, page: 1}
}

func (m *qqRecommendPlaylistMenu) IsSearchable() bool { return true }

func (m *qqRecommendPlaylistMenu) GetMenuKey() string { return "qq_recommend_playlists" }

func (m *qqRecommendPlaylistMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqRecommendPlaylistMenu) Playlists() []structs.Playlist { return m.playlists }

func (m *qqRecommendPlaylistMenu) SubMenu(_ *model.App, index int) model.Menu {
	if index < 0 || index >= len(m.playlists) {
		return nil
	}
	return NewQQPlaylistDetailMenu(m.baseMenu, m.playlists[index].Id)
}

func (m *qqRecommendPlaylistMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		m.page = 1
		if err := m.fetch(); err != nil {
			m.netease.NotifyError("加载推荐歌单失败", err.Error())
			return false, nil
		}
		if len(m.menus) == 0 {
			m.netease.NotifyInfo("推荐歌单", "暂无数据")
			return false, nil
		}
		return true, nil
	}
}

func (m *qqRecommendPlaylistMenu) fetch() error {
	playlists, err := m.netease.qqClient.GetRecommendSonglists(m.page, 30)
	if err != nil {
		return err
	}

	converted := toStructPlaylists(playlists)
	if m.page == 1 {
		m.playlists = converted
	} else {
		m.playlists = append(m.playlists, converted...)
	}
	m.hasMore = len(converted) > 0
	m.menus = menux.GetViewFromPlaylists(m.playlists)
	return nil
}

func (m *qqRecommendPlaylistMenu) BottomOutHook() model.Hook {
	if !m.hasMore {
		return nil
	}
	return func(main *model.Main) (bool, model.Page) {
		m.page++
		if err := m.fetch(); err != nil {
			m.page--
			return false, nil
		}
		return true, nil
	}
}

// qqUserPlaylistMenu 展示当前 QQ 账号的歌单（自建 + 收藏）。
type qqUserPlaylistMenu struct {
	baseMenu
	menus     []model.MenuItem
	playlists []structs.Playlist
}

// NewQQUserPlaylistMenu 创建「我的歌单」菜单。
func NewQQUserPlaylistMenu(base baseMenu) *qqUserPlaylistMenu {
	return &qqUserPlaylistMenu{baseMenu: base}
}

func (m *qqUserPlaylistMenu) IsSearchable() bool { return true }

func (m *qqUserPlaylistMenu) GetMenuKey() string { return "qq_user_playlists" }

func (m *qqUserPlaylistMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqUserPlaylistMenu) Playlists() []structs.Playlist { return m.playlists }

func (m *qqUserPlaylistMenu) SubMenu(_ *model.App, index int) model.Menu {
	if index < 0 || index >= len(m.playlists) {
		return nil
	}
	return NewQQPlaylistDetailMenu(m.baseMenu, m.playlists[index].Id)
}

func (m *qqUserPlaylistMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		if !m.netease.qqClient.LoggedIn() {
			page, _ := m.netease.ToQQLoginPage(EnterMenuCallback(main))
			return false, page
		}

		cred := m.netease.qqClient.Credential()
		created, err := m.netease.qqClient.GetCreatedSonglists(cred.MusicID)
		if err != nil {
			m.netease.NotifyError("加载我的歌单失败", err.Error())
			return false, nil
		}

		m.playlists = toStructPlaylists(created)
		m.menus = menux.GetViewFromPlaylists(m.playlists)
		if len(m.menus) == 0 {
			m.netease.NotifyInfo("我的歌单", "该账号暂无自建歌单")
			return false, nil
		}
		return true, nil
	}
}

// qqFavMenu 展示 QQ 音乐「我喜欢」歌单。
type qqFavMenu struct {
	baseMenu
	menus []model.MenuItem
	songs []structs.Song
}

// NewQQFavMenu 创建「我的收藏」菜单。
func NewQQFavMenu(base baseMenu) *qqFavMenu {
	return &qqFavMenu{baseMenu: base}
}

func (m *qqFavMenu) IsSearchable() bool { return true }

func (m *qqFavMenu) IsPlayable() bool { return true }

func (m *qqFavMenu) GetMenuKey() string { return "qq_fav_songs" }

func (m *qqFavMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqFavMenu) Songs() []structs.Song { return m.songs }

func (m *qqFavMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }

func (m *qqFavMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		if !m.netease.qqClient.LoggedIn() {
			page, _ := m.netease.ToQQLoginPage(EnterMenuCallback(main))
			return false, page
		}

		cred := m.netease.qqClient.Credential()
		songs, _, err := m.netease.qqClient.GetFavSongs(cred.MusicID, 1, 200)
		if err != nil {
			m.netease.NotifyError("加载我的收藏失败", err.Error())
			return false, nil
		}

		m.songs = toStructSongs(songs)
		m.menus = menux.GetViewFromSongs(m.songs)
		if len(m.menus) == 0 {
			m.netease.NotifyInfo("我的收藏", "暂无收藏歌曲")
			return false, nil
		}
		return true, nil
	}
}

// qqHotSingerMenu 展示 QQ 音乐热门歌手。
type qqHotSingerMenu struct {
	baseMenu
	menus   []model.MenuItem
	singers []qqSingerEntry
}

// qqSingerEntry 是歌手条目（保留 mid 供详情页使用）。
type qqSingerEntry struct {
	Mid  string
	Name string
}

// NewQQHotSingerMenu 创建热门歌手菜单。
func NewQQHotSingerMenu(base baseMenu) *qqHotSingerMenu {
	return &qqHotSingerMenu{baseMenu: base}
}

func (m *qqHotSingerMenu) IsSearchable() bool { return true }

func (m *qqHotSingerMenu) GetMenuKey() string { return "qq_hot_singers" }

func (m *qqHotSingerMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqHotSingerMenu) SubMenu(_ *model.App, index int) model.Menu {
	if index < 0 || index >= len(m.singers) {
		return nil
	}
	return NewQQArtistDetailMenuByMid(m.baseMenu, m.singers[index].Mid, m.singers[index].Name)
}

func (m *qqHotSingerMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if len(m.menus) > 0 {
			return true, nil
		}
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}

		singers, err := m.netease.qqClient.GetSingerList(1, 100)
		if err != nil {
			m.netease.NotifyError("加载热门歌手失败", err.Error())
			return false, nil
		}

		for _, s := range singers {
			m.singers = append(m.singers, qqSingerEntry{Mid: s.Mid, Name: s.Name})
			m.menus = append(m.menus, model.MenuItem{Title: s.Name})
		}
		if len(m.menus) == 0 {
			m.netease.NotifyInfo("热门歌手", "暂无数据")
			return false, nil
		}
		return true, nil
	}
}

// qqPlaylistIdOf 从歌单 ID 生成菜单 Key 的辅助（保持与详情菜单一致）。
func qqPlaylistIdOf(p structs.Playlist) string {
	return fmt.Sprintf("qq_playlist_detail_%d", p.Id)
}

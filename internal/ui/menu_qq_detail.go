package ui

import (
	"fmt"

	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/menux"
)

// qqPlaylistDetailMenu 展示 QQ 音乐歌单详情。
type qqPlaylistDetailMenu struct {
	baseMenu
	menus      []model.MenuItem
	songs      []structs.Song
	playlistId int64
	title      string

	page    int
	total   int
	hasMore bool
}

// NewQQPlaylistDetailMenu 创建 QQ 歌单详情菜单。
func NewQQPlaylistDetailMenu(base baseMenu, playlistId int64) *qqPlaylistDetailMenu {
	return &qqPlaylistDetailMenu{
		baseMenu:   base,
		playlistId: playlistId,
		page:       1,
	}
}

func (m *qqPlaylistDetailMenu) IsSearchable() bool { return true }

func (m *qqPlaylistDetailMenu) IsPlayable() bool { return true }

func (m *qqPlaylistDetailMenu) GetMenuKey() string {
	return fmt.Sprintf("qq_playlist_detail_%d", m.playlistId)
}

func (m *qqPlaylistDetailMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqPlaylistDetailMenu) Songs() []structs.Song { return m.songs }

func (m *qqPlaylistDetailMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }

func (m *qqPlaylistDetailMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}

		m.page = 1
		if err := m.fetch(); err != nil {
			m.netease.NotifyError("加载 QQ 音乐歌单失败", err.Error())
			return false, nil
		}
		return true, nil
	}
}

// fetch 拉取当前页歌曲。
func (m *qqPlaylistDetailMenu) fetch() error {
	detail, err := m.netease.qqClient.GetSonglistDetail(m.playlistId, m.page, 100)
	if err != nil {
		return err
	}

	songs := toStructSongs(detail.Songs)
	if m.page == 1 {
		m.songs = songs
		m.title = detail.Info.Title
	} else {
		m.songs = append(m.songs, songs...)
	}
	m.total = detail.Total
	m.hasMore = detail.HasMore
	m.menus = menux.GetViewFromSongs(m.songs)
	return nil
}

// BottomOutHook 翻到底部时加载下一页。
func (m *qqPlaylistDetailMenu) BottomOutHook() model.Hook {
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

// qqTopMenu 展示 QQ 音乐榜单分组。
type qqTopMenu struct {
	baseMenu
	menus      []model.MenuItem
	categories []qqTopCategoryEntry
}

// qqTopCategoryEntry 是扁平化后的榜单条目。
type qqTopCategoryEntry struct {
	GroupName string
	TopID     int64
	Title     string
	UpdateTip string
}

// NewQQTopMenu 创建 QQ 榜单菜单。
func NewQQTopMenu(base baseMenu) *qqTopMenu {
	return &qqTopMenu{baseMenu: base}
}

func (m *qqTopMenu) IsSearchable() bool { return true }

func (m *qqTopMenu) GetMenuKey() string { return "qq_tops" }

func (m *qqTopMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqTopMenu) SubMenu(_ *model.App, index int) model.Menu {
	if index < 0 || index >= len(m.categories) {
		return nil
	}
	return NewQQTopDetailMenu(m.baseMenu, m.categories[index].TopID, m.categories[index].Title)
}

func (m *qqTopMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if len(m.menus) > 0 {
			return true, nil
		}
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}

		groups, err := m.netease.qqClient.GetTopCategories()
		if err != nil {
			m.netease.NotifyError("加载 QQ 音乐榜单失败", err.Error())
			return false, nil
		}

		for _, group := range groups {
			for _, top := range group.Toplist {
				m.categories = append(m.categories, qqTopCategoryEntry{
					GroupName: group.GroupName,
					TopID:     top.TopID,
					Title:     top.Title,
					UpdateTip: top.UpdateTime,
				})
				m.menus = append(m.menus, model.MenuItem{
					Title:    top.Title,
					Subtitle: fmt.Sprintf("[%s]", group.GroupName),
				})
			}
		}

		if len(m.menus) == 0 {
			m.netease.NotifyInfo("QQ 音乐榜单", "暂无可用榜单")
			return false, nil
		}
		return true, nil
	}
}

// qqTopDetailMenu 展示单个榜单的歌曲。
type qqTopDetailMenu struct {
	baseMenu
	menus   []model.MenuItem
	songs   []structs.Song
	topID   int64
	title   string
	page    int
	hasMore bool
}

// NewQQTopDetailMenu 创建榜单详情菜单。
func NewQQTopDetailMenu(base baseMenu, topID int64, title string) *qqTopDetailMenu {
	return &qqTopDetailMenu{
		baseMenu: base,
		topID:    topID,
		title:    title,
		page:     1,
	}
}

func (m *qqTopDetailMenu) IsSearchable() bool { return true }

func (m *qqTopDetailMenu) IsPlayable() bool { return true }

func (m *qqTopDetailMenu) GetMenuKey() string {
	return fmt.Sprintf("qq_top_detail_%d", m.topID)
}

func (m *qqTopDetailMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqTopDetailMenu) Songs() []structs.Song { return m.songs }

func (m *qqTopDetailMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }

func (m *qqTopDetailMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		m.page = 1
		if err := m.fetch(); err != nil {
			m.netease.NotifyError("加载 QQ 音乐榜单失败", err.Error())
			return false, nil
		}
		return true, nil
	}
}

func (m *qqTopDetailMenu) fetch() error {
	detail, err := m.netease.qqClient.GetTopDetail(m.topID, m.page, 100)
	if err != nil {
		return err
	}

	songs := toStructSongs(detail.Songs)
	if m.page == 1 {
		m.songs = songs
		if detail.Info.Title != "" {
			m.title = detail.Info.Title
		}
	} else {
		m.songs = append(m.songs, songs...)
	}
	m.hasMore = detail.HasMore
	m.menus = menux.GetViewFromSongs(m.songs)
	return nil
}

func (m *qqTopDetailMenu) BottomOutHook() model.Hook {
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

// qqAlbumDetailMenu 展示 QQ 音乐专辑歌曲。
type qqAlbumDetailMenu struct {
	baseMenu
	menus    []model.MenuItem
	songs    []structs.Song
	albumMid string
	title    string
}

// NewQQAlbumDetailMenu 创建 QQ 专辑详情菜单。
func NewQQAlbumDetailMenu(base baseMenu, albumId int64, title string) *qqAlbumDetailMenu {
	return &qqAlbumDetailMenu{
		baseMenu: base,
		title:    title,
	}
}

// NewQQAlbumDetailMenuByMid 按专辑 MID 创建菜单（QQ 接口以 mid 为主键）。
func NewQQAlbumDetailMenuByMid(base baseMenu, albumMid, title string) *qqAlbumDetailMenu {
	return &qqAlbumDetailMenu{
		baseMenu: base,
		albumMid: albumMid,
		title:    title,
	}
}

func (m *qqAlbumDetailMenu) IsSearchable() bool { return true }

func (m *qqAlbumDetailMenu) IsPlayable() bool { return true }

func (m *qqAlbumDetailMenu) GetMenuKey() string {
	return fmt.Sprintf("qq_album_detail_%s", m.albumMid)
}

func (m *qqAlbumDetailMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqAlbumDetailMenu) Songs() []structs.Song { return m.songs }

func (m *qqAlbumDetailMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }

func (m *qqAlbumDetailMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		if m.albumMid == "" {
			m.netease.NotifyError("专辑信息缺失", "缺少专辑 MID")
			return false, nil
		}

		songs, _, err := m.netease.qqClient.GetAlbumSongs(m.albumMid, 1, 200)
		if err != nil {
			m.netease.NotifyError("加载 QQ 音乐专辑失败", err.Error())
			return false, nil
		}

		m.songs = toStructSongs(songs)
		m.menus = menux.GetViewFromSongs(m.songs)
		return true, nil
	}
}

// qqArtistDetailMenu 展示 QQ 音乐歌手歌曲。
type qqArtistDetailMenu struct {
	baseMenu
	menus     []model.MenuItem
	songs     []structs.Song
	singerMid string
	name      string
	page      int
	hasMore   bool
}

// NewQQArtistDetailMenu 创建 QQ 歌手详情菜单。
func NewQQArtistDetailMenu(base baseMenu, singerId int64, name string) *qqArtistDetailMenu {
	return &qqArtistDetailMenu{
		baseMenu: base,
		name:     name,
		page:     1,
	}
}

// NewQQArtistDetailMenuByMid 按歌手 MID 创建菜单。
func NewQQArtistDetailMenuByMid(base baseMenu, singerMid, name string) *qqArtistDetailMenu {
	return &qqArtistDetailMenu{
		baseMenu:  base,
		singerMid: singerMid,
		name:      name,
		page:      1,
	}
}

func (m *qqArtistDetailMenu) IsSearchable() bool { return true }

func (m *qqArtistDetailMenu) IsPlayable() bool { return true }

func (m *qqArtistDetailMenu) GetMenuKey() string {
	return fmt.Sprintf("qq_artist_detail_%s", m.singerMid)
}

func (m *qqArtistDetailMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqArtistDetailMenu) Songs() []structs.Song { return m.songs }

func (m *qqArtistDetailMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }

func (m *qqArtistDetailMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		if m.singerMid == "" {
			m.netease.NotifyError("歌手信息缺失", "缺少歌手 MID")
			return false, nil
		}

		m.page = 1
		if err := m.fetch(); err != nil {
			m.netease.NotifyError("加载 QQ 音乐歌手歌曲失败", err.Error())
			return false, nil
		}
		return true, nil
	}
}

func (m *qqArtistDetailMenu) fetch() error {
	songs, total, err := m.netease.qqClient.GetSingerSongs(m.singerMid, m.page, 100)
	if err != nil {
		return err
	}

	converted := toStructSongs(songs)
	if m.page == 1 {
		m.songs = converted
	} else {
		m.songs = append(m.songs, converted...)
	}
	m.hasMore = len(m.songs) < total
	m.menus = menux.GetViewFromSongs(m.songs)
	return nil
}

func (m *qqArtistDetailMenu) BottomOutHook() model.Hook {
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

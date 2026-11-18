package ui

import (
	"fmt"

	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/menux"
)

// QQSearchType 是 QQ 渠道的搜索维度。
type QQSearchType uint8

const (
	QQSearchSong QQSearchType = iota
	QQSearchSinger
	QQSearchAlbum
	QQSearchPlaylist
)

// qqSearchTypeMenu 是 QQ 渠道的搜索维度选择菜单。
type qqSearchTypeMenu struct {
	baseMenu
	menus []model.MenuItem
}

// NewQQSearchTypeMenu 创建 QQ 搜索维度菜单。
func NewQQSearchTypeMenu(base baseMenu) *qqSearchTypeMenu {
	return &qqSearchTypeMenu{
		baseMenu: base,
		menus: []model.MenuItem{
			{Title: "按单曲"},
			{Title: "按歌手"},
			{Title: "按专辑"},
			{Title: "按歌单"},
		},
	}
}

func (m *qqSearchTypeMenu) GetMenuKey() string { return "qq_search_type" }

func (m *qqSearchTypeMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqSearchTypeMenu) SubMenu(_ *model.App, index int) model.Menu {
	types := []QQSearchType{QQSearchSong, QQSearchSinger, QQSearchAlbum, QQSearchPlaylist}
	if index < 0 || index >= len(types) {
		return nil
	}
	return NewQQSearchResultMenu(m.baseMenu, types[index])
}

// qqSearchResultMenu 展示 QQ 渠道的搜索结果。
type qqSearchResultMenu struct {
	baseMenu
	menus      []model.MenuItem
	searchType QQSearchType
	keyword    string

	songs     []structs.Song
	singers   []structs.Artist
	albums    []structs.Album
	playlists []structs.Playlist

	page  int
	total int
}

// NewQQSearchResultMenu 创建 QQ 搜索结果菜单。
func NewQQSearchResultMenu(base baseMenu, searchType QQSearchType) *qqSearchResultMenu {
	return &qqSearchResultMenu{
		baseMenu:   base,
		searchType: searchType,
		page:       1,
	}
}

func (m *qqSearchResultMenu) IsSearchable() bool { return true }

func (m *qqSearchResultMenu) IsPlayable() bool {
	return m.searchType == QQSearchSong
}

func (m *qqSearchResultMenu) GetMenuKey() string {
	return fmt.Sprintf("qq_search_result_%d_%s", m.searchType, m.keyword)
}

func (m *qqSearchResultMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqSearchResultMenu) Songs() []structs.Song { return m.songs }

func (m *qqSearchResultMenu) Playlists() []structs.Playlist { return m.playlists }

func (m *qqSearchResultMenu) Albums() []structs.Album { return m.albums }

func (m *qqSearchResultMenu) Artists() []structs.Artist { return m.singers }

func (m *qqSearchResultMenu) SubMenu(_ *model.App, index int) model.Menu {
	switch m.searchType {
	case QQSearchPlaylist:
		if index < len(m.playlists) {
			return NewQQPlaylistDetailMenu(m.baseMenu, m.playlists[index].Id)
		}
	case QQSearchAlbum:
		if index < len(m.albums) {
			return NewQQAlbumDetailMenu(m.baseMenu, m.albums[index].Id, m.albums[index].Name)
		}
	case QQSearchSinger:
		if index < len(m.singers) {
			return NewQQArtistDetailMenu(m.baseMenu, m.singers[index].Id, m.singers[index].Name)
		}
	}
	return nil
}

func (m *qqSearchResultMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		// 复用网易云的搜索页输入框：它只负责收集关键词。
		if m.netease.search.wordsInput.Value() == "" {
			page, _ := m.netease.ToSearchPage(StSingleSong)
			return false, page
		}

		m.keyword = m.netease.search.wordsInput.Value()
		m.page = 1

		if err := m.fetch(); err != nil {
			m.netease.NotifyError("QQ 音乐搜索失败", err.Error())
			return false, nil
		}
		return true, nil
	}
}

// fetch 拉取当前页搜索结果。
func (m *qqSearchResultMenu) fetch() error {
	client := m.netease.qqClient
	if client == nil {
		return fmt.Errorf("QQ 音乐渠道未初始化")
	}

	searchType := qqSearchTypeToAPI(m.searchType)
	result, err := client.Search(m.keyword, searchType, m.page, 30)
	if err != nil {
		return err
	}

	switch m.searchType {
	case QQSearchSong:
		m.songs = toStructSongs(result.Songs)
		m.menus = menux.GetViewFromSongs(m.songs)
	case QQSearchSinger:
		m.singers = make([]structs.Artist, 0, len(result.Singers))
		for _, s := range result.Singers {
			m.singers = append(m.singers, structs.Artist{Id: s.ID, Name: s.Name})
		}
		m.menus = menux.GetViewFromArtists(m.singers)
	case QQSearchAlbum:
		m.albums = make([]structs.Album, 0, len(result.Albums))
		for _, a := range result.Albums {
			m.albums = append(m.albums, structs.Album{
				Id:     a.ID,
				Name:   a.Name,
				PicUrl: a.CoverURL(300),
			})
		}
		m.menus = menux.GetViewFromAlbums(m.albums)
	case QQSearchPlaylist:
		m.playlists = toStructPlaylists(result.Playlists)
		m.menus = menux.GetViewFromPlaylists(m.playlists)
	}

	m.total = result.Total
	return nil
}

// BottomOutHook 在翻到底部时加载下一页。
func (m *qqSearchResultMenu) BottomOutHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		m.page++
		if err := m.fetch(); err != nil {
			m.page--
			return false, nil
		}
		return true, nil
	}
}

// qqSearchTypeToAPI 映射到 QQ 接口的搜索类型。
func qqSearchTypeToAPI(t QQSearchType) qqmusicSearchType {
	switch t {
	case QQSearchSinger:
		return qqmusicSearchTypeSinger
	case QQSearchAlbum:
		return qqmusicSearchTypeAlbum
	case QQSearchPlaylist:
		return qqmusicSearchTypeSonglist
	default:
		return qqmusicSearchTypeSong
	}
}

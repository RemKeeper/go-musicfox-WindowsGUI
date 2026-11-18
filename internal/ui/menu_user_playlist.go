package ui

import (
	"fmt"

	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/netease"
	"github.com/go-musicfox/go-musicfox/internal/structs"
	_struct "github.com/go-musicfox/go-musicfox/utils/struct"
)

const CurUser int64 = 0

type UserPlaylistMenu struct {
	baseMenu
	menus     []model.MenuItem
	playlists []structs.Playlist
	userId    int64
	offset    int
	limit     int
	hasMore   bool
}

func NewUserPlaylistMenu(base baseMenu, userId int64) *UserPlaylistMenu {
	return &UserPlaylistMenu{
		baseMenu: base,
		userId:   userId,
		offset:   0,
		limit:    100,
	}
}

func (m *UserPlaylistMenu) IsSearchable() bool {
	return true
}

func (m *UserPlaylistMenu) GetMenuKey() string {
	return fmt.Sprintf("user_playlist_%d", m.userId)
}

func (m *UserPlaylistMenu) MenuViews() []model.MenuItem {
	return m.menus
}

func (m *UserPlaylistMenu) Playlists() []structs.Playlist {
	return m.playlists
}

func (m *UserPlaylistMenu) SubMenu(_ *model.App, index int) model.Menu {
	// 必须用 >= 而非 >：index 等于长度时同样越界。
	// 未登录或尚未加载歌单时 playlists 为空，此时点击任意项都会 panic。
	if index < 0 || index >= len(m.playlists) {
		return nil
	}
	return NewPlaylistDetailMenu(m.baseMenu, m.playlists[index].Id)
}

func (m *UserPlaylistMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		// 等于0，获取当前用户歌单
		if m.userId == CurUser && _struct.CheckUserInfo(m.netease.user) == _struct.NeedLogin {
			page, _ := m.netease.ToLoginPage(EnterMenuCallback(main))
			return false, page
		}

		userId := m.userId
		if m.userId == CurUser {
			// 等于0，获取当前用户歌单
			userId = m.netease.user.UserId
		}

		codeType, playlists, hasMore := netease.FetchUserPlaylists(userId, m.limit, m.offset)
		if codeType == _struct.NeedLogin {
			page, _ := m.netease.ToLoginPage(EnterMenuCallback(main))
			return false, page
		} else if codeType != _struct.Success {
			return false, nil
		}

		m.playlists = playlists
		var menus []model.MenuItem
		for _, playlist := range m.playlists {
			menus = append(menus, model.MenuItem{Title: _struct.ReplaceSpecialStr(playlist.Name)})
		}
		m.menus = menus
		m.hasMore = hasMore

		return true, nil
	}
}

func (m *UserPlaylistMenu) BottomOutHook() model.Hook {
	if !m.hasMore {
		return nil
	}
	return func(main *model.Main) (bool, model.Page) {
		userId := m.userId
		if m.userId == CurUser {
			// 等于0，获取当前用户歌单
			userId = m.netease.user.UserId
		}

		m.offset = m.offset + len(m.menus)
		codeType, playlists, hasMore := netease.FetchUserPlaylists(userId, m.limit, m.offset)
		if codeType == _struct.NeedLogin {
			page, _ := m.netease.ToLoginPage(func() model.Page {
				main.RefreshMenuList()
				return nil
			})
			return false, page
		} else if codeType != _struct.Success {
			return false, nil
		}

		m.playlists = append(m.playlists, playlists...)
		var menus []model.MenuItem
		for _, playlist := range m.playlists {
			menus = append(menus, model.MenuItem{Title: _struct.ReplaceSpecialStr(playlist.Name)})
		}
		m.menus = menus
		m.hasMore = hasMore

		return true, nil
	}
}

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
)

const (
	mainMenuHelpIndex        = 14
	mainMenuCheckUpdateIndex = 15
	// mainMenuSourceIndex 是「切换音乐源」入口的位置。
	mainMenuSourceIndex = 16
)

// MainMenu 是主界面一级菜单。
//
// 菜单内容随当前渠道（网易云 / QQ 音乐）动态生成：切换渠道后，
// 一级菜单项与子菜单同时替换，播放器、歌词、播放列表保持不变。
type MainMenu struct {
	baseMenu
	menus    []model.MenuItem
	menuList []Menu
}

// NewMainMenu 创建主菜单。
//
// 初始内容取决于当前渠道，因此必须先确定 n.source。
func NewMainMenu(netease *Netease) *MainMenu {
	base := newBaseMenu(netease)
	mainMenu := &MainMenu{baseMenu: base}
	mainMenu.rebuild()
	return mainMenu
}

// rebuild 按当前渠道重建菜单项与子菜单。
func (m *MainMenu) rebuild() {
	if m.netease.source == SourceQQ {
		m.buildQQ()
		return
	}
	m.buildNetease()
}

// buildNetease 构建网易云渠道菜单（保持原有顺序与内容）。
func (m *MainMenu) buildNetease() {
	base := m.baseMenu
	m.menus = []model.MenuItem{
		{Title: "每日推荐歌曲"},
		{Title: "每日推荐歌单"},
		{Title: "我的歌单"},
		{Title: "我的收藏"},
		{Title: "私人FM"},
		{Title: "专辑列表"},
		{Title: "搜索"},
		{Title: "排行榜"},
		{Title: "精选歌单"},
		{Title: "热门歌手"},
		{Title: "最近播放歌曲"},
		{Title: "云盘"},
		{Title: "主播电台"},
		{Title: "LastFM"},
		{Title: "帮助"},
		{Title: "检查更新"},
		{Title: "切换音乐源"},
	}
	m.menuList = []Menu{
		NewDailyRecommendSongsMenu(base),
		NewDailyRecommendPlaylistMenu(base),
		NewUserPlaylistMenu(base, CurUser),
		NewUserCollectionMenu(base),
		NewPersonalFmMenu(base),
		NewAlbumListMenu(base),
		NewSearchTypeMenu(base),
		NewRanksMenu(base),
		NewHighQualityPlaylistsMenu(base),
		NewHotArtistsMenu(base),
		NewRecentSongsMenu(base),
		NewCloudMenu(base),
		NewRadioDjTypeMenu(base),
		NewLastfm(base),
		nil, // 帮助由 Action 直接打开 Markdown 弹窗，不再进入子菜单。
		nil, // 检查更新由 Action 异步执行，并直接显示 TUI 通知。
		nil, // 切换音乐源由 Action 直接处理。
	}
}

// buildQQ 构建 QQ 音乐渠道菜单。
//
// 只保留 QQ 音乐确实提供的能力：搜索、排行榜、推荐歌单、我的歌单、
// 我的收藏、热门歌手、专辑。云盘、主播电台、私人FM 在 QQ 侧无对应接口，
// 因此不出现，避免用户点进去看到空列表。
func (m *MainMenu) buildQQ() {
	base := m.baseMenu
	m.menus = []model.MenuItem{
		{Title: "猜你喜欢"},
		{Title: "雷达推荐"},
		{Title: "搜索"},
		{Title: "排行榜"},
		{Title: "推荐歌单"},
		{Title: "我的歌单"},
		{Title: "我的收藏"},
		{Title: "热门歌手"},
		{Title: "帮助"},
		{Title: "检查更新"},
		{Title: "切换音乐源"},
	}
	m.menuList = []Menu{
		NewQQGuessRadioMenu(base),
		NewQQRadarMenu(base),
		NewQQSearchTypeMenu(base),
		NewQQTopMenu(base),
		NewQQRecommendPlaylistMenu(base),
		NewQQUserPlaylistMenu(base),
		NewQQFavMenu(base),
		NewQQHotSingerMenu(base),
		nil, // 帮助
		nil, // 检查更新
		nil, // 切换音乐源
	}
}

// qqSourceIndex 返回 QQ 菜单中「切换音乐源」的下标。
func (m *MainMenu) qqSourceIndex() int { return len(m.menus) - 1 }

// neteaseSourceIndex 返回网易云菜单中「切换音乐源」的下标。
func (m *MainMenu) neteaseSourceIndex() int { return mainMenuSourceIndex }

func (m *MainMenu) FormatMenuItem(item *model.MenuItem) {
	subtitle := "[未登录]"
	if m.netease.source == SourceQQ {
		if m.netease.qqUser != nil {
			subtitle = "[" + m.netease.qqUser.Nickname + "]"
		}
	} else if m.netease.user != nil {
		subtitle = "[" + m.netease.user.Nickname + "]"
	}
	item.Subtitle = subtitle
}

func (m *MainMenu) GetMenuKey() string {
	return "main_menu"
}

func (m *MainMenu) MenuViews() []model.MenuItem {
	for i, menu := range m.menuList {
		if menu != nil && i < len(m.menus) {
			menu.FormatMenuItem(&m.menus[i])
		}
	}
	return m.menus
}

func (m *MainMenu) SubMenu(_ *model.App, index int) model.Menu {
	if index < 0 || index >= len(m.menuList) {
		return nil
	}
	return m.menuList[index]
}

func (m *MainMenu) Action(app *model.App, index int) (model.Page, tea.Cmd) {
	// 渠道相关菜单的「切换音乐源」下标随菜单长度变化。
	if index == m.qqSourceIndex() || index == m.neteaseSourceIndex() {
		return m.switchSource(app)
	}

	switch index {
	case mainMenuHelpIndex:
		showHelpPopup(app)
		return app.MustMain(), nil
	case mainMenuCheckUpdateIndex:
		return app.MustMain(), checkUpdateCmd()
	default:
		return m.baseMenu.Action(app, index)
	}
}

// switchSource 切换到另一个音乐渠道并重建菜单。
func (m *MainMenu) switchSource(app *model.App) (model.Page, tea.Cmd) {
	next := otherSource(m.netease.source)
	// SetSource 内部已完成菜单重建、列表刷新与分页重置。
	m.netease.SetSource(next)

	m.netease.NotifyInfo("已切换音乐源", sourceName(next))
	return app.MustMain(), app.RerenderCmd(true)
}

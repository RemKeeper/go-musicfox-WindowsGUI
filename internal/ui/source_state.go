package ui

import (
	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/structs"
)

// Source 返回当前激活的音乐渠道。
func (n *Netease) Source() Source {
	return n.source
}

// SetSource 切换当前音乐渠道。
//
// 只改变「数据来源」：播放器、歌词服务、播放列表、桌面歌词都保持原样，
// 因此切换渠道不会中断正在播放的歌曲。
//
// 切换后两个渠道的菜单项数量不同（网易云 17 项、QQ 音乐 9 项），
// 必须同时刷新 menuList 并重置选中位置与分页，否则 View 里
// menuList[start:end] 会因 start > end 而 panic。
func (n *Netease) SetSource(src Source) {
	n.source = normalizeSource(src)

	if n.App == nil {
		return
	}
	main := n.MustMain()
	if main == nil {
		return
	}

	// 主菜单需要按新渠道重建（子菜单与菜单项都会变化）。
	if menu, ok := main.CurMenu().(*MainMenu); ok {
		menu.rebuild()
	}
	main.RefreshMenuList()
	main.MoveTop()

	// 顶栏标题显示当前渠道名，必须随切换更新。
	if title := main.MenuTitle(); title != nil {
		title.Title = sourceName(n.source)
		main.RefreshMenuTitle()
	}
}

// SourceName 返回当前渠道的显示名，用于顶栏标题。
func (n *Netease) SourceName() string {
	return sourceName(n.source)
}

// CurrentUser 返回当前渠道的登录用户，未登录为 nil。
func (n *Netease) CurrentUser() *structs.User {
	if n.source == SourceQQ {
		return n.qqUser
	}
	return n.user
}

// IsLoggedIn 报告当前渠道是否已登录。
func (n *Netease) IsLoggedIn() bool {
	if n.source == SourceQQ {
		return n.qqClient != nil && n.qqClient.LoggedIn()
	}
	return n.user != nil && n.user.UserId != 0
}

// ToCurrentLoginPage 跳转到当前渠道的登录页。
//
// 网易云走原有登录页；QQ 音乐走扫码登录页。
func (n *Netease) ToCurrentLoginPage(callback func() model.Page) (model.Page, bool) {
	if n.source == SourceQQ {
		page, _ := n.ToQQLoginPage(callback)
		return page, true
	}
	page, _ := n.ToLoginPage(callback)
	return page, true
}

package ui

import (
	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/go-musicfox/utils/notify"
)

// QQ 渠道的搜索类型别名，避免在菜单文件中直接依赖 qqmusic 常量名。
type qqmusicSearchType = qqmusic.SearchType

const (
	qqmusicSearchTypeSong     = qqmusic.SearchSong
	qqmusicSearchTypeSinger   = qqmusic.SearchSinger
	qqmusicSearchTypeAlbum    = qqmusic.SearchAlbum
	qqmusicSearchTypeSonglist = qqmusic.SearchSonglist
)

// NotifyError 在 TUI 内显示一条错误通知。
//
// 供 QQ 渠道菜单在接口失败时反馈，避免静默失败让用户误以为无数据。
func (n *Netease) NotifyError(title, message string) {
	if n == nil || n.App == nil {
		return
	}
	n.App.Notify(model.NotificationSpec{
		Level:   model.NotificationError,
		Title:   title,
		Message: message,
	})
}

// NotifyInfo 在 TUI 内显示一条提示通知。
func (n *Netease) NotifyInfo(title, message string) {
	if n == nil || n.App == nil {
		return
	}
	n.App.Notify(model.NotificationSpec{
		Level:   model.NotificationInfo,
		Title:   title,
		Message: message,
	})
}

// notifyDesktop 同时发送桌面通知（受配置控制）。
func (n *Netease) notifyDesktop(title, text string) {
	go notify.Notify(notify.NotifyContent{
		Title: title,
		Text:  text,
	})
}

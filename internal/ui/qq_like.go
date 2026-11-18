package ui

import (
	"log/slog"

	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/likelist"
	"github.com/go-musicfox/go-musicfox/utils/notify"
	"github.com/go-musicfox/go-musicfox/utils/slogx"
)

// likeQQSong 在 QQ 音乐渠道收藏/取消收藏一首歌。
//
// QQ 的「我喜欢」是 dirid=201 的虚拟歌单，收藏即向该歌单增删歌曲。
// 成功后做乐观更新（likelist.SetLike），避免为了刷新一个图标重新拉整表。
func likeQQSong(n *Netease, song structs.Song, isLike bool) model.Page {
	if n.qqClient == nil {
		n.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
		return nil
	}
	if !n.qqClient.LoggedIn() {
		page, _ := n.ToQQLoginPage(func() model.Page {
			return likeSong(n, isLike, true)
		})
		return page
	}

	var err error
	if isLike {
		err = n.qqClient.LikeSong(song.Id, song.SongType)
	} else {
		err = n.qqClient.UnlikeSong(song.Id, song.SongType)
	}

	if err != nil {
		slog.Warn("QQ 音乐收藏操作失败", "songId", song.Id, slogx.Error(err))
		n.NotifyError("操作失败", err.Error())
		return nil
	}

	// 乐观更新本地状态，让图标立即反映结果。
	likelist.SetLike(structs.SourceQQ, song.Id, isLike)

	title := "已添加到我喜欢"
	if !isLike {
		title = "已从我喜欢的移除"
	}
	notify.Notify(notify.NotifyContent{
		Title: title,
		Text:  song.Name,
	})
	n.Rerender(false)
	return nil
}

// refreshQQLikeList 拉取 QQ 音乐「我喜欢」全量 ID 并写入本地缓存。
//
// 用于启动时或登录后同步收藏状态，使歌曲列表里的爱心图标正确。
func refreshQQLikeList(n *Netease) {
	if n.qqClient == nil || !n.qqClient.LoggedIn() {
		return
	}

	cred := n.qqClient.Credential()
	var ids []int64
	const pageSize = 500

	// 分页拉取，上限 20 页（1 万首）以防异常情况下无限循环。
	for page := 1; page <= 20; page++ {
		songs, total, err := n.qqClient.GetFavSongs(cred.MusicID, page, pageSize)
		if err != nil {
			slog.Warn("拉取 QQ 音乐我喜欢失败", "page", page, slogx.Error(err))
			break
		}
		for _, s := range songs {
			ids = append(ids, s.ID)
		}
		if len(songs) == 0 || len(ids) >= total {
			break
		}
	}

	likelist.Replace(structs.SourceQQ, ids)
	slog.Info("QQ 音乐我喜欢已同步", "count", len(ids))
}

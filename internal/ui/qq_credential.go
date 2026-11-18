package ui

import (
	"encoding/json"
	"log/slog"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/go-musicfox/internal/storage"
	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/slogx"
)

// saveQQCredential 持久化 QQ 音乐登录态。
func saveQQCredential(cred qqmusic.Credential) error {
	table := storage.NewTable()
	return table.SetByKVModel(storage.QQMusicCredential{}, cred)
}

// loadQQCredential 读取并应用已保存的 QQ 音乐登录态。
//
// 返回是否成功恢复登录态。凭证无效或读取失败都静默降级为未登录，
// 不阻断启动。
func loadQQCredential(n *Netease) bool {
	if n.qqClient == nil {
		return false
	}

	table := storage.NewTable()
	raw, err := table.GetByKVModel(storage.QQMusicCredential{})
	if err != nil || len(raw) == 0 {
		return false
	}

	var cred qqmusic.Credential
	if err := json.Unmarshal(raw, &cred); err != nil {
		slog.Warn("QQ 音乐凭证解析失败", slogx.Error(err))
		return false
	}
	if !cred.Valid() {
		return false
	}

	n.qqClient.SetCredential(cred)

	// musickey 是短时效的（默认 3 天），过期后所有需要登录的接口都会
	// 退化为匿名，表现为「无法播放」。启动时按需续期。
	n.qqClient.EnsureFresh()
	if fresh := n.qqClient.Credential(); fresh.MusicKey != cred.MusicKey {
		if err := saveQQCredential(fresh); err != nil {
			slog.Warn("QQ 音乐凭证续期后保存失败", slogx.Error(err))
		}
		cred = fresh
	}

	n.qqUser = &structs.User{
		UserId:   cred.MusicID,
		Nickname: qqNickname(cred.MusicID),
	}
	slog.Info("QQ 音乐登录态已恢复", "musicid", cred.MusicID)
	return true
}

// clearQQCredential 清除持久化的 QQ 音乐登录态。
func clearQQCredential(n *Netease) {
	if n.qqClient != nil {
		n.qqClient.Logout()
	}
	n.qqUser = nil

	table := storage.NewTable()
	if err := table.DeleteByKVModel(storage.QQMusicCredential{}); err != nil {
		slog.Warn("清除 QQ 音乐凭证失败", slogx.Error(err))
	}
}

// qqNickname 生成 QQ 用户的展示名。
//
// QQ 接口没有在登录响应里直接给出昵称，这里用 ID 兜底；
// 后续可调用用户信息接口补全。
func qqNickname(musicID int64) string {
	return "QQ用户" + itoa(musicID)
}

// itoa 是 strconv.FormatInt 的薄封装，避免在此文件引入额外 import。
func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

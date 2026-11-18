package storage

import (
	"github.com/go-musicfox/go-musicfox/internal/types"
)

// QQMusicCredential 持久化 QQ 音乐登录态。
//
// 与网易云的 Cookie 文件分开存放，避免两个渠道的登录态互相覆盖。
type QQMusicCredential struct{}

func (c QQMusicCredential) GetDbName() string {
	return types.AppDBName
}

func (c QQMusicCredential) GetTableName() string {
	return "default_bucket"
}

func (c QQMusicCredential) GetKey() string {
	return "qq_music_credential"
}

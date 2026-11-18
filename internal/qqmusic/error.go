package qqmusic

import (
	"errors"
	"fmt"
)

// errRiskControl 表示请求被风控拦截，换指纹重试即可恢复。
var errRiskControl = errors.New("qqmusic: 触发风控")

// ErrNotLoggedIn 表示接口需要登录态。
var ErrNotLoggedIn = errors.New("qqmusic: 需要登录")

// ErrNoPlayableURL 表示歌曲无可播放链接。
//
// 常见原因：该版本无版权、已下架、或需付费购买。
// 调用方应提示用户换一个版本，而不是笼统报「播放失败」。
var ErrNoPlayableURL = errors.New("qqmusic: 无可播放链接")

// ErrCopyrightRestricted 表示歌曲因版权原因不可播放。
//
// 与 ErrNoPlayableURL 的区别：错误码明确为 104003 时用它，
// 便于上层区分「版权问题」和「网络/参数问题」。
var ErrCopyrightRestricted = errors.New("qqmusic: 该版本无版权或需付费，无法播放")

// APIError 是 CGI 业务错误。
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("qqmusic: code=%d, msg=%s", e.Code, e.Msg)
	}
	return fmt.Sprintf("qqmusic: code=%d", e.Code)
}

// 常见业务错误码（来自 QQMusicApi 文档与实测）。
const (
	// CodeNoPermission 无权限（未登录、无版权、或等级不够）。
	CodeNoPermission = 104003
	// CodeVKeyFailed VKey 获取失败。
	CodeVKeyFailed = 104004
	// CodeDeviceRestricted 播放设备受限。
	CodeDeviceRestricted = 104013
	// CodeSongNotExist 歌曲不存在或已下架。
	CodeSongNotExist = 101404
)

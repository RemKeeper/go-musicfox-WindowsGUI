package qqmusic

import "fmt"

// favDirID 是「我喜欢」歌单的固定目录 ID。
//
// QQ 音乐把「我喜欢」建模为一个 dirid=201 的虚拟歌单，收藏/取消收藏
// 都通过歌单写接口操作它，而不是独立接口。
const favDirID = 201

// songTypeForWrite 把读取接口返回的歌曲类型转换为写接口使用的类型。
//
// 对齐 QQMusicApi utils/common.py:to_songlist_write_type：
// 0/1 统一为 0（普通单曲），5/111/112/113 原样保留（特殊曲目），
// 其余一律按 1 处理。
func songTypeForWrite(songType int) int {
	switch songType {
	case 0, 1:
		return 0
	case 5, 111, 112, 113:
		return songType
	default:
		return 1
	}
}

// songWriteInfo 是歌单写操作的歌曲项。
type songWriteInfo struct {
	SongID   int64 `json:"songId"`
	SongType int   `json:"songType"`
}

// buildSonglistOperParam 构造歌单写操作的最小参数。
func buildSonglistOperParam(dirID int64, tid int64, songs []songWriteInfo) map[string]any {
	return map[string]any{
		"dirId":      dirID,
		"tid":        tid,
		"bFmtUtf8":   true,
		"v_songInfo": songs,
	}
}

// LikeSong 将歌曲加入「我喜欢」。
//
// songType 使用读取结果里的 Song.Type（普通单曲传 0）。
// 歌曲已在「我喜欢」中也返回 nil（幂等）。
func (c *Client) LikeSong(songID int64, songType int) error {
	return c.songlistWrite("AddSonglist", favDirID, songID, songType)
}

// UnlikeSong 将歌曲移出「我喜欢」。
//
// 歌曲本就不在「我喜欢」中也返回 nil（幂等）。
func (c *Client) UnlikeSong(songID int64, songType int) error {
	return c.songlistWrite("DelSonglist", favDirID, songID, songType)
}

// songlistWrite 执行一次歌单写操作。
func (c *Client) songlistWrite(method string, dirID, songID int64, songType int) error {
	if !c.LoggedIn() {
		return ErrNotLoggedIn
	}
	if songID == 0 {
		return fmt.Errorf("qqmusic: 歌曲 ID 无效")
	}

	songs := []songWriteInfo{{
		SongID:   songID,
		SongType: songTypeForWrite(songType),
	}}

	data, err := c.Do(cgiRequest{
		Module: "music.musicasset.PlaylistDetailWrite",
		Method: method,
		// 写接口必须走 Android 平台：Web 平台下即使带 uin/g_tk
		// 也会返回 code=1000（参数校验失败），表现为「收藏操作失败」。
		Android: true,
		Param:   buildSonglistOperParam(dirID, 0, songs),
	})
	if err != nil {
		return err
	}

	var parsed struct {
		RetCode int `json:"retCode"`
	}
	if err := decode(data, &parsed); err != nil {
		return err
	}
	// retCode 非 0 表示业务失败；80092 是「歌曲已在/不在歌单」的幂等码。
	if parsed.RetCode != 0 && parsed.RetCode != 80092 {
		return &APIError{Code: parsed.RetCode, Msg: "歌单写入失败"}
	}
	return nil
}

// AddSongsToPlaylist 把歌曲加入指定歌单（dirid 非 201 时用于自建歌单）。
func (c *Client) AddSongsToPlaylist(dirID int64, songID int64, songType int) error {
	return c.songlistWrite("AddSonglist", dirID, songID, songType)
}

// RemoveSongsFromPlaylist 从指定歌单移除歌曲。
func (c *Client) RemoveSongsFromPlaylist(dirID int64, songID int64, songType int) error {
	return c.songlistWrite("DelSonglist", dirID, songID, songType)
}

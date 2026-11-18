package qqmusic

import (
	"errors"
	"testing"
)

// TestSongTypeForWrite 校验歌单写入类型转换，
// 对齐 QQMusicApi utils/common.py:to_songlist_write_type。
func TestSongTypeForWrite(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 0},     // 普通单曲
		{1, 0},     // 旧类型，归一为 0
		{5, 5},     // 保留
		{111, 111}, // 保留
		{112, 112},
		{113, 113},
		{2, 1}, // 其他一律 1
		{99, 1},
		{-1, 1},
	}
	for _, tc := range cases {
		if got := songTypeForWrite(tc.in); got != tc.want {
			t.Errorf("songTypeForWrite(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestLikeRequiresLogin 校验未登录时收藏操作返回 ErrNotLoggedIn。
func TestLikeRequiresLogin(t *testing.T) {
	c := NewClient() // 无凭证

	if err := c.LikeSong(97773, 0); !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("LikeSong 未登录应返回 ErrNotLoggedIn，实际: %v", err)
	}
	if err := c.UnlikeSong(97773, 0); !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("UnlikeSong 未登录应返回 ErrNotLoggedIn，实际: %v", err)
	}
}

// TestLikeRejectsZeroSongID 校验非法歌曲 ID 被拒绝。
func TestLikeRejectsZeroSongID(t *testing.T) {
	c := NewClient()
	c.SetCredential(Credential{MusicID: 123, MusicKey: "Q_H_L_test"})

	if err := c.LikeSong(0, 0); err == nil {
		t.Error("songID=0 应报错")
	}
}

// TestBuildSonglistOperParam 校验写操作参数结构。
func TestBuildSonglistOperParam(t *testing.T) {
	songs := []songWriteInfo{{SongID: 97773, SongType: 0}}
	param := buildSonglistOperParam(favDirID, 0, songs)

	if got := param["dirId"]; got != int64(favDirID) {
		t.Errorf("dirId = %v, want %d", got, favDirID)
	}
	if got := param["bFmtUtf8"]; got != true {
		t.Errorf("bFmtUtf8 = %v, want true", got)
	}
	list, ok := param["v_songInfo"].([]songWriteInfo)
	if !ok {
		t.Fatalf("v_songInfo 类型错误: %T", param["v_songInfo"])
	}
	if len(list) != 1 || list[0].SongID != 97773 {
		t.Errorf("v_songInfo 内容错误: %+v", list)
	}
}

// TestFavDirID 锁定「我喜欢」的固定目录 ID。
func TestFavDirID(t *testing.T) {
	if favDirID != 201 {
		t.Errorf("favDirID = %d, want 201（QQ 音乐「我喜欢」固定 dirid）", favDirID)
	}
}

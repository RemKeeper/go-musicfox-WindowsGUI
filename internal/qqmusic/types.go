package qqmusic

import (
	"encoding/json"
	"fmt"
	"time"
)

// Song 是 QQ 音乐的歌曲原始结构（仅保留 musicfox 需要的字段）。
type Song struct {
	ID       int64    `json:"id"`
	Mid      string   `json:"mid"`
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	Type     int      `json:"type"`     // 歌单写入类型，普通单曲为 0
	Interval int      `json:"interval"` // 秒
	Singer   []Singer `json:"singer"`
	Album    Album    `json:"album"`
	File     File     `json:"file"`
	Pay      Pay      `json:"pay"`
	Status   int      `json:"status"`
	MV       MV       `json:"mv"`
}

// Singer 是歌手摘要。
type Singer struct {
	ID   int64  `json:"id"`
	Mid  string `json:"mid"`
	Name string `json:"name"`
}

// Album 是专辑摘要。
type Album struct {
	ID   int64  `json:"id"`
	Mid  string `json:"mid"`
	Name string `json:"name"`
}

// CoverURL 返回专辑封面链接。
func (a Album) CoverURL(size int) string {
	return AlbumCoverURL(a.Mid, size)
}

// File 是歌曲各音质的文件信息。
type File struct {
	MediaMid string  `json:"media_mid"`
	Size128  int64   `json:"size_128mp3"`
	Size320  int64   `json:"size_320mp3"`
	SizeFlac int64   `json:"size_flac"`
	Size192A int64   `json:"size_192aac"`
	Size96A  int64   `json:"size_96aac"`
	Size48A  int64   `json:"size_48aac"`
	Size192O int64   `json:"size_192ogg"`
	Size96O  int64   `json:"size_96ogg"`
	SizeDTS  int64   `json:"size_dts"`
	SizeNew  []int64 `json:"size_new"`
}

// Pay 是付费权限信息。
type Pay struct {
	PayMonth int `json:"pay_month"`
	PayPlay  int `json:"pay_play"`
	PayDown  int `json:"pay_down"`
}

// MV 是 MV 摘要。
type MV struct {
	ID  int64  `json:"id"`
	Vid string `json:"vid"`
}

// Duration 返回歌曲时长。
func (s Song) Duration() time.Duration {
	return time.Duration(s.Interval) * time.Second
}

// CoverURL 返回专辑封面链接。
func (s Song) CoverURL(size int) string {
	return AlbumCoverURL(s.Album.Mid, size)
}

// AlbumCoverURL 按 QQ 音乐的 photo_new 规则拼接专辑封面。
func AlbumCoverURL(albumMid string, size int) string {
	if albumMid == "" {
		return ""
	}
	if size <= 0 {
		size = 300
	}
	return fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R%dx%dM000%s.jpg", size, size, albumMid)
}

// SingerCoverURL 按 photo_new 规则拼接歌手头像。
func SingerCoverURL(singerMid string, size int) string {
	if singerMid == "" {
		return ""
	}
	if size <= 0 {
		size = 300
	}
	return fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T001R%dx%dM000%s.jpg", size, size, singerMid)
}

// Playlist 是歌单摘要。
type Playlist struct {
	ID       int64  `json:"id"`
	DirID    int64  `json:"dirid"`
	Title    string `json:"title"`
	PicURL   string `json:"picurl"`
	Desc     string `json:"desc"`
	SongNum  int    `json:"songnum"`
	HostNick string `json:"host_nick"`
	HostUin  int64  `json:"host_uin"`
}

// TopCategory 是榜单分组。
type TopCategory struct {
	GroupID   int64        `json:"groupId"`
	GroupName string       `json:"groupName"`
	Toplist   []TopSummary `json:"toplist"`
}

// TopSummary 是单个榜单摘要。
type TopSummary struct {
	TopID       int64  `json:"topId"`
	Title       string `json:"title"`
	TitleDetail string `json:"titleDetail"`
	Intro       string `json:"intro"`
	Period      string `json:"period"`
	UpdateTime  string `json:"updateTime"`
	ListenNum   int64  `json:"listenNum"`
	TotalNum    int    `json:"totalNum"`
	FrontPicURL string `json:"frontPicUrl"`
}

// decode 将 data 反序列化到 out。
func decode(data json.RawMessage, out any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("qqmusic: 解析响应结构失败: %w", err)
	}
	return nil
}

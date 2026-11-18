package structs

import (
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/pkg/errors"
)

type Song struct {
	Id               int64         `json:"id"`
	Name             string        `json:"name"`
	Alg              string        `json:"alg"` // 每日推荐，私人FM回传使用
	Duration         time.Duration `json:"duration"`
	Artists          []Artist      `json:"artists"`
	Album            `json:"album"`
	DjRadioEpisodeId int64   `json:"djRadioEpisodeId"` // 若为播客，则非 0
	DjRadio          DjRadio `json:"djRadio"`          // 播客，电台使用
	UnMatched        bool    `json:"unMatched"`        // 云盘内资源匹配状态

	// Source 是歌曲所属渠道。空串等价于 SourceNetease，保证旧快照兼容。
	Source string `json:"source,omitempty"`
	// Mid 是 QQ 音乐等渠道的字符串主键（网易云不使用）。
	Mid string `json:"mid,omitempty"`
	// MediaMid 是 QQ 音乐播放链接所需的媒体标识，可能为空。
	MediaMid string `json:"mediaMid,omitempty"`
	// SongType 是 QQ 音乐的歌单写入类型（普通单曲为 0）。
	// 收藏/取消收藏时需原样回传，否则特殊曲目会写错。
	SongType int `json:"songType,omitempty"`
	// FileSizes 是各音质档位的文件大小声明（字节），仅 QQ 渠道填充。
	//
	// 键为 qqmusic.Quality.Key() 的稳定标识（如 "sq"、"sq_ogg"），
	// 值为该档位声明的字节数；0 或缺失表示该档位不可用。
	// 用于状态栏显示实际码率与文件大小，避免播放时额外发 HEAD 请求。
	//
	// 注意不能用显示名作键：SQ(FLAC) 与 SQ(OGG 640) 显示名相同，会冲突。
	FileSizes map[string]int64 `json:"fileSizes,omitempty"`
}

// 渠道标识。
const (
	// SourceNetease 是网易云音乐渠道（默认）。
	SourceNetease = "netease"
	// SourceQQ 是 QQ 音乐渠道。
	SourceQQ = "qq"
)

// EffectiveSource 返回规范化的渠道标识，空串归一为网易云。
func (s Song) EffectiveSource() string {
	if s.Source == "" {
		return SourceNetease
	}
	return s.Source
}

// IsQQ 报告该歌曲是否属于 QQ 音乐渠道。
func (s Song) IsQQ() bool {
	return s.Source == SourceQQ
}

func (s Song) ArtistName() string {
	var artistNames []string
	for _, artist := range s.Artists {
		artistNames = append(artistNames, artist.Name)
	}
	return strings.Join(artistNames, ",")
}

// NewSongFromShortNameSongsJson 从歌单获取数据
func NewSongFromShortNameSongsJson(json []byte) (Song, error) {
	var song Song
	if len(json) == 0 {
		return song, errors.New("json is empty")
	}

	id, err := jsonparser.GetInt(json, "id")
	if err != nil {
		return song, err
	}
	song.Id = id

	if name, err := jsonparser.GetString(json, "name"); err == nil {
		song.Name = name
	}
	if alg, err := jsonparser.GetString(json, "alg"); err == nil {
		song.Alg = alg
	}
	if duration, err := jsonparser.GetInt(json, "dt"); err == nil {
		song.Duration = time.Millisecond * time.Duration(duration)
	}

	if album, err := NewAlbumFromJson(json, "al"); err == nil {
		song.Album = album
	}

	_, _ = jsonparser.ArrayEach(json, func(value []byte, dataType jsonparser.ValueType, offset int, _ error) {
		artist, err := NewArtist(value)
		if err == nil {
			song.Artists = append(song.Artists, artist)
		}
	}, "ar")

	return song, nil
}

// NewSongFromCommonJson 从私人FM获取数据
func NewSongFromCommonJson(json []byte) (Song, error) {
	var song Song
	if len(json) == 0 {
		return song, errors.New("json is empty")
	}

	id, err := jsonparser.GetInt(json, "id")
	if err != nil {
		return song, err
	}
	song.Id = id

	if name, err := jsonparser.GetString(json, "name"); err == nil {
		song.Name = name
	}
	if alg, err := jsonparser.GetString(json, "alg"); err == nil {
		song.Alg = alg
	}
	if duration, err := jsonparser.GetInt(json, "duration"); err == nil {
		song.Duration = time.Millisecond * time.Duration(duration)
	}
	if album, err := NewAlbumFromJson(json, "album"); err == nil {
		song.Album = album
	}

	_, _ = jsonparser.ArrayEach(json, func(value []byte, dataType jsonparser.ValueType, offset int, _ error) {
		artist, err := NewArtist(value)

		if err == nil {
			song.Artists = append(song.Artists, artist)
		}
	}, "artists")

	return song, nil
}

// NewSongFromIntelligenceJson 心动模式获取数据
func NewSongFromIntelligenceJson(json []byte) (Song, error) {
	var song Song
	if len(json) == 0 {
		return song, errors.New("json is empty")
	}

	id, err := jsonparser.GetInt(json, "songInfo", "id")
	if err != nil {
		return song, err
	}
	song.Id = id

	if name, err := jsonparser.GetString(json, "songInfo", "name"); err == nil {
		song.Name = name
	}
	if duration, err := jsonparser.GetInt(json, "songInfo", "dt"); err == nil {
		song.Duration = time.Millisecond * time.Duration(duration)
	}
	if album, err := NewAlbumFromJson(json, "songInfo", "al"); err == nil {
		song.Album = album
	}

	_, _ = jsonparser.ArrayEach(json, func(value []byte, dataType jsonparser.ValueType, offset int, _ error) {
		artist, err := NewArtist(value)

		if err == nil {
			song.Artists = append(song.Artists, artist)
		}
	}, "songInfo", "ar")

	return song, nil
}

// NewSongFromAlbumSongsJson 从专辑获取数据
func NewSongFromAlbumSongsJson(json []byte) (Song, error) {
	return NewSongFromShortNameSongsJson(json)
}

// NewSongFromArtistSongsJson 从歌手获取数据
func NewSongFromArtistSongsJson(json []byte) (Song, error) {
	return NewSongFromShortNameSongsJson(json)
}

// NewSongFromDjRadioProgramJson 从DjRadio节目中获取数据
func NewSongFromDjRadioProgramJson(json []byte, keys ...string) (Song, error) {
	var song Song
	if len(json) == 0 {
		return song, errors.New("json is empty")
	}

	targetData := json
	if len(keys) > 0 {
		extractedData, _, _, err := jsonparser.Get(json, keys...)
		if err != nil {
			return song, err
		}
		targetData = extractedData
	}

	id, err := jsonparser.GetInt(json, "mainSong", "id")
	if err != nil {
		return song, err
	}
	song.Id = id

	if name, err := jsonparser.GetString(targetData, "mainSong", "name"); err == nil {
		song.Name = name
	}
	if duration, err := jsonparser.GetInt(targetData, "mainSong", "duration"); err == nil {
		song.Duration = time.Millisecond * time.Duration(duration)
	}
	if album, err := NewAlbumFromJson(targetData, "mainSong", "album"); err == nil {
		song.Album = album
	}

	if episodeId, err := jsonparser.GetInt(targetData, "id"); err == nil {
		song.DjRadioEpisodeId = episodeId
	}

	if radio, err := NewDjRadioFromJson(targetData, "radio"); err == nil {
		song.DjRadio = radio
	}
	if dj, err := NewUserFromJson(targetData, "dj"); err == nil {
		song.DjRadio.Dj = dj
	}

	var artist Artist
	// artist.Id = song.Dj.UserId
	artist.Name = song.DjRadio.Dj.Nickname
	song.Artists = append(song.Artists, artist)

	return song, nil
}

// NewSongFromDjRankProgramJson 从DjRadio节目中获取数据
func NewSongFromDjRankProgramJson(json []byte) (Song, error) {
	return NewSongFromDjRadioProgramJson(json, "program")
}

// NewSongFromCloudJson 从云盘中获取数据
func NewSongFromCloudJson(json []byte) (Song, error) {
	var song Song
	if len(json) == 0 {
		return song, errors.New("json is empty")
	}

	id, err := jsonparser.GetInt(json, "songId")
	if err != nil {
		return song, err
	}
	song.Id = id

	if name, err := jsonparser.GetString(json, "songName"); err == nil {
		song.Name = name
	}
	if duration, err := jsonparser.GetInt(json, "simpleSong", "dt"); err == nil {
		song.Duration = time.Millisecond * time.Duration(duration)
	}
	if album, err := NewAlbumFromJson(json, "simpleSong", "al"); err == nil {
		song.Album = album
	}

	if matchType, err := jsonparser.GetString(json, "matchType"); err == nil {
		song.UnMatched = (matchType == "unmatched")
	}

	_, _ = jsonparser.ArrayEach(json, func(value []byte, dataType jsonparser.ValueType, offset int, _ error) {
		artist, err := NewArtist(value)

		if err == nil {
			song.Artists = append(song.Artists, artist)
		}
	}, "simpleSong", "ar")

	return song, nil
}

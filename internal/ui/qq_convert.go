package ui

import (
	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/go-musicfox/internal/structs"
)

// qqClient 是 QQ 音乐渠道的全局客户端。
//
// 生命周期与 Netease 实例一致，在 NewNetease 中创建。
type qqClient struct {
	*qqmusic.Client
}

// newQQClient 创建 QQ 音乐客户端。
func newQQClient() *qqClient {
	return &qqClient{Client: qqmusic.NewClient()}
}

// toStructSong 将 QQ 音乐歌曲转换为 musicfox 的通用歌曲结构。
//
// 关键转换：
//   - Source 标记为 qq，使 track 层路由到 QQ 渠道；
//   - Mid / MediaMid 写入，播放链接与歌词都依赖它们；
//   - Duration 由秒转 time.Duration；
//   - Album.PicUrl 按 QQ 的 photo_new 规则拼接。
func toStructSong(s qqmusic.Song) structs.Song {
	artists := make([]structs.Artist, 0, len(s.Singer))
	for _, singer := range s.Singer {
		artists = append(artists, structs.Artist{
			Id:   singer.ID,
			Name: singer.Name,
		})
	}

	name := s.Name
	if name == "" {
		name = s.Title
	}

	return structs.Song{
		Id:       s.ID,
		Name:     name,
		Duration: s.Duration(),
		Artists:  artists,
		Album: structs.Album{
			Id:     s.Album.ID,
			Name:   s.Album.Name,
			PicUrl: s.CoverURL(300),
		},
		Source:    structs.SourceQQ,
		Mid:       s.Mid,
		MediaMid:  s.File.MediaMid,
		SongType:  s.Type,
		FileSizes: collectFileSizes(s.File),
	}
}

// collectFileSizes 把 QQ 歌曲各音质档位声明的文件大小转成通用 map。
//
// 键用 qqmusic.Quality.Key()（稳定唯一标识），不能用 String()：
// SQ(FLAC) 与 SQ(OGG 640) 的显示名都是「SQ」，用显示名会键冲突，
// 导致其中一档的真实大小被另一档的 0 值覆盖。
//
// 实测声明值与实际下载大小完全一致，因此可直接用于显示，
// 无需播放时再发 HEAD 请求。
func collectFileSizes(f qqmusic.File) map[string]int64 {
	sizes := map[string]int64{
		qqmusic.QualityStandard.Key(): f.Size128,
		qqmusic.QualityHQ.Key():       f.Size320,
		qqmusic.QualitySQ.Key():       f.SizeFlac,
	}
	// O801 对应 size_new[5]、母带对应 size_new[0]（实测确认）。
	if len(f.SizeNew) > 5 {
		sizes[qqmusic.QualityOGG640.Key()] = f.SizeNew[5]
	}
	if len(f.SizeNew) > 0 {
		sizes[qqmusic.QualityMaster.Key()] = f.SizeNew[0]
	}

	// 清掉未声明的档位，避免显示成 0 字节。
	for k, v := range sizes {
		if v <= 0 {
			delete(sizes, k)
		}
	}
	return sizes
}

// toStructSongs 批量转换。
func toStructSongs(songs []qqmusic.Song) []structs.Song {
	out := make([]structs.Song, 0, len(songs))
	for _, s := range songs {
		out = append(out, toStructSong(s))
	}
	return out
}

// toStructPlaylist 将 QQ 音乐歌单转换为通用歌单结构。
func toStructPlaylist(p qqmusic.Playlist) structs.Playlist {
	return structs.Playlist{
		Id:   p.ID,
		Name: p.Title,
		Creator: structs.User{
			UserId:   p.HostUin,
			Nickname: p.HostNick,
		},
	}
}

// toStructPlaylists 批量转换歌单。
func toStructPlaylists(playlists []qqmusic.Playlist) []structs.Playlist {
	out := make([]structs.Playlist, 0, len(playlists))
	for _, p := range playlists {
		out = append(out, toStructPlaylist(p))
	}
	return out
}

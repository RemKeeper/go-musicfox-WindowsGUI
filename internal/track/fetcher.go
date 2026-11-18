package track

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/netease"
	"github.com/go-musicfox/netease-music/service"
	"github.com/pkg/errors"
)

// Fetcher 是渠道无关的音源获取接口。
type Fetcher interface {
	FetchPlayableInfo(ctx context.Context, song structs.Song) (*PlayableInfo, error)
	FetchStream(ctx context.Context, source PlayableSource) (io.ReadCloser, error)
	FetchLyric(ctx context.Context, song structs.Song) (structs.LRCData, error)
	FetchCloudLyric(ctx context.Context, userID, songID int64) (structs.LRCData, error)
}

// QQMusicFetcher 是 QQ 音乐渠道的音源获取实现。
type QQMusicFetcher interface {
	FetchPlayableInfo(song qqmusic.Song, quality qqmusic.Quality) (*qqmusic.PlayableInfo, error)
	FetchLyric(songMid string) (structs.LRCData, error)
}

type fetcher struct {
	httpClient *http.Client
	quality    service.SongQualityLevel
	qq         QQMusicFetcher
}

// Option 配置 fetcher。
type Option func(*fetcher)

// NewFetcher 创建渠道感知的 fetcher。
func NewFetcher(opts ...Option) Fetcher {
	f := &fetcher{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		quality:    service.Standard,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// WithFetcherHTTPClient 设置 HTTP 客户端。
func WithFetcherHTTPClient(client *http.Client) Option {
	return func(f *fetcher) {
		f.httpClient = client
	}
}

// WithFetcherSongQuality 设置音质。
func WithFetcherSongQuality(quality service.SongQualityLevel) Option {
	return func(f *fetcher) {
		f.quality = quality
	}
}

// withQQMusicFetcher 是内部 Option，把 QQ 渠道实现注入 fetcher。
func withQQMusicFetcher(qq QQMusicFetcher) Option {
	return func(f *fetcher) {
		f.qq = qq
	}
}

// FetchPlayableInfo 按歌曲所属渠道获取播放信息。
func (f *fetcher) FetchPlayableInfo(ctx context.Context, song structs.Song) (*PlayableInfo, error) {
	if song.IsQQ() {
		return f.fetchQQPlayableInfo(song)
	}

	info, err := netease.FetchPlayableInfo(song.Id, f.quality)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch playable info for song %d: %w", song.Id, err)
	}
	return &PlayableInfo{
		URL:       info.URL,
		MusicType: info.MusicType,
		Size:      info.Size,
		Quality:   neteaseQualityLabel(f.quality, info.MusicType),
	}, nil
}

// fetchQQPlayableInfo 获取 QQ 音乐渠道的播放信息。
func (f *fetcher) fetchQQPlayableInfo(song structs.Song) (*PlayableInfo, error) {
	if f.qq == nil {
		return nil, errors.New("qqmusic: 渠道未初始化")
	}
	if song.Mid == "" {
		return nil, fmt.Errorf("qqmusic: 歌曲 %d 缺少 mid", song.Id)
	}

	qqSong := qqmusic.Song{
		ID:   song.Id,
		Mid:  song.Mid,
		File: qqmusic.File{MediaMid: song.MediaMid},
	}

	info, err := f.qq.FetchPlayableInfo(qqSong, qqQualityFrom(f.quality))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch qq playable info for song %d: %w", song.Id, err)
	}

	// 实际音质标签：QQ 渠道会降级，用返回的真实档位而非请求值。
	label := qqQualityLabel(info.Quality, info.MusicType)

	// 文件大小来自歌曲元数据的声明值（与实际下载完全一致）。
	// 用 Quality.Key() 查表：显示标签「SQ」同时对应 FLAC 与 OGG640 两档，
	// 只有 Key 能唯一确定实际编码。
	size := info.Size
	if size <= 0 && song.FileSizes != nil {
		size = song.FileSizes[info.Quality.Key()]
	}

	return &PlayableInfo{
		URL:       info.URL,
		MusicType: info.MusicType,
		Size:      size,
		Quality:   label,
	}, nil
}

// FetchStream 拉取音频流。
func (f *fetcher) FetchStream(ctx context.Context, source PlayableSource) (io.ReadCloser, error) {
	if source.Info == nil || source.Info.URL == "" {
		return nil, fmt.Errorf("no playable url for song %d", source.Id)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.Info.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request for song %d: %w", source.Id, err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get failed for song %d (%s): %w", source.Id, source.Info.URL, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("invalid http status for song %d: %s", source.Id, resp.Status)
	}
	return resp.Body, nil
}

// FetchLyric 按渠道获取歌词。
func (f *fetcher) FetchLyric(ctx context.Context, song structs.Song) (structs.LRCData, error) {
	if song.IsQQ() {
		if f.qq == nil {
			return structs.LRCData{}, errors.New("qqmusic: 渠道未初始化")
		}
		if song.Mid == "" {
			return structs.LRCData{}, fmt.Errorf("qqmusic: 歌曲 %d 缺少 mid", song.Id)
		}
		data, err := f.qq.FetchLyric(song.Mid)
		if err != nil {
			return structs.LRCData{}, errors.Wrapf(err, "failed to fetch qq lyric for song %d", song.Id)
		}
		return data, nil
	}

	lrcData, err := netease.FetchLyric(song.Id)
	if err != nil {
		return structs.LRCData{}, errors.Wrapf(err, "failed to fetch lyric for song %d", song.Id)
	}
	return lrcData, nil
}

// FetchCloudLyric 获取云盘歌曲内嵌歌词（仅网易云有云盘概念）。
func (f *fetcher) FetchCloudLyric(ctx context.Context, userID, songID int64) (structs.LRCData, error) {
	lrcData, err := netease.FetchCloudLyric(userID, songID)
	if err != nil {
		return structs.LRCData{}, errors.Wrapf(err, "failed to fetch cloud lyric for song %d", songID)
	}
	return lrcData, nil
}

// qqQualityLabel 把 QQ 音质档位转成状态栏显示的简短标签。
//
// 优先用实际拿到的编码（MusicType）判断，因为它反映真实结果：
// 降级链可能把 SQ 请求回退成 ogg 或 mp3。
//
// 档位归属（实测码率）：
//   - O801 OGG 640 约 450kbps，高于 M800 的 265kbps，归入 SQ 档
//   - O800 OGG 320 与 M800 同属 HQ 档，标注编码以便区分
func qqQualityLabel(q qqmusic.Quality, musicType string) string {
	switch musicType {
	case "flac":
		// 区分普通无损与臻品母带
		if q == qqmusic.QualityMaster {
			return "臻品母带"
		}
		return "无损"
	case "ogg":
		if q == qqmusic.QualityOGG640 {
			// 码率高于 M800，属于 SQ 档；不标注编码避免误认为有损降级。
			return "SQ"
		}
		return "HQ(OGG)"
	case "mp3":
		if q == qqmusic.QualityHQ {
			return "HQ"
		}
		return "标准"
	default:
		if musicType != "" {
			return strings.ToUpper(musicType)
		}
		return q.String()
	}
}

// neteaseQualityLabel 返回网易云渠道实际拿到的音质标签。
//
// 网易云的 FetchPlayableInfo 按请求音质返回，无降级，
// 因此直接用请求值；MusicType 仅用于兜底。
func neteaseQualityLabel(level service.SongQualityLevel, musicType string) string {
	switch level {
	case service.Lossless:
		return "无损"
	case service.Hires:
		return "Hi-Res"
	case service.Exhigh:
		return "极高"
	case service.Higher:
		return "较高"
	case service.Standard:
		return "标准"
	default:
		if musicType != "" {
			return strings.ToUpper(musicType)
		}
		return string(level)
	}
}

// qqQualityFrom 将网易云音质枚举映射为 QQ 音质档位。
//
// 映射策略：只要用户设置不低于 HQ，就优先尝试最高可用音质。
//
// 实测「我喜欢」里 10 首有 9 首 F000 可用（账号可免费听无损），
// 而原先把 HQ 映射为 M800 会让用户只拿到 MP3 320k。
// QQ 侧自带降级链，不可用时自动回退到 OGG 640k / MP3，因此
// 这里可以放心地请求最高可用音质。
//
// 注意：必须覆盖 QQ 音乐特有的三档（jyeffect/sky/jymaster），
// 否则会落到 default 只请求标准音质——这正是「几首歌都是标准」的原因。
func qqQualityFrom(level service.SongQualityLevel) qqmusic.Quality {
	switch level {
	case service.JYMaster:
		// 超清母带：请求臻品母带（AI00）。
		return qqmusic.QualityMaster
	case service.JYEffect, service.Sky:
		// 高清环绕声 / 沉浸环绕声：QQ 侧无对应编码，
		// 用无损 FLAC 作为最高可用档位。
		return qqmusic.QualitySQ
	case service.Lossless, service.Hires, service.Exhigh, service.Higher:
		// 无损及以上：先试 FLAC，不可用时降级链自动回退。
		return qqmusic.QualitySQ
	case service.Standard:
		return qqmusic.QualityStandard
	default:
		// 未知档位按无损处理，避免因配置值拼写差异而静默降级。
		return qqmusic.QualitySQ
	}
}

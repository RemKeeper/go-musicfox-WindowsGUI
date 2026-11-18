package qqmusic

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Quality 是 QQ 音乐的音质档位。
type Quality uint8

const (
	QualityStandard Quality = iota // 标准 MP3 128k
	QualityHQ                      // HQ MP3 320k
	QualitySQ                      // SQ 无损 FLAC
	QualityOGG320                  // HQ OGG 320k
	QualityOGG640                  // SQ OGG 640k
	QualityMaster                  // 臻品母带 FLAC
)

// fileType 是音质对应的文件编码前缀与扩展名。
type fileType struct {
	prefix string
	ext    string
}

// fileTypes 映射音质到 QQ 音乐的文件编码（对齐 QQMusicApi modules/song.py:SongFileType）。
var fileTypes = map[Quality]fileType{
	QualityStandard: {"M500", ".mp3"},
	QualityHQ:       {"M800", ".mp3"},
	QualitySQ:       {"F000", ".flac"},
	QualityOGG320:   {"O800", ".ogg"},
	QualityOGG640:   {"O801", ".ogg"},
	QualityMaster:   {"AI00", ".flac"},
}

// Ext 返回该音质的文件扩展名（不含点）。
func (q Quality) Ext() string {
	if ft, ok := fileTypes[q]; ok {
		return strings.TrimPrefix(ft.ext, ".")
	}
	return "mp3"
}

// Key 返回该音质的稳定唯一标识，用于 map 键与持久化。
//
// 不能用 String()：QualitySQ 与 QualityOGG640 的显示名都是「SQ」
// （两者实际码率相近，对用户展示一致），但它们对应不同的文件编码，
// 大小也不同。混用会导致键冲突。
func (q Quality) Key() string {
	switch q {
	case QualityStandard:
		return "standard"
	case QualityHQ:
		return "hq"
	case QualitySQ:
		return "sq"
	case QualityOGG320:
		return "hq_ogg"
	case QualityOGG640:
		return "sq_ogg"
	case QualityMaster:
		return "master"
	default:
		return "unknown"
	}
}

// String 返回音质名称。
//
// 命名对齐 QQ 音乐客户端的档位展示：
//   - 单曲无无损时，O801(OGG 640) 的实际码率（约 450kbps）高于
//     M800(MP3 320k)，因此显示为「SQ」而非「SQ(OGG)」——
//     后者会让用户误以为是有损降级。
//   - O800 是 OGG 320，与 M800 同属 HQ 档，标注编码以区分。
func (q Quality) String() string {
	switch q {
	case QualityStandard:
		return "标准"
	case QualityHQ:
		return "HQ"
	case QualitySQ:
		return "SQ"
	case QualityOGG320:
		return "HQ(OGG)"
	case QualityOGG640:
		return "SQ"
	case QualityMaster:
		return "臻品母带"
	default:
		return "未知"
	}
}

// PlayableInfo 是歌曲的可播放信息。
type PlayableInfo struct {
	URL       string
	MusicType string
	Size      int64
	Quality   Quality
}

// cdnCache 缓存 CDN 域名列表，避免每次播放都请求调度接口。
type cdnCache struct {
	mu   sync.Mutex
	sips []string
	idx  int
}

// next 返回下一个可用的 CDN 域名。
func (c *cdnCache) next() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.sips) == 0 {
		return ""
	}
	s := c.sips[c.idx%len(c.sips)]
	c.idx++
	return s
}

// ensure 在缓存为空时拉取 CDN 域名列表。
func (c *cdnCache) ensure(client *Client) error {
	c.mu.Lock()
	has := len(c.sips) > 0
	c.mu.Unlock()
	if has {
		return nil
	}

	sips, err := client.GetCdnDispatch()
	if err != nil {
		return err
	}
	if len(sips) == 0 {
		return errors.New("qqmusic: CDN 调度返回空列表")
	}

	c.mu.Lock()
	c.sips = sips
	c.mu.Unlock()
	return nil
}

// FetchPlayableInfo 获取歌曲的播放链接。
//
// 该接口必须登录；未登录时返回 ErrNotLoggedIn。
// 若请求的音质不可用（版权/会员限制），会自动降级到更低音质重试。
func (c *Client) FetchPlayableInfo(song Song, quality Quality) (*PlayableInfo, error) {
	if !c.LoggedIn() {
		return nil, ErrNotLoggedIn
	}
	if song.Mid == "" {
		return nil, fmt.Errorf("qqmusic: 歌曲缺少 mid")
	}

	// media_mid 缺失时回退用歌曲 mid（部分歌曲两者相同）。
	mediaMid := song.File.MediaMid
	if mediaMid == "" {
		mediaMid = song.Mid
	}

	if err := c.cdn.ensure(c); err != nil {
		return nil, err
	}

	// 两层循环：先按音质优先级，再按 ctx 取值。
	//
	// ctx 是 vkey 的「播放场景」参数，实测对部分歌曲是决定性的：
	// 《告白》（mid=002lAHwM16lrPS）在 ctx=0 时所有编码都返回 104003，
	// 但 ctx=1/2 能正常拿到 purl 并下载到完整音频。
	// 官方客户端会按场景传不同 ctx，因此这里逐个尝试。
	lastCode := 0
	for _, q := range qualityFallbacks(quality) {
		ft, ok := fileTypes[q]
		if !ok {
			continue
		}
		// 该音质文件大小声明为 0 表示不存在，跳过。
		if !songHasQuality(song, q) {
			continue
		}

		// filename 用 media_mid 拼接。
		//
		// 这是关键：QQ 的音频文件名基于 media_mid 而非歌曲 mid，
		// 两者常常不同（如晴天 songmid=0039MnYb0qxYhV、
		// media_mid=0049m3up3X6Q8s）。用错会得到 result=104003。
		filename := ft.prefix + mediaMid + ft.ext

		for _, ctx := range vkeyContexts {
			info, err := c.requestVKey(song.Mid, filename, q, ctx)
			if err != nil {
				var apiErr *APIError
				if errors.As(err, &apiErr) {
					lastCode = apiErr.Code
				}
				continue
			}
			if info != nil && info.URL != "" {
				// 补上该档位的声明大小，供上层显示码率与文件大小。
				info.Size = declaredSize(song, info.Quality)
				return info, nil
			}
		}
	}

	// 全部编码都无链接：按业务码给出可诊断的错误。
	switch lastCode {
	case CodeNoPermission:
		return nil, ErrCopyrightRestricted
	case CodeSongNotExist:
		return nil, fmt.Errorf("qqmusic: 歌曲《%s》已下架或不存在", song.Name)
	default:
		return nil, ErrNoPlayableURL
	}
}

// vkeyContexts 是 vkey 请求依次尝试的 ctx（播放场景）取值。
//
// 实测 ctx 对部分歌曲是决定性的：《告白》在 ctx=0 时所有编码都返回
// 104003，ctx=1/2 能正常拿到 purl。官方客户端按场景传不同值，
// 因此这里按「默认 → 场景 1 → 场景 2」顺序尝试。
var vkeyContexts = []int{0, 1, 2}

// requestVKey 请求单个音质的播放链接。
func (c *Client) requestVKey(songMid, filename string, quality Quality, ctx int) (*PlayableInfo, error) {
	cred := c.Credential()

	data, err := c.Do(cgiRequest{
		Module: "music.vkey.GetVkey",
		Method: "UrlGetVkey",
		// vkey 必须走 Android 平台：Web 平台即使带 uin/g_tk 也拿不到 purl。
		Android: true,
		Param: map[string]any{
			// uin 必须用 str_musicid（字符串形式），用整数会让接口返回空 purl。
			"uin":      cred.Uin(),
			"filename": []string{filename},
			// guid 必须与 comm 里的 OpenUDID 一致，否则风控判定设备不可信，
			// 返回 retcode=104009 (invalidq)。
			"guid":     c.deviceGUID(),
			"songmid":  []string{songMid},
			"songtype": []int{0},
			"ctx":      ctx,
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		RetCode    int      `json:"retcode"`
		Sip        []string `json:"sip"`
		MidURLInfo []struct {
			Purl    string `json:"purl"`
			Result  int    `json:"result"`
			Vkey    string `json:"vkey"`
			ErrType string `json:"errtype"`
		} `json:"midurlinfo"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}

	if len(parsed.MidURLInfo) == 0 {
		return nil, ErrNoPlayableURL
	}
	item := parsed.MidURLInfo[0]
	if item.Purl == "" {
		return nil, &APIError{Code: item.Result}
	}

	// purl 是相对路径，需要拼接 CDN 域名。
	base := ""
	if len(parsed.Sip) > 0 {
		base = parsed.Sip[0]
	} else {
		base = c.cdn.next()
	}
	if base == "" {
		base = "https://isure.stream.qqmusic.qq.com/"
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	return &PlayableInfo{
		URL:       base + strings.TrimPrefix(item.Purl, "/"),
		MusicType: quality.Ext(),
		Quality:   quality,
	}, nil
}

// declaredSize 返回歌曲元数据里声明的该音质文件大小（字节）。
//
// 实测声明值与实际下载大小完全一致（如《鱼玄机》M500=3524501、
// M800=8810849、O801=15117114），因此无需额外 HEAD 请求，
// 直接用元数据即可，避免播放时多一次网络往返。
//
// 返回 0 表示该音质未声明大小（不可用）。
func declaredSize(song Song, q Quality) int64 {
	f := song.File
	switch q {
	case QualityStandard:
		return f.Size128
	case QualityHQ:
		return f.Size320
	case QualitySQ:
		return f.SizeFlac
	case QualityOGG320:
		// O800 对应 size_new 的索引未在文档中明确，
		// 实测该编码多数歌曲返回 404，这里不做推断。
		return 0
	case QualityOGG640:
		// O801 对应 size_new[5]（实测《鱼玄机》15117114 精确匹配）。
		if len(f.SizeNew) > 5 {
			return f.SizeNew[5]
		}
		return 0
	case QualityMaster:
		// AI00 臻品母带对应 size_new[0]。
		if len(f.SizeNew) > 0 {
			return f.SizeNew[0]
		}
		return 0
	default:
		return 0
	}
}

// qualityFallbacks 返回从指定音质开始的降级序列。
//
// 实测：QQ 音乐对不同编码的权限判定是独立的。同一首歌可能 M500/M800
// 全部返回 104003（无权限），但 F000（FLAC 无损）或 O801（SQ OGG 640k）
// 能拿到 purl。因此降级链必须覆盖全部编码。
//
// 排序原则：**无损优先**，且母带必须紧邻 FLAC。
// 实测「我喜欢」里 10 首有 9 首 F000 可用，而 F000 是无损 FLAC，
// 音质优于 OGG 640k。若把 O801 排在 FLAC 前面，会白白降级到有损编码。
func qualityFallbacks(q Quality) []Quality {
	// 完整尝试顺序：从请求的音质开始，再按「母带 → 无损 → 高码率有损」遍历。
	//
	// Master 必须紧跟 SQ：请求母带但歌曲未声明母带时，应立即回退到
	// FLAC，而不是跳到 OGG 640k（那是有损编码）。
	all := []Quality{
		QualityMaster, // AI00，臻品母带（仅在有声明时尝试）
		QualitySQ,     // F000，无损 FLAC
		QualityOGG640, // O801，SQ OGG 640k
		QualityHQ,     // M800，MP3 320k
		QualityOGG320, // O800，HQ OGG 320k
		QualityStandard,
	}

	start := 0
	for i, item := range all {
		if item == q {
			start = i
			break
		}
	}

	// 从 start 开始绕一圈，保证请求的音质优先，其余作为兜底。
	out := make([]Quality, 0, len(all))
	out = append(out, all[start:]...)
	out = append(out, all[:start]...)
	return out
}

// songHasQuality 判断歌曲是否声明了该音质的文件。
//
// 只用于「跳过明显不存在的音质」以省一次请求，不能作为权限判断依据：
// 实测 size 字段与实际可播放权限并不一致，且同一首歌可能只有 OGG
// 编码可用。因此除母带外一律放行，由服务端裁决。
func songHasQuality(song Song, q Quality) bool {
	f := song.File
	switch q {
	case QualityMaster:
		// 母带体积大且绝大多数账号无权限，仅在明确声明时才尝试。
		return len(f.SizeNew) > 0 && f.SizeNew[0] > 0
	default:
		// 其余编码交由服务端裁决：size 字段不可靠，
		// 提前跳过会漏掉唯一可用的编码。
		return true
	}
}

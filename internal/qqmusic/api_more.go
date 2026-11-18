package qqmusic

import "fmt"

// GetRecommendSonglists 获取推荐歌单。
func (c *Client) GetRecommendSonglists(page, num int) ([]Playlist, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 30
	}

	data, err := c.Do(cgiRequest{
		Module: "music.playlist.PlaylistSquare",
		Method: "GetRecommendFeed",
		Param: map[string]any{
			"From": num * (page - 1),
			"Size": num,
		},
	})
	if err != nil {
		return nil, err
	}

	// 响应结构为 `List[].Playlist.basic`，用中间结构承接。
	//
	// 字段名易错点（实测确认）：
	//   - 歌单 ID 必须用 `tid`，不能用 `dirid`。
	//     `dirid` 是推荐位的内部目录号（如 92），传给详情接口会静默返回空列表；
	//     `tid` 才是真正的歌单 ID（如 9082901147），详情接口只认它。
	//   - 封面在 `cover.default_url`
	var parsed struct {
		HasMore bool `json:"HasMore"`
		List    []struct {
			Playlist struct {
				Basic struct {
					Tid   int64  `json:"tid"`
					DirID int64  `json:"dirid"`
					Title string `json:"title"`
					Desc  string `json:"desc"`
					Cover struct {
						DefaultURL string `json:"default_url"`
					} `json:"cover"`
					SongCnt int   `json:"song_cnt"`
					PlayCnt int64 `json:"play_cnt"`
					Creator struct {
						Nick string `json:"nick"`
						Uin  string `json:"uin"`
					} `json:"creator"`
				} `json:"basic"`
			} `json:"Playlist"`
		} `json:"List"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}

	out := make([]Playlist, 0, len(parsed.List))
	for _, item := range parsed.List {
		b := item.Playlist.Basic
		// 优先 tid；个别条目可能只给 dirid，此时退回 dirid 保证不为 0。
		id := b.Tid
		if id == 0 {
			id = b.DirID
		}
		out = append(out, Playlist{
			ID:       id,
			DirID:    b.DirID,
			Title:    b.Title,
			PicURL:   b.Cover.DefaultURL,
			Desc:     b.Desc,
			SongNum:  b.SongCnt,
			HostNick: b.Creator.Nick,
		})
	}
	return out, nil
}

// GetCreatedSonglists 获取用户自建歌单。
func (c *Client) GetCreatedSonglists(uin int64) ([]Playlist, error) {
	data, err := c.Do(cgiRequest{
		Module: "music.musicasset.PlaylistBaseRead",
		Method: "GetPlaylistByUin",
		Param: map[string]any{
			"uin": fmt.Sprintf("%d", uin),
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Playlists []struct {
			ID      int64  `json:"tid"`
			DirID   int64  `json:"dirId"`
			Title   string `json:"dirName"`
			PicURL  string `json:"bigpicUrl"`
			SongNum int    `json:"songNum"`
			Nick    string `json:"nick"`
			Uin     string `json:"uin"`
		} `json:"v_playlist"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}

	out := make([]Playlist, 0, len(parsed.Playlists))
	for _, p := range parsed.Playlists {
		out = append(out, Playlist{
			ID:       p.ID,
			DirID:    p.DirID,
			Title:    p.Title,
			PicURL:   p.PicURL,
			SongNum:  p.SongNum,
			HostNick: p.Nick,
		})
	}
	return out, nil
}

// GetFavSongs 获取「我喜欢」歌单的歌曲（dirid 固定为 201）。
func (c *Client) GetFavSongs(uin int64, page, num int) ([]Song, int, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 100
	}

	data, err := c.Do(cgiRequest{
		Module: "music.srfDissInfo.DissInfo",
		Method: "CgiGetDiss",
		Param: map[string]any{
			"disstid":      0,
			"dirid":        201,
			"tag":          true,
			"song_begin":   num * (page - 1),
			"song_num":     num,
			"userinfo":     true,
			"orderlist":    true,
			"onlysonglist": false,
			"enc_host_uin": fmt.Sprintf("%d", uin),
		},
	})
	if err != nil {
		return nil, 0, err
	}

	var parsed struct {
		Songs        []Song `json:"songlist"`
		TotalSongNum int    `json:"total_song_num"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, 0, err
	}
	return parsed.Songs, parsed.TotalSongNum, nil
}

// GetSingerList 获取歌手列表（按热度）。
func (c *Client) GetSingerList(page, num int) ([]Singer, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 80
	}

	data, err := c.Do(cgiRequest{
		Module: "music.musichallSinger.SingerList",
		Method: "GetSingerListIndex",
		Param: map[string]any{
			"area":     -100,
			"sex":      -100,
			"genre":    -100,
			"index":    -100,
			"sin":      num * (page - 1),
			"cur_page": page,
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		SingerList []struct {
			SingerID   int64  `json:"singer_id"`
			SingerMid  string `json:"singer_mid"`
			SingerName string `json:"singer_name"`
		} `json:"singerlist"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}

	out := make([]Singer, 0, len(parsed.SingerList))
	for _, s := range parsed.SingerList {
		out = append(out, Singer{
			ID:   s.SingerID,
			Mid:  s.SingerMid,
			Name: s.SingerName,
		})
	}
	return out, nil
}

// GuessRadioID 是「猜你喜欢」电台的固定 ID。
//
// 实测该 ID 无需用户指定，服务端按账号听歌画像返回推荐。
const GuessRadioID = 99

// GetGuessRecommend 获取「猜你喜欢」推荐歌曲，对应网易云的心动模式。
//
// 关键特性（实测）：每次调用返回的歌单**都不同**，服务端按听歌画像
// 实时挑选，无需客户端传种子歌曲。这与 GetRadarRecommend 的
// 稳定分页不同，因此适合做「无限随机推送」。
//
// num 为本次请求的歌曲数，服务端单次上限约 5 首，传更大值也只返回 5 首。
func (c *Client) GetGuessRecommend(num int) ([]Song, error) {
	if num <= 0 {
		num = 5
	}
	// 服务端单次最多返回 5 首，超出无意义。
	if num > 5 {
		num = 5
	}

	data, err := c.Do(cgiRequest{
		Module: "music.radioProxy.MbTrackRadioSvr",
		Method: "get_radio_track",
		// 电台接口必须走 Android 平台，否则需要额外凭证。
		Android: true,
		Param: map[string]any{
			"id":       GuessRadioID,
			"num":      num,
			"from":     0,
			"scene":    0,
			"song_ids": []any{},
		},
	})
	if err != nil {
		return nil, err
	}

	// 响应结构：{"id":99,"name":"猜你喜欢","tracks":[Song...]}
	var parsed struct {
		Name   string `json:"name"`
		Tracks []Song `json:"tracks"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}
	return parsed.Tracks, nil
}

// GetSimilarSongs 获取与指定歌曲相似的推荐，对应网易云的「心动模式」。
//
// 与「猜你喜欢」的区别：
//   - 猜你喜欢：全局随机，与当前播放的歌曲无关
//   - 相似推荐：以 songID 为种子，返回风格相近的歌曲，
//     适合在某个歌单里持续播放同风格音乐
//
// 实测响应结构为 {"retcode":0,"vecSong":[{"track":Song}...]}，
// 单次返回约 20 首。
func (c *Client) GetSimilarSongs(songID int64) ([]Song, error) {
	if songID == 0 {
		return nil, fmt.Errorf("qqmusic: 相似推荐需要有效的歌曲 ID")
	}

	data, err := c.Do(cgiRequest{
		Module:  "music.recommend.TrackRelationServer",
		Method:  "GetSimilarSongs",
		Android: true,
		Param:   map[string]any{"songid": songID},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		RetCode int `json:"retcode"`
		VecSong []struct {
			Track Song `json:"track"`
		} `json:"vecSong"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}
	if parsed.RetCode != 0 {
		return nil, &APIError{Code: parsed.RetCode, Msg: "相似推荐失败"}
	}

	out := make([]Song, 0, len(parsed.VecSong))
	for _, item := range parsed.VecSong {
		// 过滤掉无效条目，避免下游拿空 mid 去请求播放链接。
		if item.Track.Mid == "" {
			continue
		}
		out = append(out, item.Track)
	}
	return out, nil
}

// GetRadarRecommend 获取「雷达推荐」歌曲，按页稳定返回。
//
// 与 GetGuessRecommend 的区别：雷达推荐支持稳定分页（同一 Page 返回
// 同一批歌），适合「查看更多」；猜你喜欢每次都是新的随机结果。
func (c *Client) GetRadarRecommend(page int) ([]Song, bool, error) {
	if page <= 0 {
		page = 1
	}

	data, err := c.Do(cgiRequest{
		Module:  "music.recommend.TrackRelationServer",
		Method:  "GetRadarSong",
		Android: true,
		Param: map[string]any{
			"Page":          page,
			"ReqType":       0,
			"FavSongs":      []any{},
			"EntranceSongs": []any{},
		},
	})
	if err != nil {
		return nil, false, err
	}

	// 响应结构：{"VecSongs":[{"Track":Song}...],"HasMore":true}
	var parsed struct {
		HasMore  bool `json:"HasMore"`
		VecSongs []struct {
			Track Song `json:"Track"`
		} `json:"VecSongs"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, false, err
	}

	out := make([]Song, 0, len(parsed.VecSongs))
	for _, item := range parsed.VecSongs {
		out = append(out, item.Track)
	}
	return out, parsed.HasMore, nil
}

package qqmusic

import (
	"fmt"
)

// SearchType 是搜索类型。
type SearchType int

const (
	SearchSong     SearchType = 0
	SearchSinger   SearchType = 1
	SearchAlbum    SearchType = 2
	SearchSonglist SearchType = 3
	SearchMV       SearchType = 4
)

// SearchResult 是分类搜索结果。
type SearchResult struct {
	Songs     []Song
	Singers   []Singer
	Albums    []Album
	Playlists []Playlist
	Total     int
	HasMore   bool
}

// searchBody 是搜索响应的 body 部分。
type searchBody struct {
	ItemSong     []Song     `json:"item_song"`
	ItemSinger   []Singer   `json:"singer"`
	ItemAlbum    []Album    `json:"item_album"`
	ItemSonglist []Playlist `json:"item_songlist"`
}

type searchData struct {
	Body searchBody `json:"body"`
	Meta struct {
		Sum      int `json:"sum"`
		NextPage int `json:"nextpage"`
	} `json:"meta"`
}

// Search 执行分类搜索。
//
// 该接口有服务端风控，必须走签名通道 + 完整设备指纹；Client.Do 已内置换指纹重试。
func (c *Client) Search(keyword string, searchType SearchType, page, num int) (*SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 20
	}

	param := map[string]any{
		"searchid":      randomSearchID(),
		"query":         keyword,
		"search_type":   int(searchType),
		"num_per_page":  num,
		"page_num":      page,
		"highlight":     false,
		"grp":           true,
		"selectors":     map[string]any{},
		"vec_selectors": []any{},
	}

	data, err := c.Do(cgiRequest{
		Module: "music.search.SearchCgiService",
		Method: "DoSearchForQQMusicMobile",
		Param:  param,
		Sign:   true,
	})
	if err != nil {
		return nil, err
	}

	var parsed searchData
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}

	result := &SearchResult{
		Songs:     parsed.Body.ItemSong,
		Singers:   parsed.Body.ItemSinger,
		Albums:    parsed.Body.ItemAlbum,
		Playlists: parsed.Body.ItemSonglist,
		Total:     parsed.Meta.Sum,
		HasMore:   parsed.Meta.NextPage != -1,
	}
	return result, nil
}

// randomSearchID 生成搜索会话 ID（对齐 QQMusicApi utils/common.py:get_searchID）。
func randomSearchID() string {
	e := 1 + randIntN(20)
	t := int64(e) * 18014398509481984
	n := int64(randIntN(4194304)) * 4294967296
	r := (timeNowMillis()) % (24 * 60 * 60 * 1000)
	return fmt.Sprintf("%d", t+n+r)
}

// QuerySong 批量查询歌曲详情。
func (c *Client) QuerySong(mids []string) ([]Song, error) {
	if len(mids) == 0 {
		return nil, nil
	}
	types := make([]int, len(mids))
	stamps := make([]int, len(mids))

	data, err := c.Do(cgiRequest{
		Module: "music.trackInfo.UniformRuleCtrl",
		Method: "CgiGetTrackInfo",
		Param: map[string]any{
			"ctx":          0,
			"client":       1,
			"types":        types,
			"modify_stamp": stamps,
			"mids":         mids,
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Tracks []Song `json:"tracks"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}
	return parsed.Tracks, nil
}

// QuerySongByID 查询单曲详情。
func (c *Client) QuerySongByID(id int64) (*Song, error) {
	data, err := c.Do(cgiRequest{
		Module: "music.trackInfo.UniformRuleCtrl",
		Method: "CgiGetTrackInfo",
		Param: map[string]any{
			"ctx":          0,
			"client":       1,
			"types":        []int{0},
			"modify_stamp": []int{0},
			"ids":          []int64{id},
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Tracks []Song `json:"tracks"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Tracks) == 0 {
		return nil, fmt.Errorf("qqmusic: 未找到歌曲 id=%d", id)
	}
	return &parsed.Tracks[0], nil
}

// SonglistDetail 是歌单详情。
type SonglistDetail struct {
	Info    Playlist
	Songs   []Song
	Total   int
	HasMore bool
}

// GetSonglistDetail 获取歌单详情与歌曲。
func (c *Client) GetSonglistDetail(disstid int64, page, num int) (*SonglistDetail, error) {
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
			"disstid":      disstid,
			"dirid":        0,
			"tag":          true,
			"song_begin":   num * (page - 1),
			"song_num":     num,
			"userinfo":     true,
			"orderlist":    true,
			"onlysonglist": false,
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		DirInfo struct {
			ID       int64  `json:"id"`
			Title    string `json:"title"`
			PicURL   string `json:"picurl"`
			Desc     string `json:"desc"`
			SongNum  int    `json:"songnum"`
			HostNick string `json:"host_nick"`
			HostUin  int64  `json:"host_uin"`
		} `json:"dirinfo"`
		Songs        []Song `json:"songlist"`
		TotalSongNum int    `json:"total_song_num"`
		HasMore      int    `json:"hasmore"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}

	return &SonglistDetail{
		Info: Playlist{
			ID:       parsed.DirInfo.ID,
			Title:    parsed.DirInfo.Title,
			PicURL:   parsed.DirInfo.PicURL,
			Desc:     parsed.DirInfo.Desc,
			SongNum:  parsed.DirInfo.SongNum,
			HostNick: parsed.DirInfo.HostNick,
			HostUin:  parsed.DirInfo.HostUin,
		},
		Songs:   parsed.Songs,
		Total:   parsed.TotalSongNum,
		HasMore: parsed.HasMore != 0,
	}, nil
}

// GetTopCategories 获取所有榜单分类。
func (c *Client) GetTopCategories() ([]TopCategory, error) {
	data, err := c.Do(cgiRequest{
		Module: "music.musicToplist.Toplist",
		Method: "GetAll",
		Param:  map[string]any{},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Group []TopCategory `json:"group"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}
	return parsed.Group, nil
}

// TopDetail 是榜单详情。
type TopDetail struct {
	Info    TopSummary
	Songs   []Song
	HasMore bool
}

// GetTopDetail 获取榜单详情与歌曲。
func (c *Client) GetTopDetail(topID int64, page, num int) (*TopDetail, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 100
	}

	data, err := c.Do(cgiRequest{
		Module: "music.musicToplist.Toplist",
		Method: "GetDetail",
		Param: map[string]any{
			"topId":    topID,
			"offset":   num * (page - 1),
			"num":      num,
			"withTags": true,
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Data         TopSummary `json:"data"`
		SongInfoList []Song     `json:"songInfoList"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}

	return &TopDetail{
		Info:    parsed.Data,
		Songs:   parsed.SongInfoList,
		HasMore: len(parsed.SongInfoList) >= num,
	}, nil
}

// GetAlbumSongs 获取专辑歌曲。
func (c *Client) GetAlbumSongs(albumMid string, page, num int) ([]Song, int, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 100
	}

	data, err := c.Do(cgiRequest{
		Module: "music.musichallAlbum.AlbumSongList",
		Method: "GetAlbumSongList",
		Param: map[string]any{
			"albumMid": albumMid,
			"begin":    num * (page - 1),
			"num":      num,
		},
	})
	if err != nil {
		return nil, 0, err
	}

	var parsed struct {
		TotalNum int `json:"totalNum"`
		SongList []struct {
			SongInfo Song `json:"songInfo"`
		} `json:"songList"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, 0, err
	}

	songs := make([]Song, 0, len(parsed.SongList))
	for _, item := range parsed.SongList {
		songs = append(songs, item.SongInfo)
	}
	return songs, parsed.TotalNum, nil
}

// GetSingerSongs 获取歌手歌曲。
func (c *Client) GetSingerSongs(singerMid string, page, num int) ([]Song, int, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 100
	}

	data, err := c.Do(cgiRequest{
		Module: "musichall.song_list_server",
		Method: "GetSingerSongList",
		Param: map[string]any{
			"singerMid": singerMid,
			"order":     1, // 按热度
			"number":    num,
			"begin":     num * (page - 1),
		},
	})
	if err != nil {
		return nil, 0, err
	}

	var parsed struct {
		TotalNum int `json:"totalNum"`
		SongList []struct {
			SongInfo Song `json:"songInfo"`
		} `json:"songList"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, 0, err
	}

	songs := make([]Song, 0, len(parsed.SongList))
	for _, item := range parsed.SongList {
		songs = append(songs, item.SongInfo)
	}
	return songs, parsed.TotalNum, nil
}

// GetSingerAlbums 获取歌手专辑。
func (c *Client) GetSingerAlbums(singerMid string, page, num int) ([]Album, int, error) {
	if page <= 0 {
		page = 1
	}
	if num <= 0 {
		num = 100
	}

	data, err := c.Do(cgiRequest{
		Module: "music.musichallAlbum.AlbumListServer",
		Method: "GetAlbumList",
		Param: map[string]any{
			"singerMid": singerMid,
			"order":     1,
			"number":    num,
			"begin":     num * (page - 1),
		},
	})
	if err != nil {
		return nil, 0, err
	}

	var parsed struct {
		TotalNum  int `json:"totalNum"`
		AlbumList []struct {
			AlbumInfo Album `json:"albumInfo"`
		} `json:"albumList"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, 0, err
	}

	albums := make([]Album, 0, len(parsed.AlbumList))
	for _, item := range parsed.AlbumList {
		albums = append(albums, item.AlbumInfo)
	}
	return albums, parsed.TotalNum, nil
}

// GetCdnDispatch 获取音频 CDN 域名列表。
func (c *Client) GetCdnDispatch() ([]string, error) {
	data, err := c.Do(cgiRequest{
		Module: "music.audioCdnDispatch.cdnDispatch",
		Method: "GetCdnDispatch",
		Param: map[string]any{
			"guid":           randHex(10),
			"uid":            "0",
			"use_new_domain": 1,
			"use_ipv6":       1,
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Sip []string `json:"sip"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}
	return parsed.Sip, nil
}

package qqmusic

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/go-musicfox/go-musicfox/internal/structs"
)

// lyricData 是 GetPlayLyricInfo 的响应。
type lyricData struct {
	Lyric string `json:"lyric"`
	Trans string `json:"trans"`
	Roma  string `json:"roma"`
}

// FetchLyric 获取歌词，返回 musicfox 的 LRCData。
//
// 策略：
//  1. 先取 LRC（crypt=0 返回 base64 文本）作为 Original；
//  2. 再取 QRC 逐字歌词，解密后剥离 XML 填入 Yrc，供逐字渲染使用；
//  3. 翻译歌词单独请求，解密后填入 Translated。
//
// 任一子请求失败都不影响其余部分：歌词是增强信息，不应阻断播放。
func (c *Client) FetchLyric(songMid string) (structs.LRCData, error) {
	var data structs.LRCData

	// 1. LRC 原文
	lrc, err := c.fetchLyricPart(songMid, false, false, false)
	if err != nil {
		return data, err
	}
	if decoded := decodeBase64Text(lrc.Lyric); decoded != "" {
		data.Original = decoded
	}

	// 2. QRC 逐字歌词
	qrc, err := c.fetchLyricPart(songMid, true, false, false)
	if err == nil {
		if decrypted, decErr := QrcDecrypt(qrc.Lyric); decErr == nil {
			if content := QrcLyricContent(decrypted); content != "" {
				data.Yrc = content
			}
		}
	}

	// 3. 翻译（QRC 模式下 trans 与逐字歌词同源，优先取带 qrc 的那次）
	trans, err := c.fetchLyricPart(songMid, true, true, false)
	if err == nil && trans.Trans != "" {
		if decrypted, decErr := QrcDecrypt(trans.Trans); decErr == nil {
			data.Translated = decrypted
		} else if plain := decodeBase64Text(trans.Trans); plain != "" {
			data.Translated = plain
		}
	}
	if data.Translated == "" {
		// 回退：不带 qrc 的翻译请求返回 base64 文本
		if plain, plainErr := c.fetchLyricPart(songMid, false, true, false); plainErr == nil {
			data.Translated = decodeBase64Text(plain.Trans)
		}
	}

	if data.Original == "" && data.Yrc == "" {
		return data, fmt.Errorf("qqmusic: 歌曲 %s 无可用歌词", songMid)
	}
	return data, nil
}

// fetchLyricPart 请求歌词的单个组成部分。
func (c *Client) fetchLyricPart(songMid string, qrc, trans, roma bool) (lyricData, error) {
	var out lyricData

	data, err := c.Do(cgiRequest{
		Module: "music.musichallSong.PlayLyricInfo",
		Method: "GetPlayLyricInfo",
		Param: map[string]any{
			"songMid": songMid,
			"crypt":   0,
			"lrc_t":   0,
			"qrc":     boolToInt(qrc),
			"qrc_t":   0,
			"trans":   boolToInt(trans),
			"trans_t": 0,
			"roma":    boolToInt(roma),
			"roma_t":  0,
			"type":    1,
		},
	})
	if err != nil {
		return out, err
	}
	if err := decode(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

// decodeBase64Text 解码接口返回的 base64 歌词文本。
//
// 接口在 crypt=0 时返回 base64(LRC)；非法输入返回空串而非报错。
func decodeBase64Text(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// 部分响应直接是明文 LRC，此时无需解码。
	if strings.HasPrefix(s, "[") {
		return s
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(raw)
}

// boolToInt 将 bool 转为接口要求的 0/1。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

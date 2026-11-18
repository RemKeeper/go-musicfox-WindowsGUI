package ui

import (
	"github.com/go-musicfox/go-musicfox/internal/structs"
)

// Source 是音乐渠道标识。
//
// 与 structs.SourceNetease / structs.SourceQQ 保持一致，便于直接写入 Song.Source。
type Source = string

const (
	// SourceNetease 是网易云音乐渠道。
	SourceNetease Source = structs.SourceNetease
	// SourceQQ 是 QQ 音乐渠道。
	SourceQQ Source = structs.SourceQQ
)

// SourceInfo 描述一个渠道的展示信息。
type SourceInfo struct {
	// ID 是渠道标识，与 structs.Song.Source 对应。
	ID Source
	// Name 是渠道中文名，用于菜单标题。
	Name string
	// Description 是渠道描述，用于启动日志与帮助。
	Description string
}

// allSources 是支持的渠道列表，顺序即主菜单切换顺序。
var allSources = []SourceInfo{
	{ID: SourceNetease, Name: "网易云音乐", Description: "Netease Cloud Music"},
	{ID: SourceQQ, Name: "QQ音乐", Description: "QQ Music"},
}

// sourceByID 按 ID 查找渠道信息。
func sourceByID(id Source) (SourceInfo, bool) {
	for _, s := range allSources {
		if s.ID == id {
			return s, true
		}
	}
	return SourceInfo{}, false
}

// sourceName 返回渠道中文名，未知渠道回退为原始 ID。
func sourceName(id Source) string {
	if s, ok := sourceByID(id); ok {
		return s.Name
	}
	return id
}

// normalizeSource 将任意输入归一为受支持的渠道 ID。
//
// 空串、未知值都回退到网易云，保证默认行为不变。
func normalizeSource(id string) Source {
	switch id {
	case SourceQQ, "qqmusic", "qq_music":
		return SourceQQ
	case "163":
		return SourceNetease
	default:
		return SourceNetease
	}
}

// otherSource 返回另一个渠道（用于切换）。
func otherSource(cur Source) Source {
	if cur == SourceQQ {
		return SourceNetease
	}
	return SourceQQ
}

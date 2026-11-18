package likelist

import (
	"strconv"
	"sync"

	"github.com/buger/jsonparser"
	"github.com/go-musicfox/netease-music/service"

	"github.com/go-musicfox/go-musicfox/internal/structs"
)

// LikeList 是某个渠道的「我喜欢」歌曲 ID 集合。
type LikeList map[int64]struct{}

var (
	// lists 按渠道隔离「我喜欢」状态。
	//
	// 网易云与 QQ 音乐的歌曲 ID 空间重叠，共用一份集合会互相污染，
	// 因此必须按渠道分别存储。
	lists = map[string]LikeList{}
	mu    sync.RWMutex
)

// sourceKey 归一化渠道标识，空串视为网易云。
func sourceKey(source string) string {
	if source == "" {
		return structs.SourceNetease
	}
	return source
}

// IsLikeSong 判断歌曲是否在网易云「我喜欢」中。
//
// 保留该签名以兼容既有调用点；多渠道场景请用 IsLikeSongOf。
func IsLikeSong(songId int64) bool {
	return IsLikeSongOf(structs.SourceNetease, songId)
}

// IsLikeSongOf 判断歌曲是否在指定渠道的「我喜欢」中。
func IsLikeSongOf(source string, songId int64) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := lists[sourceKey(source)][songId]
	return ok
}

// SetLike 在本地缓存中标记/取消标记一首歌，用于操作成功后的乐观更新，
// 避免为了刷新一个图标而重新拉取整个列表。
func SetLike(source string, songId int64, liked bool) {
	mu.Lock()
	defer mu.Unlock()

	key := sourceKey(source)
	list, ok := lists[key]
	if !ok {
		list = make(LikeList)
		lists[key] = list
	}
	if liked {
		list[songId] = struct{}{}
	} else {
		delete(list, songId)
	}
}

// Replace 用给定 ID 集合整体替换某渠道的「我喜欢」缓存。
func Replace(source string, ids []int64) {
	mu.Lock()
	defer mu.Unlock()

	list := make(LikeList, len(ids))
	for _, id := range ids {
		list[id] = struct{}{}
	}
	lists[sourceKey(source)] = list
}

// Len 返回某渠道缓存的歌曲数量。
func Len(source string) int {
	mu.RLock()
	defer mu.RUnlock()
	return len(lists[sourceKey(source)])
}

// RefreshLikeList 刷新网易云「我喜欢」列表。
//
// 保留该签名以兼容既有调用点。
func RefreshLikeList(userId int64) {
	s := &service.LikeListService{UID: strconv.FormatInt(userId, 10)}
	_, resp := s.LikeList()

	var ids []int64
	_, _ = jsonparser.ArrayEach(resp, func(value []byte, _ jsonparser.ValueType, _ int, err error) {
		if err != nil {
			return
		}
		if id, err := jsonparser.ParseInt(value); err == nil {
			ids = append(ids, id)
		}
	}, "ids")

	Replace(structs.SourceNetease, ids)
}

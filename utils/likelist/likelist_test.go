package likelist

import (
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/structs"
)

// TestChannelsAreIsolated 是渠道隔离的关键回归测试：
// 网易云与 QQ 音乐的同号歌曲不能共享「我喜欢」状态。
func TestChannelsAreIsolated(t *testing.T) {
	const songID = int64(12345)

	// 同一 ID 在两个渠道都标记为喜欢
	SetLike(structs.SourceNetease, songID, true)
	SetLike(structs.SourceQQ, songID, true)

	// 取消 QQ 的喜欢，网易云不应受影响
	SetLike(structs.SourceQQ, songID, false)

	if !IsLikeSongOf(structs.SourceNetease, songID) {
		t.Error("取消 QQ 喜欢后，网易云状态被误改")
	}
	if IsLikeSongOf(structs.SourceQQ, songID) {
		t.Error("QQ 喜欢未被取消")
	}
}

// TestEmptySourceIsNetease 校验空渠道归一为网易云，兼容旧调用点。
func TestEmptySourceIsNetease(t *testing.T) {
	const songID = int64(999)

	SetLike("", songID, true)
	if !IsLikeSongOf(structs.SourceNetease, songID) {
		t.Error("空 source 应写入网易云")
	}
	if !IsLikeSong(songID) {
		t.Error("兼容签名 IsLikeSong 应读到网易云状态")
	}

	SetLike("", songID, false)
}

// TestReplace 校验整体替换语义。
func TestReplace(t *testing.T) {
	Replace(structs.SourceQQ, []int64{1, 2, 3})
	if got := Len(structs.SourceQQ); got != 3 {
		t.Errorf("Len = %d, want 3", got)
	}
	if !IsLikeSongOf(structs.SourceQQ, 2) {
		t.Error("ID 2 应在集合中")
	}

	// 再次替换应完全覆盖，而非累加
	Replace(structs.SourceQQ, []int64{9})
	if got := Len(structs.SourceQQ); got != 1 {
		t.Errorf("替换后 Len = %d, want 1", got)
	}
	if IsLikeSongOf(structs.SourceQQ, 2) {
		t.Error("旧数据未被清除")
	}
}

// TestReplaceEmpty 校验替换为空集合。
func TestReplaceEmpty(t *testing.T) {
	Replace(structs.SourceQQ, []int64{1, 2})
	Replace(structs.SourceQQ, nil)
	if got := Len(structs.SourceQQ); got != 0 {
		t.Errorf("清空后 Len = %d, want 0", got)
	}
}

// TestSetLikeIdempotent 校验重复设置同一状态不出错。
func TestSetLikeIdempotent(t *testing.T) {
	const id = int64(777)
	SetLike(structs.SourceQQ, id, true)
	SetLike(structs.SourceQQ, id, true)
	if got := Len(structs.SourceQQ); got != 1 {
		t.Errorf("重复设置后 Len = %d, want 1", got)
	}

	SetLike(structs.SourceQQ, id, false)
	SetLike(structs.SourceQQ, id, false)
	if got := Len(structs.SourceQQ); got != 0 {
		t.Errorf("重复取消后 Len = %d, want 0", got)
	}
}

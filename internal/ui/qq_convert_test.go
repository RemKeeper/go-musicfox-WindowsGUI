package ui

import (
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
)

// TestCollectFileSizes 校验各音质档位的声明大小被正确收集。
//
// 字段映射经实测确认（《鱼玄机》）：
//   - M500 → size_128mp3
//   - M800 → size_320mp3
//   - O801 → size_new[5]
//   - AI00 → size_new[0]
func TestCollectFileSizes(t *testing.T) {
	f := qqmusic.File{
		Size128:  3524501,
		Size320:  8810849,
		SizeFlac: 0, // 该曲无无损
		// 索引：     0        1        2        3       4  5        6
		SizeNew: []int64{0, 19354756, 50689238, 7224130, 0, 15117114, 26753985},
	}

	sizes := collectFileSizes(f)

	// 应包含标准、HQ、SQ(OGG)
	if got, want := sizes[qqmusic.QualityStandard.Key()], int64(3524501); got != want {
		t.Errorf("标准大小 = %d, want %d", got, want)
	}
	if got, want := sizes[qqmusic.QualityHQ.Key()], int64(8810849); got != want {
		t.Errorf("HQ 大小 = %d, want %d", got, want)
	}
	if got, want := sizes[qqmusic.QualityOGG640.Key()], int64(15117114); got != want {
		t.Errorf("SQ(OGG) 大小 = %d, want %d（应取 size_new[5]）", got, want)
	}

	// SizeFlac=0 应被剔除，不留下 0 值
	if _, ok := sizes[qqmusic.QualitySQ.Key()]; ok {
		t.Error("SQ 大小为 0 时应被剔除，避免显示成 0 字节")
	}
	// SizeNew[0]=0 的母带同样应剔除
	if _, ok := sizes[qqmusic.QualityMaster.Key()]; ok {
		t.Error("母带大小为 0 时应被剔除")
	}
}

// TestCollectFileSizesWithMaster 校验母带大小取 size_new[0]。
func TestCollectFileSizesWithMaster(t *testing.T) {
	f := qqmusic.File{
		Size128:  1000000,
		Size320:  3000000,
		SizeFlac: 9000000,
		SizeNew:  []int64{50000000}, // 母带
	}

	sizes := collectFileSizes(f)

	if got, want := sizes[qqmusic.QualityMaster.Key()], int64(50000000); got != want {
		t.Errorf("母带大小 = %d, want %d（应取 size_new[0]）", got, want)
	}
	if got, want := sizes[qqmusic.QualitySQ.Key()], int64(9000000); got != want {
		t.Errorf("SQ 大小 = %d, want %d", got, want)
	}
}

// TestCollectFileSizesEmpty 校验空 File 不 panic 且返回合理结果。
func TestCollectFileSizesEmpty(t *testing.T) {
	sizes := collectFileSizes(qqmusic.File{})
	if len(sizes) != 0 {
		t.Errorf("空 File 应返回空 map，实际 %v", sizes)
	}
}

// TestCollectFileSizesShortSizeNew 校验 size_new 长度不足时不越界。
func TestCollectFileSizesShortSizeNew(t *testing.T) {
	// size_new 只有 3 个元素，访问 [5] 会越界，必须有保护
	f := qqmusic.File{
		Size128: 1000000,
		SizeNew: []int64{1, 2, 3},
	}

	sizes := collectFileSizes(f)
	if _, ok := sizes[qqmusic.QualityOGG640.Key()]; ok {
		t.Error("size_new 长度不足时不应产出 SQ(OGG) 大小")
	}
	if got := sizes[qqmusic.QualityMaster.Key()]; got != 1 {
		t.Errorf("母带大小 = %d, want 1", got)
	}
}

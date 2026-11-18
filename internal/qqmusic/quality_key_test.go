package qqmusic

import (
	"testing"
)

// TestQualityKeyUnique 校验所有音质档位的 Key 互不相同。
//
// 背景：QualitySQ（FLAC）与 QualityOGG640（OGG 640）的 String()
// 都是「SQ」（两者码率相近，对用户展示一致），但它们是不同的文件编码，
// 大小也不同。用 String() 作 map 键会导致其中一档的数据被覆盖。
func TestQualityKeyUnique(t *testing.T) {
	all := []Quality{
		QualityStandard, QualityHQ, QualitySQ,
		QualityOGG320, QualityOGG640, QualityMaster,
	}

	seen := make(map[string]Quality, len(all))
	for _, q := range all {
		key := q.Key()
		if key == "" || key == "unknown" {
			t.Errorf("音质 %v 的 Key 为空或 unknown", q)
			continue
		}
		if prev, ok := seen[key]; ok {
			t.Errorf("Key %q 冲突：%v 与 %v 相同", key, prev, q)
		}
		seen[key] = q
	}

	if len(seen) != len(all) {
		t.Errorf("Key 数量 = %d, want %d（必须一一对应）", len(seen), len(all))
	}
}

// TestQualityStringCollisionDocumented 记录 String 存在重复的事实。
//
// 这不是 bug，而是有意设计：SQ 与 OGG640 对用户的听感档位一致。
// 本测试确保后续若有人「修复」成不同名字，能意识到会影响
// 状态栏显示的一致性。
func TestQualityStringCollisionDocumented(t *testing.T) {
	if QualitySQ.String() != QualityOGG640.String() {
		t.Logf("注意：SQ(%q) 与 OGG640(%q) 的显示名已不同，"+
			"若有代码依赖它们相同需同步调整",
			QualitySQ.String(), QualityOGG640.String())
	}

	// 但 Key 必须不同
	if QualitySQ.Key() == QualityOGG640.Key() {
		t.Fatal("SQ 与 OGG640 的 Key 必须不同")
	}
}

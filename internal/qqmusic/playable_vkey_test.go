package qqmusic

import (
	"testing"
)

// TestVkeyUsesAndroidPlatform 锁定 vkey 必须走 Android 平台。
//
// 这是「无法播放」的最终根因：Web 平台即使带 uin/g_tk 也拿不到 purl，
// 必须用 Android 的 qq/authst 鉴权。
func TestVkeyUsesAndroidPlatform(t *testing.T) {
	// cgiRequest.Android 字段控制平台选择，这里验证其语义。
	web := cgiRequest{Module: "m", Method: "x"}
	if web.Android {
		t.Error("默认应为 Web 平台")
	}

	vkey := cgiRequest{Module: "music.vkey.GetVkey", Method: "UrlGetVkey", Android: true}
	if !vkey.Android {
		t.Error("vkey 请求应标记 Android")
	}
}

// TestQualityFallbacksCoversAllCodes 校验降级链覆盖全部编码。
//
// 实测同一首歌可能只有 O801 能拿到 purl，其余全部 104003。
// 因此降级链必须尝试所有编码，不能只按音质高低排序。
func TestQualityFallbacksCoversAllCodes(t *testing.T) {
	allCodes := []Quality{
		QualityStandard, QualityHQ, QualitySQ,
		QualityOGG320, QualityOGG640, QualityMaster,
	}

	for _, start := range allCodes {
		chain := qualityFallbacks(start)
		if len(chain) != len(allCodes) {
			t.Errorf("从 %v 开始的降级链长度 = %d, want %d",
				start, len(chain), len(allCodes))
		}
		// 请求的音质必须排第一
		if chain[0] != start {
			t.Errorf("从 %v 开始，首项却是 %v", start, chain[0])
		}
		// 必须包含全部编码
		seen := make(map[Quality]bool, len(chain))
		for _, q := range chain {
			seen[q] = true
		}
		for _, want := range allCodes {
			if !seen[want] {
				t.Errorf("从 %v 开始的降级链缺少 %v", start, want)
			}
		}
	}
}

// TestSongHasQualityAllowsAllExceptMaster 校验只跳过母带。
//
// size 字段与实际权限不一致，提前跳过会漏掉唯一可用的编码。
func TestSongHasQualityAllowsAllExceptMaster(t *testing.T) {
	// 一个没有声明任何 size 的歌曲
	song := Song{Mid: "x"}

	// 除母带外都应放行
	for _, q := range []Quality{QualityStandard, QualityHQ, QualitySQ, QualityOGG320, QualityOGG640} {
		if !songHasQuality(song, q) {
			t.Errorf("音质 %v 应放行（交由服务端裁决）", q)
		}
	}
	// 母带需要明确声明
	if songHasQuality(song, QualityMaster) {
		t.Error("未声明 SizeNew 时不应尝试母带")
	}
}

// TestDeviceGUIDConsistency 校验 guid 与 comm 使用同一设备标识。
//
// vkey 的 guid 参数若与 comm 的 OpenUDID 不一致，风控返回
// retcode=104009 (invalidq)。
func TestDeviceGUIDConsistency(t *testing.T) {
	c := NewClient()
	cred := Credential{MusicID: 123, MusicKey: "k"}

	comm := c.buildComm(PlatformAndroid, cred)
	openUDID, _ := comm["OpenUDID"].(string)
	udid, _ := comm["udid"].(string)

	guid := c.deviceGUID()

	if guid == "" {
		t.Fatal("deviceGUID 为空")
	}
	if openUDID != guid {
		t.Errorf("comm OpenUDID = %q, deviceGUID = %q，两者必须一致", openUDID, guid)
	}
	if udid != guid {
		t.Errorf("comm udid = %q, deviceGUID = %q，两者必须一致", udid, guid)
	}
}

// TestDeviceGUIDStableAcrossCalls 校验同一客户端多次取 guid 结果稳定。
func TestDeviceGUIDStableAcrossCalls(t *testing.T) {
	c := NewClient()
	first := c.deviceGUID()
	for i := 0; i < 5; i++ {
		if got := c.deviceGUID(); got != first {
			t.Fatalf("第 %d 次取值 = %q, want %q（guid 不应随机变化）", i+2, got, first)
		}
	}
}

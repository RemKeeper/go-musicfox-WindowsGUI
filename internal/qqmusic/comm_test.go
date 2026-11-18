package qqmusic

import (
	"testing"
)

// TestBuildCommAllStrings 校验 comm 中所有值都是字符串。
//
// 历史 bug：uin / g_tk 以整数发送，QQ 接口对类型敏感，
// 导致 vkey 返回空 purl，表现为「无法播放」。
func TestBuildCommAllStrings(t *testing.T) {
	c := NewClient()
	cred := Credential{
		MusicID:    123456,
		StrMusicID: "00123456",
		MusicKey:   "Q_H_L_test_key",
		LoginTyp:   2,
	}

	for _, platform := range []Platform{PlatformWeb, PlatformAndroid} {
		comm := c.buildComm(platform, cred)
		for k, v := range comm {
			if _, ok := v.(string); !ok {
				t.Errorf("platform=%v comm[%q] 类型 = %T, want string", platform, k, v)
			}
		}
	}
}

// TestBuildCommCarriesCredential 校验登录态被写入 comm。
func TestBuildCommCarriesCredential(t *testing.T) {
	c := NewClient()
	cred := Credential{
		MusicID:    123456,
		StrMusicID: "00123456",
		MusicKey:   "Q_H_L_test_key",
		LoginTyp:   2,
	}

	// Android：qq + authst
	android := c.buildComm(PlatformAndroid, cred)
	if got := android["qq"]; got != "00123456" {
		t.Errorf("android comm qq = %v, want 00123456（应使用 str_musicid）", got)
	}
	if got := android["authst"]; got != "Q_H_L_test_key" {
		t.Errorf("android comm authst = %v, want musickey", got)
	}

	// Web：uin + g_tk
	web := c.buildComm(PlatformWeb, cred)
	if got := web["uin"]; got != "00123456" {
		t.Errorf("web comm uin = %v, want 00123456", got)
	}
	if got := web["g_tk"]; got == "" || got == "0" {
		t.Errorf("web comm g_tk 未计算: %v", got)
	}
	if web["g_tk"] != web["g_tk_new_20200303"] {
		t.Error("g_tk 与 g_tk_new_20200303 应一致")
	}
}

// TestBuildCommAnonymous 校验未登录时 comm 仍然合法。
func TestBuildCommAnonymous(t *testing.T) {
	c := NewClient()

	web := c.buildComm(PlatformWeb, Credential{})
	if got := web["uin"]; got != "0" {
		t.Errorf("未登录 uin = %v, want \"0\"", got)
	}

	android := c.buildComm(PlatformAndroid, Credential{})
	// 未登录时不应带 qq/authst
	if _, ok := android["qq"]; ok {
		t.Error("未登录时 android comm 不应包含 qq")
	}
	if _, ok := android["authst"]; ok {
		t.Error("未登录时 android comm 不应包含 authst")
	}
}

// TestBuildCommRequiredFields 校验各平台必需字段齐全。
func TestBuildCommRequiredFields(t *testing.T) {
	c := NewClient()
	cred := Credential{MusicID: 1, MusicKey: "k"}

	webFields := []string{"ct", "cv", "format", "uin", "g_tk", "platform"}
	web := c.buildComm(PlatformWeb, cred)
	for _, f := range webFields {
		if _, ok := web[f]; !ok {
			t.Errorf("web comm 缺少字段 %q", f)
		}
	}

	androidFields := []string{"ct", "cv", "v", "chid", "tmeAppID", "QIMEI36", "OpenUDID", "aid"}
	android := c.buildComm(PlatformAndroid, cred)
	for _, f := range androidFields {
		if _, ok := android[f]; !ok {
			t.Errorf("android comm 缺少字段 %q", f)
		}
	}
}

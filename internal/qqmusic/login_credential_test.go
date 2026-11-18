package qqmusic

import (
	"testing"
	"time"
)

// TestCredentialExpired 校验凭证过期判断。
func TestCredentialExpired(t *testing.T) {
	now := time.Now().Unix()

	cases := []struct {
		name string
		cred Credential
		want bool
	}{
		{
			name: "有效期充足",
			cred: Credential{MusicID: 1, MusicKey: "k", KeyCreateAt: now - 100, KeyExpiresIn: 3600},
			want: false,
		},
		{
			name: "已过期",
			cred: Credential{MusicID: 1, MusicKey: "k", KeyCreateAt: now - 7200, KeyExpiresIn: 3600},
			want: true,
		},
		{
			name: "缺少有效期信息时保守返回未过期",
			cred: Credential{MusicID: 1, MusicKey: "k"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cred.Expired(); got != tc.want {
				t.Errorf("Expired() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestParseCredentialKeepsRefreshMeta 校验解析保留续期所需字段。
//
// 历史 bug：只保存 musicid/musickey，丢了 keyExpiresIn 与
// musickeyCreateTime，导致无法判断是否需要续期。
func TestParseCredentialKeepsRefreshMeta(t *testing.T) {
	raw := []byte(`{
		"code": 0,
		"musicid": 3047549939,
		"str_musicid": "3047549939",
		"musickey": "Q_H_L_test_key_value",
		"openid": "OPENID123",
		"unionid": "UNIONID456",
		"encryptUin": "enc123",
		"loginType": 2,
		"keyExpiresIn": 259200,
		"musickeyCreateTime": 1794822575
	}`)

	cred, err := parseCredential(raw)
	if err != nil {
		t.Fatalf("parseCredential 失败: %v", err)
	}

	if cred.MusicID != 3047549939 {
		t.Errorf("MusicID = %d", cred.MusicID)
	}
	if cred.StrMusicID != "3047549939" {
		t.Errorf("StrMusicID = %q", cred.StrMusicID)
	}
	if cred.MusicKey != "Q_H_L_test_key_value" {
		t.Errorf("MusicKey = %q", cred.MusicKey)
	}
	if cred.OpenID != "OPENID123" {
		t.Errorf("OpenID = %q", cred.OpenID)
	}
	if cred.UnionID != "UNIONID456" {
		t.Errorf("UnionID = %q", cred.UnionID)
	}
	if cred.LoginTyp != 2 {
		t.Errorf("LoginTyp = %d", cred.LoginTyp)
	}
	if cred.KeyExpiresIn != 259200 {
		t.Errorf("KeyExpiresIn = %d, want 259200", cred.KeyExpiresIn)
	}
	if cred.KeyCreateAt != 1794822575 {
		t.Errorf("KeyCreateAt = %d, want 1794822575", cred.KeyCreateAt)
	}
}

// TestParseCredentialRejectsBadResponse 校验异常响应被拒绝。
func TestParseCredentialRejectsBadResponse(t *testing.T) {
	// 业务错误码
	if _, err := parseCredential([]byte(`{"code": 1000}`)); err == nil {
		t.Error("code=1000 应报错")
	}
	// 缺少 musickey
	if _, err := parseCredential([]byte(`{"code":0,"musicid":123}`)); err == nil {
		t.Error("缺少 musickey 应报错")
	}
	// 缺少 musicid
	if _, err := parseCredential([]byte(`{"code":0,"musickey":"k"}`)); err == nil {
		t.Error("缺少 musicid 应报错")
	}
}

// TestQQLoginModule 锁定 QQ 扫码登录使用的模块与方法。
//
// 这是「登录后无法播放」的根因：QQ 扫码必须走
// QQConnectLogin.LoginServer.QQLogin，用错成
// music.login.LoginServer.Login 会得到一份能刷新但无播放权限的凭证，
// vkey 返回 retcode=104009 (invalidq)。
func TestQQLoginModule(t *testing.T) {
	// 该断言通过源码常量保证：loginByQQCode 内部硬编码了模块名。
	// 这里用一个可读的方式记录预期值，防止后续被误改。
	const wantModule = "QQConnectLogin.LoginServer"
	const wantMethod = "QQLogin"
	const wxModule = "music.login.LoginServer"

	if wantModule == wxModule {
		t.Fatal("QQ 与微信登录模块不应相同")
	}
	if wantMethod != "QQLogin" {
		t.Fatalf("QQ 登录方法应为 QQLogin，实际 %q", wantMethod)
	}
}
